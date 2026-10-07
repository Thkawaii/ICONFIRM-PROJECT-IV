import { Fragment, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { getImportLicenseItems, getImportLicenseSummary, uploadImportLicense, previewImportLicense, deleteImportLicenseItem, clearImportLicense, updateImportLicenseItem } from '../api/importLicense.js';
import { EditableCell, LockBadge, useLiveRefresh } from '../components/InlineEdit.jsx';
import { confirmUploadDeletes, syncResultParts } from '../lib/uploadSync.js';
import { PreviewResult, ChangePreview, ExtraColumnsCell } from '../components/FormatTools.jsx';
import AppShell from '../components/AppShell.jsx';
import FileDropZone from '../components/Filedropzone.jsx';
import SelectField from '../components/Selectfield.jsx';
import { confirmDelete, toastError, toastSuccess } from '../lib/toast.js';
import { computeLicenseExpiry, formatThaiDate, daysLeftLabel, STATUS_LABEL, EXPIRY_STATUS, COMPLETED_FILTER, COMPLETED_LABEL, isLicenseCompleted } from '../lib/licenseExpiry.js';
import { computeExportLicenseDates, leadDaysLabel, leadBadgeClass, LEAD_STATUS, LEAD_STATUS_LABEL, LEAD_BADGE_CLASS, EXPORT_LICENSE_LEAD_DAYS, renewalRoundOf, renewalRoundLabel, isExportChecklistDone, noteText, CHECKLIST_LABEL_DONE, CHECKLIST_LABEL_OPEN } from '../lib/exportLicenseRules.js';
import { useDailyTick } from '../lib/useDailyTick.js';
import { useAppParams } from '../lib/nav.jsx';
import { buildStyledXlsxWorkbookBlob, downloadBlob } from '../lib/xlsx.js';
import PeriodRangePicker from '../components/PeriodRangePicker.jsx';
import { inPeriod, periodRangeLabel, periodFileTag } from '../lib/dateRange.js';
import { ArrowPathIcon, ArrowsRightLeftIcon, CheckBadgeIcon, CheckBadgeSolidIcon, CheckCircleIcon, CheckIcon, ChevronDoubleLeftIcon, ChevronDoubleRightIcon, ChevronDownIcon, ChevronLeftIcon, ChevronRightIcon, ClipboardDocumentCheckIcon, ClockIcon, CubeIcon, DocumentTextIcon, MinusIcon, RectangleStackIcon, ReceiptPercentIcon, ShieldCheckIcon, Squares2X2Icon, TagIcon, TrashIcon, TruckIcon, WrenchScrewdriverIcon, XMarkIcon } from '../components/icons.jsx';
export const WH_NAV_ITEMS = [{
  to: '/warehouse/license-overview',
  label: 'License',
  icon: <RectangleStackIcon className="size-4" />,
  roles: ['LOG']
}, {
  to: '/warehouse/confirm',
  label: 'Part Confirmation',
  icon: <ClipboardDocumentCheckIcon className="size-4" />,
  roles: ['WH']
}];
const EXPIRY_BADGE_CLASS = {
  [EXPIRY_STATUS.EXPIRED]: 'il-badge il-badge-bad',
  [EXPIRY_STATUS.EXPIRING]: 'il-badge il-badge-warn',
  [EXPIRY_STATUS.VALID]: 'il-badge il-badge-ok',
  [EXPIRY_STATUS.NO_DATE]: 'il-badge il-badge-muted'
};

function StatLabel({
  parts
}) {
  return <span className="il-stat-label-text">
      {parts.map((part, i) => <Fragment key={i}>
          {i > 0 && <wbr />}
          <span className="il-nobr">{part}</span>
        </Fragment>)}
    </span>;
}

function CompleteFlag({
  show
}) {
  if (!show) return null;
  return <span className="il-complete-flag" title={COMPLETED_LABEL} aria-label={COMPLETED_LABEL}>
      <CheckCircleIcon className="size-4" />
    </span>;
}

// สถานะ Checklist ของ Export License — มาจากช่อง Remark ในไฟล์ Excel เท่านั้น
// ผู้ใช้ไม่ต้องกดปุ่มเสร็จสิ้นทีละรายการในหน้าเว็บอีกต่อไป
function ChecklistBadge({
  row
}) {
  const done = isExportChecklistDone(row);
  const note = noteText(row);
  const title = note ? `Remark: ${note}` : done ? CHECKLIST_LABEL_DONE : 'ยังไม่มีคำว่า "เสร็จแล้ว" ในช่อง Remark';
  return <span className={'il-badge il-checklist-badge ' + (done ? 'il-checklist-done' : 'il-checklist-open')} title={title}>
      {done ? <CheckBadgeSolidIcon className="il-checklist-icon" aria-hidden="true" /> : <MinusIcon className="size-4" aria-hidden="true" />}
      {done ? CHECKLIST_LABEL_DONE : CHECKLIST_LABEL_OPEN}
    </span>;
}

// จำนวนที่ต่ออายุ — ใบแรก = 1, ต่ออายุแล้วเลขใบอนุญาตเปลี่ยนใหม่ทุกครั้ง
function RenewalRoundBadge({
  row
}) {
  const n = renewalRoundOf(row);
  const latest = row?.IsLatestRound !== false;
  return <span className={'il-round-chip' + (n > 1 ? ' il-round-chip-multi' : '') + (latest ? '' : ' il-round-chip-old')} title={latest ? `${renewalRoundLabel(row)} (ใบปัจจุบัน)` : `${renewalRoundLabel(row)} (ใบเก่า — มีรอบใหม่กว่าแล้ว)`}>
      <span className="il-round-chip-num">{n}</span>
      {!latest && <span className="il-round-chip-old-tag">เก่า</span>}
    </span>;
}

function licenseOptionLabel(licenseNo, total, done, extra = '') {
  const nb = text => text.replace(/ /g, '\u00A0');
  const parts = [licenseNo || '(ไม่มีเลขใบอนุญาต)'];
  if (extra) parts.push(nb(extra));
  parts.push(nb(`${total} เครื่อง`));
  if (done > 0) parts.push(nb(`เสร็จสิ้น ${done}/${total}`));
  return parts.join(' · ');
}

function CompletedOptionIcon() {
  return <span className="il-option-complete" role="img" title="เสร็จสิ้นแล้ว" aria-label="เสร็จสิ้นแล้ว">
      <CheckBadgeSolidIcon className="il-option-complete-icon" aria-hidden="true" />
    </span>;
}

const ALL_COUNTRIES = 'all';
const NO_COUNTRY = '__no_country__';
const NO_COUNTRY_LABEL = 'ไม่ระบุประเทศ';
const MULTI_COUNTRY = '__multi_country__';
const MULTI_COUNTRY_LABEL = 'หลายประเทศ';
function countryKey(value) {
  const key = String(value ?? '').trim().replace(/\s+/g, ' ').toUpperCase();
  return key || NO_COUNTRY;
}
// ใบนำเข้าใบเดียวครอบคลุมได้หลายประเทศ และในไฟล์เขียนรวมไว้ช่องเดียว
// เช่น "INDONESIA , MALAYSIA" (ในไฟล์จริงมีแบบนี้ถึง 501 แถว)
//
// countryKey() ด้านบนไม่ได้แยกค่าพวกนี้ ตอน Export แยกประเทศจึงได้ชีตรวมเพิ่มมาอีกชีต
// แทนที่ใบนั้นจะไปอยู่ในชีต Indonesia และ Malaysia ทั้งสองชีต — คนที่เปิดชีต Malaysia
// เลยไม่เห็นใบที่ใช้กับมาเลเซียได้จริง
//
// แยกด้วยกติกาเดียวกับ countryKeys() ใน api/licenseOverview.js เพื่อให้ Export
// ของทั้งสองหน้าได้ผลเหมือนกัน
function countryKeysOf(value) {
  const raw = String(value ?? '').trim();
  if (!raw) return [NO_COUNTRY];
  const parts = raw.split(/[,/;]|\band\b|\u0e41\u0e25\u0e30/i).map(countryKey).filter(k => k !== NO_COUNTRY);
  return parts.length ? Array.from(new Set(parts)) : [NO_COUNTRY];
}

// ชีตที่แถวนี้ควรไปอยู่ — หนึ่งแถวอยู่ชีตเดียวเท่านั้น ไม่กระจายไปหลายชีต
//
//   ประเทศเดียว      -> ชีตของประเทศนั้น
//   หลายประเทศ       -> ชีต "หลายประเทศ" (ใบที่ยังไม่ได้แตกสายตามประเทศ)
//   ไม่ได้กรอก        -> ชีต "ไม่ระบุประเทศ"
//
// แยกใบหลายประเทศไว้ชีตเดียว เพราะถ้าใส่ซ้ำลงทุกชีตของประเทศที่ครอบคลุม
// ยอดรวมข้ามชีตจะนับใบเดียวกันหลายครั้ง
function countrySheetKey(value) {
  const keys = countryKeysOf(value);
  if (keys.length > 1) return MULTI_COUNTRY;
  return keys[0];
}
function countrySheetLabel(key) {
  if (key === MULTI_COUNTRY) return MULTI_COUNTRY_LABEL;
  return countryLabel(key);
}
function countryLabel(key) {
  if (key === NO_COUNTRY) return NO_COUNTRY_LABEL;
  return key.toLowerCase().replace(/(^|[\s-])(\S)/g, (_, sep, ch) => sep + ch.toUpperCase());
}
function buildCountryOptions(values) {
  const keys = new Set(values.map(countryKey));
  const list = Array.from(keys).filter(k => k !== NO_COUNTRY).sort((a, b) => a.localeCompare(b));
  if (keys.has(NO_COUNTRY)) list.push(NO_COUNTRY);
  return [{
    value: ALL_COUNTRIES,
    label: 'ทุกประเทศ'
  }, ...list.map(k => ({
    value: k,
    label: countryLabel(k)
  }))];
}

function LicenseRefSummary({
  label,
  values
}) {
  return <span className="il-ref-seg">
      {label} <span className="il-ref-first">{values[0] || '—'}</span>
      {values.length > 1 && <span className="il-ref-more">+{values.length - 1}</span>}
    </span>;
}
function LicenseRefList({
  label,
  values
}) {
  if (values.length === 0) return null;
  return <div className="il-ref-list">
      <span className="il-ref-list-label">
        {label} <span className="il-ref-list-count">{values.length}</span>
      </span>
      <div className="il-ref-chips">
        {values.map(v => <span key={v} className="il-ref-chip il-mono">
            {v}
          </span>)}
      </div>
    </div>;
}

function SelectCheckbox({
  checked,
  indeterminate = false,
  onChange,
  label,
  title,
  disabled = false
}) {
  // จำว่ากด Shift ค้างไว้ตอนคลิกหรือไม่ — ใช้เลือกเป็นช่วง (คลิกแถวแรก แล้ว Shift+คลิกแถวสุดท้าย)
  const shiftRef = useRef(false);
  return <label className={'il-check' + (disabled ? ' il-check-disabled' : '')} title={title} onClick={e => e.stopPropagation()} onMouseDown={e => {
    shiftRef.current = e.shiftKey;
    if (e.shiftKey) e.preventDefault(); // กันไม่ให้ข้อความในตารางถูกไฮไลต์ตอน Shift+คลิก
  }}>
      <input type="checkbox" checked={checked} disabled={disabled} ref={el => {
      if (el) el.indeterminate = indeterminate && !checked;
    }} onChange={e => {
      const shift = shiftRef.current || !!e.nativeEvent?.shiftKey;
      shiftRef.current = false;
      onChange(e.target.checked, shift);
    }} aria-label={label} />
      <span className="il-check-box" aria-hidden="true">
        {indeterminate && !checked ? <MinusIcon className="size-3" /> : <CheckIcon className="size-3" />}
      </span>
    </label>;
}

function useRowSelection(visibleRows) {
  const [selected, setSelected] = useState(() => new Set());
  const lastIdRef = useRef(null);
  useEffect(() => {
    setSelected(prev => {
      if (prev.size === 0) return prev;
      const alive = new Set(visibleRows.map(r => r.ID));
      let changed = false;
      const next = new Set();
      prev.forEach(id => {
        if (alive.has(id)) next.add(id);else changed = true;
      });
      return changed ? next : prev;
    });
  }, [visibleRows]);
  // shift=true: เลือก/ยกเลิกทุกแถวระหว่างแถวที่คลิกล่าสุดกับแถวนี้ (ข้ามหน้าได้ ตามลำดับในตาราง)
  const toggleOne = useCallback((id, shift = false) => {
    const lastId = lastIdRef.current;
    lastIdRef.current = id;
    setSelected(prev => {
      const next = new Set(prev);
      const turnOn = !prev.has(id);
      if (shift && lastId != null && lastId !== id) {
        const a = visibleRows.findIndex(r => r.ID === lastId);
        const b = visibleRows.findIndex(r => r.ID === id);
        if (a !== -1 && b !== -1) {
          const [from, to] = a < b ? [a, b] : [b, a];
          for (let i = from; i <= to; i++) {
            if (turnOn) next.add(visibleRows[i].ID);else next.delete(visibleRows[i].ID);
          }
          return next;
        }
      }
      if (turnOn) next.add(id);else next.delete(id);
      return next;
    });
  }, [visibleRows]);
  const setGroup = useCallback((ids, on) => {
    setSelected(prev => {
      const next = new Set(prev);
      ids.forEach(id => {
        if (on) next.add(id);else next.delete(id);
      });
      return next;
    });
  }, []);
  const replaceWith = useCallback(ids => {
    lastIdRef.current = null;
    setSelected(new Set(ids));
  }, []);
  const clear = useCallback(() => {
    lastIdRef.current = null;
    setSelected(new Set());
  }, []);
  const toggleAll = useCallback(on => {
    lastIdRef.current = null;
    setSelected(on ? new Set(visibleRows.map(r => r.ID)) : new Set());
  }, [visibleRows]);
  // กลับการเลือก: ที่เลือกอยู่ → ไม่เลือก, ที่ไม่ได้เลือก → เลือก
  const invert = useCallback(() => {
    lastIdRef.current = null;
    setSelected(prev => new Set(visibleRows.filter(r => !prev.has(r.ID)).map(r => r.ID)));
  }, [visibleRows]);
  const allSelected = visibleRows.length > 0 && selected.size >= visibleRows.length;
  const someSelected = selected.size > 0 && !allSelected;
  return {
    selected,
    toggleOne,
    setGroup,
    replaceWith,
    clear,
    toggleAll,
    invert,
    allSelected,
    someSelected
  };
}

const fmtCount = n => Number(n || 0).toLocaleString('en-US');

// LicenseCheckSelect: dropdown ใบอนุญาต ที่มีช่องติ๊กหน้าแต่ละใบ
// - คลิกชื่อใบ   → ดูเฉพาะใบนั้นในตาราง (เหมือนเดิม) แล้วปิด dropdown
// - ติ๊กช่องหน้าใบ → เลือกไว้หลายใบ (dropdown ไม่ปิด) แล้วใช้ปุ่มเสร็จสิ้น/ลบ ที่แถบด้านล่าง
// - ติ๊กช่อง "ทุกใบอนุญาต" → เลือก/ยกเลิกทุกใบ
function LicenseCheckSelect({
  value,
  onChange,
  options = [],
  allValue,
  checked,
  onToggleCheck,
  onCheckAll
}) {
  const [open, setOpen] = useState(false);
  const boxRef = useRef(null);
  useEffect(() => {
    if (!open) return undefined;
    function onOutside(e) {
      if (boxRef.current && !boxRef.current.contains(e.target)) setOpen(false);
    }
    function onKey(e) {
      if (e.key === 'Escape') setOpen(false);
    }
    document.addEventListener('mousedown', onOutside);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onOutside);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);
  const selected = options.find(o => o.value === value);
  const licenseOpts = options.filter(o => o.value !== allValue);
  const checkedCount = licenseOpts.filter(o => checked.has(o.value)).length;
  const allChecked = licenseOpts.length > 0 && checkedCount === licenseOpts.length;
  function pick(v) {
    onChange(v);
    setOpen(false);
  }
  return <div className="sf il-lcs" ref={boxRef}>
      <button type="button" className={'sf-trigger' + (open ? ' sf-trigger-open' : '')} onClick={() => setOpen(o => !o)} aria-haspopup="listbox" aria-expanded={open}>
        <span className="sf-value-wrap">
          <span className="sf-value">{selected ? selected.label : options[0]?.label}</span>
          {selected?.suffix && <span className="sf-suffix">{selected.suffix}</span>}
        </span>
        {checkedCount > 0 && <span className="il-lcs-badge">เลือกไว้ {fmtCount(checkedCount)} ใบ</span>}
        <span className="sf-chevron" aria-hidden="true">
          <svg width="12" height="8" viewBox="0 0 12 8" fill="none">
            <path d="M1 1.5L6 6.5L11 1.5" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
        </span>
      </button>

      {open && <ul className="sf-list il-lcs-list" role="listbox">
          {options.map(o => {
        const isAll = o.value === allValue;
        const isChecked = isAll ? allChecked : checked.has(o.value);
        return <li key={o.value || '__empty__'} className={'il-lcs-row' + (isAll ? ' il-lcs-row-all' : '') + (o.value === value ? ' sf-option-selected' : '') + (isChecked && !isAll ? ' il-lcs-row-checked' : '')}>
                <SelectCheckbox checked={isChecked} indeterminate={isAll && checkedCount > 0} onChange={on => isAll ? onCheckAll(on) : onToggleCheck(o.value)} label={isAll ? 'เลือกทุกใบอนุญาต' : `เลือก ${o.value}`} title={isAll ? allChecked ? 'ยกเลิกการเลือกทุกใบ' : 'เลือกทุกใบ' : 'เลือกได้หลายใบ'} />
                <button type="button" role="option" aria-selected={o.value === value} className="il-lcs-label" onClick={() => pick(o.value)}>
                  {o.label}
                  {o.suffix && <span className="sf-suffix sf-option-suffix">{o.suffix}</span>}
                </button>
              </li>;
      })}
          {licenseOpts.length === 0 && <li className="sf-empty">ไม่มีใบอนุญาต</li>}
        </ul>}
    </div>;
}

// แถบปุ่มสำหรับใบอนุญาตที่ติ๊กไว้ (แสดงเมื่อติ๊กอย่างน้อย 1 ใบ)
function LicenseActionBar({
  licenseCount,
  machineCount,
  openCount,
  doneCount,
  busy,
  onComplete,
  onUncomplete,
  onDelete,
  onClear
}) {
  if (licenseCount === 0) return null;
  return <div className="il-selection-bar il-license-action-bar" role="status">
      <div className="il-selection-info">
        <span className="il-selection-count">
          <CheckBadgeIcon className="size-4" />
          เลือกไว้ {fmtCount(licenseCount)} ใบอนุญาต ({fmtCount(machineCount)} เครื่อง)
        </span>
        <span className="il-selection-hint">
          {openCount > 0 ? `ยังไม่เสร็จสิ้น ${fmtCount(openCount)} เครื่อง` : 'เสร็จสิ้นแล้วทั้งหมด'}
          {doneCount > 0 && openCount > 0 ? ` · เสร็จสิ้นแล้ว ${fmtCount(doneCount)} เครื่อง` : ''}
        </span>
      </div>
      <div className="il-selection-actions">
        <button type="button" className="il-complete-btn" onClick={onComplete} disabled={busy || openCount === 0}>
          <CheckBadgeIcon className="size-4" />
          เสร็จสิ้น {fmtCount(licenseCount)} ใบ
        </button>
        <button type="button" className="il-uncomplete-btn" onClick={onUncomplete} disabled={busy || doneCount === 0}>
          <ArrowPathIcon className="size-4" />
          ยกเลิกเสร็จสิ้น
        </button>
        <button type="button" className="il-bulk-delete-btn" onClick={onDelete} disabled={busy}>
          <TrashIcon className="size-4" />
          ลบ {fmtCount(licenseCount)} ใบ
        </button>
        <button type="button" className="il-selection-clear" onClick={onClear} disabled={busy}>
          <XMarkIcon className="size-4" />
          ล้างการเลือก
        </button>
      </div>
    </div>;
}

// useCheckedSet: เก็บค่าที่ติ๊กไว้ และตัดค่าที่ไม่มีอยู่แล้วออกอัตโนมัติ (เช่น หลังลบใบ)
function useCheckedSet(validValues) {
  const [checked, setChecked] = useState(() => new Set());
  useEffect(() => {
    setChecked(prev => {
      if (prev.size === 0) return prev;
      const alive = new Set(validValues);
      const next = new Set([...prev].filter(v => alive.has(v)));
      return next.size === prev.size ? prev : next;
    });
  }, [validValues]);
  const toggle = useCallback(v => setChecked(prev => {
    const next = new Set(prev);
    if (next.has(v)) next.delete(v);else next.add(v);
    return next;
  }), []);
  const setAll = useCallback(on => setChecked(on ? new Set(validValues) : new Set()), [validValues]);
  const clear = useCallback(() => setChecked(new Set()), []);
  return {
    checked,
    toggle,
    setAll,
    clear
  };
}

function SelectionBar({
  selectedRows,
  onComplete,
  onUncomplete,
  onDelete,
  onInvert,
  onClear,
  busy,
  lockedCount = 0,
  allSelected = false,
  filterActive = false
}) {
  const count = selectedRows.length;
  if (count === 0) return null;
  const doneCount = selectedRows.filter(isLicenseCompleted).length;
  const openCount = count - doneCount;
  const deletableCount = count - lockedCount;
  return <div className="il-selection-bar" role="status">
      <div className="il-selection-info">
        <span className="il-selection-count">
          <CheckBadgeIcon className="size-4" />
          เลือกไว้ {fmtCount(count)} รายการ
          {allSelected && <span className="il-selection-scope">
              {filterActive ? 'ทุกรายการตามตัวกรอง' : 'ทุกรายการ'}
            </span>}
        </span>
        <span className="il-selection-hint">
          {openCount > 0 ? `ยังไม่เสร็จสิ้น ${fmtCount(openCount)} รายการ` : 'เสร็จสิ้นแล้วทั้งหมด'}
          {doneCount > 0 && openCount > 0 ? ` · เสร็จสิ้นแล้ว ${fmtCount(doneCount)} รายการ` : ''}
          {lockedCount > 0 ? ` · สแกนแล้ว ${fmtCount(lockedCount)} รายการ (ลบไม่ได้)` : ''}
        </span>
      </div>
      <div className="il-selection-actions">
        <button type="button" className="il-complete-btn" onClick={onComplete} disabled={busy || openCount === 0}>
          <CheckBadgeIcon className="size-4" />
          เสร็จสิ้น {fmtCount(openCount)} รายการ
        </button>
        <button type="button" className="il-uncomplete-btn" onClick={onUncomplete} disabled={busy || doneCount === 0}>
          <ArrowPathIcon className="size-4" />
          ยกเลิกสถานะ
        </button>
        {onDelete && <button type="button" className="il-bulk-delete-btn" onClick={onDelete} disabled={busy || deletableCount === 0} title={deletableCount === 0 ? 'รายการที่เลือกสแกนแล้วทั้งหมด ลบไม่ได้' : ''}>
            <TrashIcon className="size-4" />
            ลบ {fmtCount(deletableCount)} รายการ
          </button>}
        {onInvert && !allSelected && <button type="button" className="il-selection-clear" onClick={onInvert} disabled={busy} title="สลับ: รายการที่เลือกอยู่จะถูกยกเลิก และรายการที่เหลือจะถูกเลือกแทน">
            <ArrowsRightLeftIcon className="size-4" />
            กลับการเลือก
          </button>}
        <button type="button" className="il-selection-clear" onClick={onClear} disabled={busy}>
          <XMarkIcon className="size-4" />
          ล้างการเลือก
        </button>
      </div>
    </div>;
}

function ExpiryCell({
  row,
  issueDate,
  expireDate
}) {
  const exp = expireDate ? computeExpireStatus(expireDate, 30) : computeLicenseExpiry(issueDate);
  return <div className="il-expiry-cell">
      <span className={EXPIRY_BADGE_CLASS[exp.status]}>{STATUS_LABEL[exp.status]}</span>
      {exp.hasDate && <>
          <span>{formatThaiDate(exp.expiryDate)}</span>
          <span className="il-expiry-days">{daysLeftLabel(exp.daysLeft)}</span>
        </>}
    </div>;
}
export default function ImportLicensePage() {
  const today = useDailyTick();
  const params = useAppParams();
  const [items, setItems] = useState([]);
  const [summary, setSummary] = useState([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [selectedLot, setSelectedLot] = useState('');
  const [search, setSearch] = useState('');
  const [countryFilter, setCountryFilter] = useState(ALL_COUNTRIES);
  const [expiryFilter, setExpiryFilter] = useState('all');
  const [pageSize, setPageSize] = useState(25);
  const [page, setPage] = useState(1);
  const [detailRow, setDetailRow] = useState(null);
  const [file, setFile] = useState(null);
  const [uploading, setUploading] = useState(false);
  const [uploadMsg, setUploadMsg] = useState(null);
  const [previewData, setPreviewData] = useState(null);
  const [previewing, setPreviewing] = useState(false);
  async function handlePreview() {
    if (!file) {
      setUploadMsg({
        error: 'กรุณาเลือกไฟล์ก่อนตรวจสอบ'
      });
      return;
    }
    setPreviewing(true);
    setPreviewData(null);
    try {
      const data = await previewImportLicense(file);
      setPreviewData(data);
    } catch (err) {
      setUploadMsg({
        error: err.message || 'ตรวจสอบไฟล์ไม่สำเร็จ'
      });
    } finally {
      setPreviewing(false);
    }
  }
  const loadSeq = useRef(0);
  async function loadAll(silent = false) {
    const seq = ++loadSeq.current;
    if (!silent) {
      setLoading(true);
      setLoadError('');
    }
    try {
      const [rows, sum] = await Promise.all([getImportLicenseItems(), getImportLicenseSummary()]);
      if (seq !== loadSeq.current) return;
      setItems(rows || []);
      setSummary(sum || []);
    } catch (err) {
      if (!silent && seq === loadSeq.current) setLoadError(err.message || 'โหลดบัญชีใบอนุญาตนำเข้าไม่สำเร็จ');
    } finally {
      if (!silent && seq === loadSeq.current) setLoading(false);
    }
  }
  useEffect(() => {
    loadAll();
  }, []);
  useLiveRefresh(() => loadAll(true));
  async function saveImportField(row, key, value) {
    try {
      const updated = await updateImportLicenseItem(row.ID, {
        [key]: value
      });
      setItems(list => list.map(r => r.ID === row.ID ? {
        ...r,
        ...updated
      } : r));
      if (['LicenseNo', 'InvoiceNo', 'DeclarationNo', 'Model'].includes(key)) {
        getImportLicenseSummary().then(sum => setSummary(sum || [])).catch(() => {});
      }
    } catch (err) {
      if (err?.status === 409) loadAll(true);
      throw err;
    }
  }
  useEffect(() => {
    setPage(1);
  }, [selectedLot, search, countryFilter, expiryFilter, pageSize]);
  useEffect(() => {
    const lic = (params?.focusLicense || '').trim();
    if (!lic) return;
    setCountryFilter(ALL_COUNTRIES);
    setExpiryFilter('all');
    setSelectedLot('');
    setSearch(lic);
  }, [params?.focusLicense, params?.focusInvoice, params?.focusTs]);
  useEffect(() => {
    const lic = (params?.focusLicense || '').trim();
    const inv = (params?.focusInvoice || '').trim();
    if (!lic || !inv) return;
    const key = `${lic}|${inv}`;
    if (summary.some(s => `${s.LicenseNo}|${s.InvoiceNo}` === key)) {
      setSelectedLot(key);
    }
  }, [summary, params?.focusLicense, params?.focusInvoice, params?.focusTs]);
  async function handleUpload() {
    if (!file) {
      setUploadMsg({
        error: 'กรุณาเลือกไฟล์ Excel หรือ CSV ก่อน'
      });
      return;
    }
    setUploading(true);
    setUploadMsg(null);
    try {
      const summary = previewData?.summary || (await previewImportLicense(file))?.summary;
      if (!(await confirmUploadDeletes(summary, file.name))) {
        return;
      }
      const result = await uploadImportLicense(file);
      setUploadMsg({
        success: [`เพิ่มใหม่ ${result.imported ?? 0}`, `อัปเดต ${result.updated ?? 0}`, ...(result.unchanged ? [`เหมือนเดิม ${result.unchanged}`] : []), ...(result.locked ? [`สแกนแล้ว ไม่อัปเดต ${result.locked}`] : []), ...syncResultParts(result), `ข้าม ${result.skipped ?? 0}`].join(' · '),
        notice: result.extraNotice || '',
        problems: result.problems || []
      });
      setFile(null);
      setPreviewData(null);
      await loadAll();
    } catch (err) {
      setUploadMsg({
        error: err.message || 'อัปโหลดไม่สำเร็จ'
      });
    } finally {
      setUploading(false);
    }
  }
  async function handleDeleteRow(row) {
    const ok = await confirmDelete({
      text: `ลบหมายเลขเครื่อง ${row.MachineNo} ออกจากบัญชี?`
    });
    if (!ok) return;
    try {
      await deleteImportLicenseItem(row.ID);
      await loadAll();
      toastSuccess(`ลบ ${row.MachineNo} แล้ว`);
    } catch (err) {
      toastError(err.message || 'ลบไม่สำเร็จ');
      if (err?.status === 409) loadAll(true);
    }
  }
  async function handleClearAllImport() {
    const ok = await confirmDelete({
      text: 'ลบใบอนุญาตนำเข้าทั้งหมดออกจากระบบ? กู้คืนไม่ได้',
      confirmText: 'ลบทั้งหมด'
    });
    if (!ok) return;
    try {
      const res = await clearImportLicense('', '', true);
      setSelectedLot('');
      await loadAll();
      toastSuccess(`ลบแล้ว ${res.deleted ?? 0} เครื่อง`);
    } catch (err) {
      toastError(err.message || 'ลบไม่สำเร็จ');
    }
  }
  async function handleClearLicense(lot) {
    const licenseNo = lot?.LicenseNo ?? '';
    const invoiceNo = lot?.InvoiceNo ?? '';
    const label = licenseNo || (invoiceNo ? `Invoice ${invoiceNo}` : 'ล็อตนี้ (ไม่มีเลขใบอนุญาต)');
    const ok = await confirmDelete({
      text: `ลบ ${label} ออกจากระบบทั้งล็อต? กู้คืนไม่ได้`,
      confirmText: 'ลบทั้งใบ'
    });
    if (!ok) return;
    try {
      await clearImportLicense(licenseNo, invoiceNo);
      setSelectedLot('');
      await loadAll();
      toastSuccess(`ลบ ${label} แล้ว`);
    } catch (err) {
      const msg = err.message || 'ลบไม่สำเร็จ';
      setLoadError(msg);
      toastError(msg);
    }
  }
  const filtered = useMemo(() => {
    let rows = items;
    if (selectedLot) {
      const [licenseNo, invoiceNo] = selectedLot.split('|');
      rows = rows.filter(r => r.LicenseNo === licenseNo && r.InvoiceNo === invoiceNo);
    }
    if (countryFilter !== ALL_COUNTRIES) {
      rows = rows.filter(r => countryKey(r.ExportCountry) === countryFilter);
    }
    if (expiryFilter === COMPLETED_FILTER) {
      rows = rows.filter(isLicenseCompleted);
    } else if (expiryFilter !== 'all') {
      rows = rows.filter(r => !isLicenseCompleted(r) && computeLicenseExpiry(r.IssueDate).status === expiryFilter);
    }
    const term = search.trim().toLowerCase();
    if (term) {
      rows = rows.filter(r => (r.MachineNo || '').toLowerCase().includes(term) || (r.ProductionNo || '').toLowerCase().includes(term) || (r.LicenseNo || '').toLowerCase().includes(term) || (r.InvoiceNo || '').toLowerCase().includes(term) || (r.DeclarationNo || '').toLowerCase().includes(term) || (r.Model || '').toLowerCase().includes(term) || (r.ExportCountry || '').toLowerCase().includes(term) || (r.ExportLicenseNo || '').toLowerCase().includes(term));
    }
    rows = [...rows].sort((a, b) => (a.SortOrder || 0) - (b.SortOrder || 0) || a.ID - b.ID);
    return rows;
  }, [items, selectedLot, countryFilter, expiryFilter, search, today]);
  const countryOptions = useMemo(() => buildCountryOptions(items.map(r => r.ExportCountry)), [items]);
  const filterActive = !!selectedLot || countryFilter !== ALL_COUNTRIES || expiryFilter !== 'all' || search.trim() !== '';

  useEffect(() => {
    if (countryFilter !== ALL_COUNTRIES && !countryOptions.some(o => o.value === countryFilter)) {
      setCountryFilter(ALL_COUNTRIES);
    }
  }, [countryOptions, countryFilter]);
  const expiryOptions = useMemo(() => [{
    value: 'all',
    label: 'ทุกสถานะวันหมดอายุ'
  }, {
    value: EXPIRY_STATUS.NO_DATE,
    label: STATUS_LABEL[EXPIRY_STATUS.NO_DATE]
  }, {
    value: EXPIRY_STATUS.EXPIRING,
    label: STATUS_LABEL[EXPIRY_STATUS.EXPIRING]
  }, {
    value: EXPIRY_STATUS.EXPIRED,
    label: STATUS_LABEL[EXPIRY_STATUS.EXPIRED]
  }, {
    value: EXPIRY_STATUS.VALID,
    label: STATUS_LABEL[EXPIRY_STATUS.VALID]
  }, {
    value: COMPLETED_FILTER,
    label: COMPLETED_LABEL
  }], []);
  const lotOptions = useMemo(() => {
    const opts = [{
      value: '',
      label: 'ทุกใบอนุญาต'
    }];
    const lotsPerLicense = new Map();
    summary.forEach(s => lotsPerLicense.set(s.LicenseNo, (lotsPerLicense.get(s.LicenseNo) || 0) + 1));
    summary.forEach(s => {
      const completedAll = s.Total > 0 && s.CompletedCount >= s.Total;
      const extra = lotsPerLicense.get(s.LicenseNo) > 1 ? `Invoice ${s.InvoiceNo || '—'}` : '';
      opts.push({
        value: `${s.LicenseNo}|${s.InvoiceNo}`,
        label: licenseOptionLabel(s.LicenseNo, s.Total, s.CompletedCount, extra),
        suffix: completedAll ? <CompletedOptionIcon /> : null
      });
    });
    return opts;
  }, [summary]);
  const counts = useMemo(() => ({
    total: items.length,
    licenses: new Set(items.map(r => r.LicenseNo).filter(Boolean)).size,
    invoices: new Set(items.map(r => r.InvoiceNo).filter(Boolean)).size,
    completed: items.filter(isLicenseCompleted).length
  }), [items]);

  const expiryCounts = useMemo(() => {
    const worst = new Map();
    for (const row of items) {
      if (isLicenseCompleted(row)) continue;
      const key = (row.LicenseNo || '').trim();
      if (!key) continue;
      const exp = row.ExpireDate ? computeExpireStatus(row.ExpireDate, 30) : computeLicenseExpiry(row.IssueDate);
      if (exp.status !== EXPIRY_STATUS.EXPIRED && exp.status !== EXPIRY_STATUS.EXPIRING) continue;
      if (worst.get(key) === EXPIRY_STATUS.EXPIRED) continue;
      worst.set(key, exp.status);
    }
    let expiring = 0;
    let expired = 0;
    for (const status of worst.values()) {
      if (status === EXPIRY_STATUS.EXPIRED) expired++;else expiring++;
    }
    return {
      expiring,
      expired
    };
  }, [items, today]);
  const totalPages = Math.max(1, Math.ceil(filtered.length / pageSize));
  const paged = filtered.slice((page - 1) * pageSize, page * pageSize);
  function goToPage(p) {
    setPage(Math.min(Math.max(1, p), totalPages));
  }
  const currentLot = summary.find(s => `${s.LicenseNo}|${s.InvoiceNo}` === selectedLot);

  // หน้านี้ไม่มีการเลือกแถวและไม่มีปุ่มปิดงานแล้ว
  // สถานะเสร็จสิ้นมาจากไฟล์ที่อัปโหลดอย่างเดียว






  // เก็บไว้เผื่อกรณีพิเศษ (ไฟล์ที่ไม่มีคอลัมน์ Note) — ปกติสถานะมาจาก Note อัตโนมัติแล้ว
  const lotCompletedAll = currentLot ? currentLot.CompletedCount >= currentLot.Total && currentLot.Total > 0 : false;
  return <AppShell navItems={WH_NAV_ITEMS} roleLabel="Warehouse">
      <div className="wh-heading-row">
        <div>
          <h2 className="wh-title">Import License</h2>
        </div>
      </div>

      {loadError && <p className="form-error" role="alert">
          {loadError}
        </p>}

      <div className="wh-upload-card">
        <div className="fdz-row">
          <FileDropZone file={file} onSelect={f => {
          setFile(f);
          setUploadMsg(null);
          setPreviewData(null);
        }} accept=".xlsx,.xls,.csv" label="อัปโหลดบัญชีใบอนุญาตนำเข้า" disabled={uploading} />
          <button className="wh-modal-cancel" onClick={handlePreview} disabled={previewing || uploading || !file}>
            {previewing ? 'กำลังตรวจสอบ...' : 'ตรวจสอบก่อนอัปโหลด'}
          </button>
          <button className="wh-issue-btn" onClick={handleUpload} disabled={uploading || !file}>
            {uploading ? 'กำลังอัปโหลด...' : 'อัปโหลด'}
          </button>
        </div>

        {previewData && (previewData.summary ? <ChangePreview result={previewData} /> : <PreviewResult result={previewData} />)}

        {uploadMsg?.success && <p className="upload-card-msg upload-card-msg-ok wh-upload-msg">{uploadMsg.success}</p>}
        {uploadMsg?.notice && <ExtraColumnNotice text={uploadMsg.notice} />}
        {uploadMsg?.error && <p className="upload-card-msg upload-card-msg-err wh-upload-msg">{uploadMsg.error}</p>}
        {uploadMsg?.problems?.length > 0 && <ul className="il-problem-list">
            {uploadMsg.problems.map((p, i) => <li key={i}>{p}</li>)}
          </ul>}
      </div>

      <div className="dash-stats-row wh-stats-row il-stats-row-5">
        <div className="dash-stat-card">
          <div className="dash-stat-label">
            <StatLabel parts={['เครื่องในบัญชี', 'ทั้งหมด']} />
            <span className="dash-stat-icon dash-icon-blue">
              <Squares2X2Icon className="size-4" />
            </span>
          </div>
          <div className="dash-stat-value">{counts.total}</div>
        </div>
        <div className="dash-stat-card">
          <div className="dash-stat-label">
            <StatLabel parts={['ใบอนุญาต', 'นำเข้า']} />
            <span className="dash-stat-icon dash-icon-red">
              <DocumentTextIcon className="size-4" />
            </span>
          </div>
          <div className="dash-stat-value">{counts.licenses}</div>
        </div>
        <div className="dash-stat-card">
          <div className="dash-stat-label">
            <StatLabel parts={['อินวอยซ์', 'นำเข้า']} />
            <span className="dash-stat-icon dash-icon-yellow">
              <ReceiptPercentIcon className="size-4" />
            </span>
          </div>
          <div className="dash-stat-value">{counts.invoices}</div>
        </div>
        <div className="dash-stat-card il-stat-expiry">
          <div className="dash-stat-label">
            <StatLabel parts={['อายุ', 'ใบอนุญาต']} />
            <span className="dash-stat-icon dash-icon-orange">
              <ClockIcon className="size-4" />
            </span>
          </div>
          <div className="il-stat-expiry-split">
            <div className="il-stat-expiry-part il-stat-expiring">
              <div className="dash-stat-value">{expiryCounts.expiring}</div>
              <div className="dash-stat-note">ใกล้หมดอายุ</div>
            </div>
            <span className="il-stat-expiry-divider" aria-hidden="true" />
            <div className="il-stat-expiry-part il-stat-expired">
              <div className="dash-stat-value">{expiryCounts.expired}</div>
              <div className="dash-stat-note">หมดอายุแล้ว</div>
            </div>
          </div>
        </div>
        <div className="dash-stat-card il-stat-complete">
          <div className="dash-stat-label">
            <StatLabel parts={['เสร็จสิ้นแล้ว']} />
            <span className="dash-stat-icon dash-icon-green">
              <CheckBadgeIcon className="size-4" />
            </span>
          </div>
          <div className="dash-stat-value">{counts.completed}</div>
        </div>
      </div>

      {summary.length > 0 && <div className="il-lot-filter">
          <label className="il-lot-filter-label">ใบอนุญาต</label>
          <div className="il-lot-filter-select">
            <SelectField value={selectedLot} onChange={setSelectedLot} options={lotOptions} />
          </div>
        </div>}


      {currentLot && <div className="wh-so-active-bar il-license-bar">
          <div className="il-lot-info">
            <div className="il-lot-info-text">
              <span className="wh-so-active-label">ใบอนุญาตนำเข้า</span>
              <h3 className="wh-so-active-name">{currentLot.LicenseNo || '(ไม่มีเลขใบอนุญาต)'}</h3>
              <span className="wh-subtitle">
                Invoice {currentLot.InvoiceNo || '—'} · ใบขนสินค้า {currentLot.DeclarationNo || '—'} · {currentLot.Total} เครื่อง
              </span>
            </div>
            {lotCompletedAll && <span className="il-complete-stamp" title="ปิดงานทั้งใบแล้ว">
                <CheckBadgeSolidIcon className="il-complete-stamp-icon" aria-hidden="true" />
                เสร็จสิ้นแล้ว
              </span>}
          </div>
          <div className="il-lot-actions">
            <button className="wh-modal-cancel" onClick={() => handleClearLicense(currentLot)}>
              ลบทั้งใบ
            </button>
          </div>
        </div>}

      <div className="tsf-history-toolbar">
        <div className="tsf-history-pagesize">
          <div className="wh-pagesize-select">
            <SelectField value={pageSize} onChange={setPageSize} options={[{
            value: 10,
            label: '10'
          }, {
            value: 25,
            label: '25'
          }, {
            value: 50,
            label: '50'
          }, {
            value: 100,
            label: '100'
          }]} />
          </div>
          entries per page
        </div>
        <div className="il-filter-search-group">
          <div className="wh-pagesize-select il-model-filter">
            <SelectField value={countryFilter} onChange={setCountryFilter} options={countryOptions} />
          </div>
          <div className="wh-pagesize-select il-model-filter">
            <SelectField value={expiryFilter} onChange={setExpiryFilter} options={expiryOptions} />
          </div>
          <input className="wh-search" type="text" placeholder="ค้นหา หมายเลขเครื่อง / หมายเลขการผลิต / ใบอนุญาต / อินวอยซ์ / ใบขนสินค้า" value={search} onChange={e => setSearch(e.target.value)} />
          {items.length > 0 && <button className="wh-btn-danger" onClick={handleClearAllImport}>
              ลบทุกใบอนุญาต
            </button>}
        </div>
      </div>


      <div className="wh-table-card">
        <table className="wh-table il-import-table">
          <thead>
            <tr>
              <th>ลำดับ</th>
              <th>ตราอักษร</th>
              <th>แบบ/รุ่น</th>
              <th>เลขใบอนุญาตนำเข้า</th>
              <th>วันที่ออกใบอนุญาต</th>
              <th>หมดอายุ (6 เดือน)</th>
              <th>เลขอินวอยซ์นำเข้า</th>
              <th>เลขใบขนสินค้าขาเข้า</th>
              <th>จำนวน (เครื่อง)</th>
              <th>หมายเลขเครื่อง</th>
              <th>หมายเลขการผลิต</th>
              <th>หมายเหตุ</th>
              <th>ส่งออกไปประเทศ</th>
              <th>คอลัมน์เพิ่ม</th>
              <th>สถานะการสแกน</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {loading && <tr>
                <td colSpan={16} className="wh-empty-cell">
                  กำลังโหลดข้อมูล...
                </td>
              </tr>}
            {!loading && paged.map((row, i) => {
            const lock = {
              locked: !!row.Locked,
              lockReason: row.LockReason
            };
            const save = key => v => saveImportField(row, key, v);
            return <tr key={row.ID} className={row.Locked ? 'ie-row-locked' : ''}>
                  <td className="wh-cell-head" data-label="ลำดับ">
                    {(page - 1) * pageSize + i + 1}
                  </td>
                  <td data-label="ตราอักษร">
                    <EditableCell readOnly {...lock} label="ตราอักษร" value={row.Brand} onSave={save('Brand')} />
                  </td>
                  <td data-label="แบบ/รุ่น">
                    <EditableCell readOnly {...lock} label="แบบ/รุ่น" value={row.Model} onSave={save('Model')} />
                  </td>
                  <td data-label="เลขใบอนุญาตนำเข้า">
                    <span className="il-license-cell">
                      <CompleteFlag show={isLicenseCompleted(row)} />
                      <EditableCell readOnly {...lock} label="เลขใบอนุญาตนำเข้า" value={row.LicenseNo} onSave={save('LicenseNo')} />
                    </span>
                  </td>
                  <td data-label="วันที่ออกใบอนุญาต">
                    <EditableCell readOnly {...lock} type="date" label="วันที่ออกใบอนุญาต" value={row.IssueDate} display={formatThaiDate(row.IssueDate)} onSave={save('IssueDate')} />
                  </td>
                  <td data-label="หมดอายุ (6 เดือน)">
                    <ExpiryCell row={row} issueDate={row.IssueDate} expireDate={row.ExpireDate} />
                  </td>
                  <td data-label="เลขอินวอยซ์นำเข้า">
                    <EditableCell readOnly {...lock} label="เลขอินวอยซ์นำเข้า" value={row.InvoiceNo} onSave={save('InvoiceNo')} />
                  </td>
                  <td data-label="เลขใบขนสินค้าขาเข้า">
                    <EditableCell readOnly {...lock} label="เลขใบขนสินค้าขาเข้า" value={row.DeclarationNo} onSave={save('DeclarationNo')} />
                  </td>
                  <td data-label="จำนวน (เครื่อง)">
                    <EditableCell readOnly {...lock} type="number" label="จำนวน (เครื่อง)" value={row.Qty} onSave={save('Qty')} />
                  </td>
                  <td className="il-mono" data-label="หมายเลขเครื่อง">
                    <EditableCell readOnly {...lock} mono label="หมายเลขเครื่อง" value={row.MachineNo} display={<strong>{row.MachineNo}</strong>} onSave={save('MachineNo')} />
                  </td>
                  <td className="il-mono" data-label="หมายเลขการผลิต">
                    <EditableCell readOnly {...lock} mono label="หมายเลขการผลิต" value={row.ProductionNo} onSave={save('ProductionNo')} />
                  </td>
                  <td data-label="หมายเหตุ">
                    <EditableCell readOnly {...lock} label="หมายเหตุ" value={row.Remark} onSave={save('Remark')} />
                  </td>
                  <td data-label="ส่งออกไปประเทศ">
                    <EditableCell readOnly {...lock} label="ส่งออกไปประเทศ" value={row.ExportCountry} display={row.ExportCountry || <span className="il-no-country">{NO_COUNTRY_LABEL}</span>} onSave={save('ExportCountry')} />
                  </td>
                  <td data-label="คอลัมน์เพิ่ม">
                    <ExtraColumnsCell json={row.extra_json} />
                  </td>
                  <td data-label="สถานะการสแกน">
                    {row.Locked ? <LockBadge locked variant="done" reason={row.LockReason} /> : <span className="ie-empty">ยังไม่ได้สแกน</span>}
                  </td>
                  <td className="wh-cell-action">
                    <div className="il-row-actions">
                      <button className="wh-modal-cancel" onClick={() => setDetailRow(row)}>
                        รายละเอียด
                      </button>
                      <button className="wh-btn-danger" disabled={row.Locked} title={row.Locked ? `ลบไม่ได้ — ${row.LockReason || 'สแกนผ่านแล้ว'}` : ''} onClick={() => handleDeleteRow(row)}>
                        ลบ
                      </button>
                    </div>
                  </td>
                </tr>;
          })}
            {!loading && paged.length === 0 && <tr>
                <td colSpan={16} className="wh-empty-cell">
                  ยังไม่มีข้อมูล
                </td>
              </tr>}
          </tbody>
        </table>
      </div>

      {!loading && filtered.length > 0 && <div className="tsf-pagination">
          <span className="wh-subtitle" style={{
        fontSize: 13
      }}>
            Showing {(page - 1) * pageSize + 1} to {Math.min(page * pageSize, filtered.length)} of{' '}
            {filtered.length} entries
          </span>
          <div className="tsf-pagination-buttons">
            <button className="wh-modal-cancel" onClick={() => goToPage(1)} disabled={page === 1}>
              <ChevronDoubleLeftIcon className="size-4" />
            </button>
            <button className="wh-modal-cancel" onClick={() => goToPage(page - 1)} disabled={page === 1}>
              <ChevronLeftIcon className="size-4" />
            </button>
            <span className="tsf-pagination-current">
              {page} / {totalPages}
            </span>
            <button className="wh-modal-cancel" onClick={() => goToPage(page + 1)} disabled={page === totalPages}>
              <ChevronRightIcon className="size-4" />
            </button>
            <button className="wh-modal-cancel" onClick={() => goToPage(totalPages)} disabled={page === totalPages}>
              <ChevronDoubleRightIcon className="size-4" />
            </button>
          </div>
        </div>}

      {detailRow && <ImportDetailModal row={detailRow} onClose={() => setDetailRow(null)} />}
    </AppShell>;
}
function useDetailSheet(onClose) {
  const onCloseRef = useRef(onClose);
  onCloseRef.current = onClose;
  const scrollRef = useRef(null);
  useEffect(() => {
    const prevOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    const onKey = e => {
      if (e.key === 'Escape') onCloseRef.current?.();
    };
    window.addEventListener('keydown', onKey);

    const scroller = scrollRef.current;
    const overlay = scroller?.closest('.il-detail-overlay');
    const fromOutside = e => scroller && !scroller.contains(e.target);
    const onWheel = e => {
      if (!fromOutside(e)) return;
      e.preventDefault();
      scroller.scrollTop += e.deltaMode === 1 ? e.deltaY * 16 : e.deltaY;
    };
    let lastY = null;
    const onTouchStart = e => {
      lastY = fromOutside(e) && e.touches.length === 1 ? e.touches[0].clientY : null;
    };
    const onTouchMove = e => {
      if (lastY === null) return;
      const y = e.touches[0].clientY;
      scroller.scrollTop += lastY - y;
      lastY = y;
      e.preventDefault();
    };
    const onTouchEnd = () => {
      lastY = null;
    };
    if (overlay) {
      overlay.addEventListener('wheel', onWheel, {
        passive: false
      });
      overlay.addEventListener('touchstart', onTouchStart, {
        passive: true
      });
      overlay.addEventListener('touchmove', onTouchMove, {
        passive: false
      });
      overlay.addEventListener('touchend', onTouchEnd);
      overlay.addEventListener('touchcancel', onTouchEnd);
    }
    return () => {
      document.body.style.overflow = prevOverflow;
      window.removeEventListener('keydown', onKey);
      if (overlay) {
        overlay.removeEventListener('wheel', onWheel);
        overlay.removeEventListener('touchstart', onTouchStart);
        overlay.removeEventListener('touchmove', onTouchMove);
        overlay.removeEventListener('touchend', onTouchEnd);
        overlay.removeEventListener('touchcancel', onTouchEnd);
      }
    };
  }, []);
  return scrollRef;
}
function ImportDetailModal({
  row,
  onClose
}) {
  const completed = isLicenseCompleted(row);
  const scrollRef = useDetailSheet(onClose);
  const item = (label, value) => <div className="wh-detail-item">
      <span className="wh-detail-label">{label}</span>
      <span className="wh-detail-value">{value === 0 || value ? value : '—'}</span>
    </div>;
  const exp = computeLicenseExpiry(row.IssueDate);
  const CONFIRM_LABEL = {
    CONFIRMED: 'ยืนยันแล้ว',
    PENDING: 'รอยืนยัน',
    REJECTED: 'ไม่ผ่าน'
  };
  const confirmLabel = CONFIRM_LABEL[row.ConfirmStatus] || row.ConfirmStatus;
  let extraEntries = [];
  try {
    const obj = row.extra_json ? JSON.parse(row.extra_json) : null;
    if (obj) extraEntries = Object.entries(obj);
  } catch {
    extraEntries = [];
  }
  return <div className="wh-modal-overlay il-detail-overlay" onClick={onClose}>
      <div className="wh-modal wh-detail-modal il-detail-sheet" role="dialog" aria-modal="true" aria-labelledby="il-import-detail-title" onClick={e => e.stopPropagation()}>
        <button type="button" className="wh-detail-close" onClick={onClose} aria-label="ปิด">
          <XMarkIcon className="size-4" />
        </button>

        <div className="wh-detail-header">
          <span className="wh-detail-header-icon">
            <DocumentTextIcon className="size-5" />
          </span>
          <div>
            <h3 className="wh-modal-title" id="il-import-detail-title">รายละเอียดใบอนุญาตนำเข้า</h3>
            <span className="wh-detail-header-sub">{row.Model || row.Brand || '—'}</span>
          </div>
        </div>

        <div className="il-detail-scroll" ref={scrollRef}>

        <div className="wh-detail-section">
          <span className="wh-detail-section-title">
            <CubeIcon className="size-4" /> ข้อมูลเครื่อง
          </span>
          <div className="wh-detail-grid">
            {item('หมายเลขเครื่อง', row.MachineNo)}
            {item('หมายเลขการผลิต', row.ProductionNo)}
            {item('ตราอักษร', row.Brand)}
            {item('แบบ/รุ่น', row.Model)}
            {item('จำนวน (เครื่อง)', row.Qty)}
            {item('ส่งออกไปประเทศ', row.ExportCountry || NO_COUNTRY_LABEL)}
          </div>
        </div>

        <div className="wh-detail-divider" />

        <div className="wh-detail-section">
          <span className="wh-detail-section-title">
            <DocumentTextIcon className="size-4" /> ข้อมูลใบอนุญาต
          </span>
          <div className="wh-detail-grid">
            {item('เลขใบอนุญาตนำเข้า', row.LicenseNo)}
            {item('เลขอินวอยซ์นำเข้า', row.InvoiceNo)}
            {item('เลขใบขนสินค้าขาเข้า', row.DeclarationNo)}
            {item('วันที่ออกใบอนุญาต', row.IssueDate ? formatThaiDate(row.IssueDate) : '')}
            {item('วันหมดอายุ (6 เดือน)', exp.hasDate ? formatThaiDate(exp.expiryDate) : '')}
            {/* ไฟล์บัญชีใบอนุญาตนำเข้าบอกไว้ท้ายแถวว่าเครื่องนี้ถูกส่งออกด้วยใบนำออกใบไหน */}
            {item('เลขใบอนุญาตนำออก', row.ExportLicenseNo)}
            {item('วันที่ออกใบอนุญาตนำออก', row.ExportIssueDate ? formatThaiDate(row.ExportIssueDate) : '')}
            <div className="wh-detail-item">
              <span className="wh-detail-label">สถานะอายุ</span>
              <span className="wh-detail-value il-detail-status">
                {completed ? <>
                    <span className="il-badge il-badge-complete">
                      <CheckBadgeIcon className="size-3.5" />
                      {COMPLETED_LABEL}
                    </span>
                    {exp.hasDate && <span className="il-detail-days">หมดอายุ {formatThaiDate(exp.expiryDate)}</span>}
                  </> : <>
                    <span className={EXPIRY_BADGE_CLASS[exp.status]}>{STATUS_LABEL[exp.status]}</span>
                    {exp.hasDate && <span className="il-detail-days">{daysLeftLabel(exp.daysLeft)}</span>}
                  </>}
              </span>
            </div>
            {item('หมายเหตุ', row.Remark)}
          </div>
        </div>

        <div className="wh-detail-divider" />

        <div className="wh-detail-section">
          <span className="wh-detail-section-title">
            <ShieldCheckIcon className="size-4" /> สถานะการยืนยัน
          </span>
          <div className="wh-detail-grid">
            {item('สถานะ', confirmLabel)}
            {item('ผู้ยืนยัน', row.ConfirmedBy)}
            {item('วันเวลาที่ยืนยัน', row.ConfirmedDatetime ? formatThaiDate(row.ConfirmedDatetime) : '')}
            {item('สถานะปิดงาน', completed ? COMPLETED_LABEL : 'ยังไม่เสร็จสิ้น')}
            {completed && item('ผู้กดเสร็จสิ้น', row.CompletedBy)}
            {completed && item('วันที่กดเสร็จสิ้น', row.CompletedAt ? formatThaiDate(row.CompletedAt) : '')}
          </div>
        </div>

        {extraEntries.length > 0 && <>
            <div className="wh-detail-divider" />
            <div className="wh-detail-section">
              <span className="wh-detail-section-title">
                <RectangleStackIcon className="size-4" /> คอลัมน์เพิ่มจากไฟล์
              </span>
              <div className="wh-detail-grid">
                {extraEntries.map(([k, v]) => item(String(k).replace(/^\[\+\]\s*/, ''), v))}
              </div>
            </div>
          </>}

        <div className="wh-detail-meta">
          <span>
            <TagIcon className="size-3.5" /> ไฟล์ {row.FileName || '—'}
          </span>
          <span>
            <ClockIcon className="size-3.5" /> อัปโหลดเมื่อ {row.UploadDate ? formatThaiDate(row.UploadDate) : '—'}
          </span>
        </div>

        </div>

        <div className="wh-modal-actions il-modal-actions">
          <span className="il-checklist-hint" title='แก้สถานะได้โดยพิมพ์ "เสร็จแล้ว" ในช่อง Remark ของไฟล์ Excel แล้วอัปโหลดใหม่'>
            <ChecklistBadge row={row} />
            <span className="il-checklist-hint-text">
              สถานะนี้มาจากช่อง Remark ในไฟล์ Excel
            </span>
          </span>
          <button className="wh-modal-cancel" onClick={onClose}>
            ปิด
          </button>
        </div>
      </div>
    </div>;
}
function computeExpireStatus(expireRaw, withinDays = 30) {
  if (!expireRaw) {
    return {
      hasDate: false,
      expiryDate: null,
      daysLeft: null,
      status: EXPIRY_STATUS.NO_DATE
    };
  }
  const exp = new Date(expireRaw);
  if (Number.isNaN(exp.getTime())) {
    return {
      hasDate: false,
      expiryDate: null,
      daysLeft: null,
      status: EXPIRY_STATUS.NO_DATE
    };
  }
  const atMidnight = d => new Date(d.getFullYear(), d.getMonth(), d.getDate());
  const today = atMidnight(new Date());
  const expDay = atMidnight(exp);
  const daysLeft = Math.round((expDay - today) / 86400000);
  let status;
  if (daysLeft < 0) status = EXPIRY_STATUS.EXPIRED;else if (daysLeft <= withinDays) status = EXPIRY_STATUS.EXPIRING;else status = EXPIRY_STATUS.VALID;
  return {
    hasDate: true,
    expiryDate: expDay,
    daysLeft,
    status
  };
}
function ExportExpiryCell({
  expireDate
}) {
  const exp = computeExpireStatus(expireDate);
  return <div className="il-expiry-cell">
      <span className={EXPIRY_BADGE_CLASS[exp.status]}>{STATUS_LABEL[exp.status]}</span>
      {exp.hasDate && <>
          <span>{formatThaiDate(exp.expiryDate)}</span>
          <span className="il-expiry-days">{daysLeftLabel(exp.daysLeft)}</span>
        </>}
    </div>;
}
const EXTRA_COUNTRY_KEYS = ['country', 'countryname', 'exportcountry', 'ประเทศ', 'ปลายทาง', 'ส่งออกไปประเทศ'];
// ExtraColumnNotice: บอกว่าไฟล์มีคอลัมน์ที่ระบบไม่รู้จัก
//
// ไม่ใช่ข้อผิดพลาด จึงไม่ใช้สีแดง — ข้อมูลถูกเก็บไว้ครบ แค่เอาไปคำนวณต่อไม่ได้
// แต่ก็ไม่ควรเงียบ เพราะคนที่เพิ่งเพิ่มคอลัมน์เข้าไปในไฟล์ ต้องรู้ว่าระบบเห็นมันแล้ว
function ExtraColumnNotice({
  text
}) {
  if (!text) return null;
  return <p className="upload-card-msg wh-upload-msg" style={{
    background: '#eff6ff',
    color: '#1e40af',
    border: '1px solid #bfdbfe'
  }}>
      {text}
    </p>;
}
const EXTRA_COL_LIMIT = 25;
function normExtraKey(k) {
  return String(k).replace(/^\[\+\]\s*/, '').toLowerCase().replace(/[\s_./-]/g, '');
}
function extraLabel(k) {
  return String(k).replace(/^\[\+\]\s*/, '').trim();
}
function parseExtraJson(json) {
  if (!json) return {};
  try {
    const obj = JSON.parse(json);
    if (!obj || typeof obj !== 'object') return {};
    const out = {};
    Object.entries(obj).forEach(([k, v]) => {
      if (EXTRA_COUNTRY_KEYS.includes(normExtraKey(k))) return;
      const val = String(v ?? '').trim();
      if (val) out[extraLabel(k)] = val;
    });
    return out;
  } catch {
    return {};
  }
}
function collectExtraColumns(rows) {
  const counts = new Map();
  rows.forEach(r => {
    Object.keys(parseExtraJson(r.extra_json)).forEach(label => {
      counts.set(label, (counts.get(label) || 0) + 1);
    });
  });
  const labels = Array.from(counts.entries()).sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0])).map(e => e[0]);
  return {
    spread: labels.slice(0, EXTRA_COL_LIMIT),
    overflow: labels.slice(EXTRA_COL_LIMIT)
  };
}
function computeExportExpiry(row, withinDays = 7) {
  return computeExportLicenseDates(row, {
    withinDays
  });
}
function ExportOneMonthExpiryCell({
  row
}) {
  const exp = computeExportExpiry(row);
  return <div className="il-expiry-cell">
      <span className={EXPIRY_BADGE_CLASS[exp.status]}>{STATUS_LABEL[exp.status]}</span>
      {exp.hasDate && <>
          <span>{formatThaiDate(exp.expiryDate)}</span>
          <span className="il-expiry-days">{daysLeftLabel(exp.daysLeft)}</span>
        </>}
    </div>;
}
function ExportLeadTimeCell({
  row
}) {
  const exp = computeExportExpiry(row);
  if (!exp.hasDate) {
    return <div className="il-expiry-cell">
        <span className={LEAD_BADGE_CLASS[LEAD_STATUS.NO_DATE]}>
          {LEAD_STATUS_LABEL[LEAD_STATUS.NO_DATE]}
        </span>
      </div>;
  }
  return <div className="il-expiry-cell">
      <span className={leadBadgeClass(exp)}>{LEAD_STATUS_LABEL[exp.leadStatus]}</span>
      <span>{formatThaiDate(exp.leadDate)}</span>
      <span className={'il-expiry-days' + (exp.leadAlert ? ' il-lead-days-alert' : '')}>
        {leadDaysLabel(exp.leadDaysLeft)}
      </span>
    </div>;
}
function ExportTraceModal({
  row,
  country,
  onClose
}) {
  // สถานะ Checklist มาจากช่อง Remark ในไฟล์ Excel — ไม่มีปุ่มกดเสร็จสิ้นรายรายการแล้ว
  const completed = isExportChecklistDone(row);
  const scrollRef = useDetailSheet(onClose);
  const [loading, setLoading] = useState(true);
  const [data, setData] = useState(null);
  const [err, setErr] = useState(null);
  useEffect(() => {
    let alive = true;
    setLoading(true);
    setErr(null);
    getExportLicenseTrace(row.ID).then(d => alive && setData(d)).catch(e => alive && setErr(e.message || 'โหลดข้อมูลเชื่อมโยงไม่สำเร็จ')).finally(() => alive && setLoading(false));
    return () => {
      alive = false;
    };
  }, [row.ID]);
  const expiryInfo = computeExportExpiry(row);
  const item = (label, value) => value ? <div className="wh-detail-item">
        <span className="wh-detail-label">{label}</span>
        <span className="wh-detail-value">{value}</span>
      </div> : null;
  const itemAlways = (label, value) => <div className="wh-detail-item">
      <span className="wh-detail-label">{label}</span>
      <span className="wh-detail-value">{value || '—'}</span>
    </div>;
  return <div className="wh-modal-overlay il-detail-overlay" onClick={onClose}>
      <div className="wh-modal wh-detail-modal il-detail-sheet" role="dialog" aria-modal="true" aria-labelledby="il-export-detail-title" onClick={e => e.stopPropagation()}>
        <button type="button" className="wh-detail-close" onClick={onClose} aria-label="ปิด">
          <XMarkIcon className="size-4" />
        </button>

        <div className="wh-detail-header">
          <span className="wh-detail-header-icon">
            <TruckIcon className="size-5" />
          </span>
          <div>
            <h3 className="wh-modal-title" id="il-export-detail-title">รายละเอียดใบอนุญาตส่งออก</h3>
            <span className="wh-detail-header-sub">{row.MachineNo || row.ITControllerNo || '—'}</span>
          </div>
        </div>

        <div className="il-detail-scroll" ref={scrollRef}>

        <div className="wh-detail-section">
          <span className="wh-detail-section-title">
            <CubeIcon className="size-4" /> ข้อมูลชิ้นงาน
          </span>
          <div className="wh-detail-grid">
            {item('Machine No', row.MachineNo)}
            {item('IT Controller S/N', row.ITControllerNo)}
            {itemAlways('Serial Number', data?.masterData?.SerialNo)}
            {item('ประเทศปลายทาง', country || NO_COUNTRY_LABEL)}
          </div>
        </div>

        <div className="wh-detail-divider" />

        <div className="wh-detail-section">
          <span className="wh-detail-section-title">
            <DocumentTextIcon className="size-4" /> ข้อมูลใบขนส่งออก
          </span>
          <div className="wh-detail-grid">
            {item('Invoice No.', row.InvoiceNo)}
            {item('Invoice Date', row.InvoiceDate ? formatThaiDate(row.InvoiceDate) : '')}
            {item('Export Entry', row.ExportEntry)}
            {item('Export License', row.ExportLicenseNo)}
            {itemAlways('จำนวนที่ต่ออายุ', renewalRoundLabel(row))}
            {itemAlways('Import License', row.ImportLicenseNo)}
            {item('วันที่นำออกใบอนุญาต', row.IssueDate ? formatThaiDate(row.IssueDate) : '')}
            <div className="wh-detail-item">
              <span className="wh-detail-label">หมดอายุ (1 เดือน)</span>
              <span className="wh-detail-value il-detail-status">
                {completed ? <>
                    <span className="il-badge il-badge-complete">
                      <CheckBadgeIcon className="size-3.5" />
                      {COMPLETED_LABEL}
                    </span>
                    {expiryInfo.hasDate && <span className="il-detail-days">หมดอายุ {formatThaiDate(expiryInfo.expiryDate)}</span>}
                  </> : <>
                    <span className={EXPIRY_BADGE_CLASS[expiryInfo.status]}>{STATUS_LABEL[expiryInfo.status]}</span>
                    {expiryInfo.hasDate && <span className="il-detail-days">{formatThaiDate(expiryInfo.expiryDate)} · {daysLeftLabel(expiryInfo.daysLeft)}</span>}
                  </>}
              </span>
            </div>
            <div className="wh-detail-item">
              <span className="wh-detail-label">{`Lead time (${EXPORT_LICENSE_LEAD_DAYS} วัน)`}</span>
              <span className="wh-detail-value il-detail-status">
                {completed ? <>
                    <span className="il-badge il-badge-complete">
                      <CheckBadgeIcon className="size-3.5" />
                      {COMPLETED_LABEL}
                    </span>
                    {expiryInfo.hasDate && <span className="il-detail-days">ครบกำหนดยื่น {formatThaiDate(expiryInfo.leadDate)}</span>}
                  </> : <>
                    <span className={leadBadgeClass(expiryInfo)}>{LEAD_STATUS_LABEL[expiryInfo.leadStatus]}</span>
                    {expiryInfo.hasDate && <span className="il-detail-days">{formatThaiDate(expiryInfo.leadDate)} · {leadDaysLabel(expiryInfo.leadDaysLeft)}</span>}
                  </>}
              </span>
            </div>
            {completed && item('ผู้กดเสร็จสิ้น', row.CompletedBy)}
            {completed && item('วันที่กดเสร็จสิ้น', row.CompletedAt ? formatThaiDate(row.CompletedAt) : '')}
            {item("Date Ass'y", row.AssemblyDate ? formatThaiDate(row.AssemblyDate) : '')}
            {item('Remark', row.Remark)}
          </div>
        </div>

        {loading && <p className="il-detail-note">กำลังโหลดข้อมูลที่เชื่อมโยง...</p>}
        {err && <p className="il-detail-note il-detail-note-err">{err}</p>}

        {!loading && !err && <>
            <div className="wh-detail-divider" />
            <div className="wh-detail-section">
              <span className="wh-detail-section-title">
                <ShieldCheckIcon className="size-4" /> Import License
              </span>
              {data?.importLicense ? <div className="wh-detail-grid">
                  {item('เลขใบอนุญาตนำเข้า', data.importLicense.LicenseNo)}
                  {item('Invoice นำเข้า', data.importLicense.InvoiceNo)}
                  {item('รุ่น', data.importLicense.Model)}
                  {item('ประเทศส่งออก', data.importLicense.ExportCountry)}
                  {item('สถานะยืนยัน', data.importLicense.ConfirmStatus)}
                  {itemAlways('ผู้ยืนยัน (WH)', data.importLicense.ConfirmedBy)}
                  {itemAlways('วันที่เช็ค', data.importLicense.ConfirmedDatetime ? formatThaiDate(data.importLicense.ConfirmedDatetime) : '')}
                </div> : <p className="il-detail-note">
                  ไม่พบใบอนุญาตนำเข้า
                </p>}
            </div>

            {Array.isArray(data?.renewalHistory) && data.renewalHistory.length > 1 && <>
                <div className="wh-detail-divider" />
                <div className="wh-detail-section">
                  <span className="wh-detail-section-title">
                    <ArrowPathIcon className="size-4" /> ประวัติการต่ออายุ ({data.renewalHistory.length} ครั้ง)
                  </span>
                  <div className="il-renewal-history">
                    {data.renewalHistory.map(h => <div key={h.ID} className={'il-renewal-row' + (h.Current ? ' il-renewal-current' : '')}>
                        <span className="il-renewal-round">ครั้งที่ {h.RenewalRound}</span>
                        <span className="il-renewal-license il-mono">{h.ExportLicenseNo || '—'}</span>
                        <span className="il-renewal-date">
                          {h.IssueDate ? formatThaiDate(h.IssueDate) : '—'}
                          {h.ExpireDate ? ` → ${formatThaiDate(h.ExpireDate)}` : ''}
                        </span>
                        <span className={h.Completed ? 'il-badge il-badge-ok' : 'il-badge il-badge-muted'}>
                          {h.Completed ? CHECKLIST_LABEL_DONE : CHECKLIST_LABEL_OPEN}
                        </span>
                      </div>)}
                  </div>
                </div>
              </>}

            {data?.mfgAssembly && <>
                <div className="wh-detail-divider" />
                <div className="wh-detail-section">
                  <span className="wh-detail-section-title">
                    <WrenchScrewdriverIcon className="size-4" /> MFG Assembly (ผลตรวจตอนประกอบ)
                  </span>
                  <div className="wh-detail-grid">
                    {item('สถานะ', data.mfgAssembly.Status)}
                    {item('Machine No (ที่ประกอบ)', data.mfgAssembly.MachineNo)}
                    {itemAlways('วันที่ประกอบ', data.mfgAssembly.DateAssembly ? formatThaiDate(data.mfgAssembly.DateAssembly) : '')}
                    {itemAlways('Check By (MFG)', data.mfgAssembly.CreatedBy)}
                  </div>
                </div>
              </>}
          </>}

        </div>

        <div className="wh-modal-actions il-modal-actions">
          <button className="wh-modal-cancel" onClick={onClose}>
            ปิด
          </button>
        </div>
      </div>
    </div>;
}
