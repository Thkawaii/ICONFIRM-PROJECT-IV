import { useCallback, useEffect, useMemo, useState } from 'react';
import AppShell from '../components/AppShell.jsx';
import FileDropZone from '../components/Filedropzone.jsx';
import SelectField from '../components/Selectfield.jsx';
import { PreviewResult, ChangePreview } from '../components/FormatTools.jsx';
import { WH_NAV_ITEMS } from './Importlicensepage.jsx';
import { uploadImportLicense, previewImportLicense } from '../api/importLicense.js';
import { getLicenseOverview, getLicenseDetail, deleteLicenseOverviewEntry, uploadLicenseRenewalHistory, previewLicenseRenewalHistory, licenseDaysLabel, licenseDaysHint, LICENSE_STATUS_CLASS, LICENSE_TYPE_LABEL, COUNTRY_ALL, COUNTRY_NONE, COUNTRY_MULTI, COUNTRY_MULTI_LABEL, isMultiCountry, countryKey, countryKeys, countryLabel, countryDisplay, clearLicenseOverview, setLicenseIssueDate, CLEAR_SCOPE, CLEAR_SCOPE_OPTIONS, TYPE_FILTER_ALL, STATUS_FILTER_ALL, TYPE_FILTER_OPTIONS, STATUS_FILTER_OPTIONS, matchTypeFilter, matchStatusFilter } from '../api/licenseOverview.js';
import { formatThaiDate } from '../lib/licenseExpiry.js';
import { buildStyledXlsxWorkbookBlob, downloadBlob } from '../lib/xlsx.js';
import { inPeriod, periodRangeLabel, periodFileTag } from '../lib/dateRange.js';
import PeriodRangePicker from '../components/PeriodRangePicker.jsx';
import DatePickerField from '../components/DatePickerField.jsx';
import { confirmDelete, toastError, toastSuccess } from '../lib/toast.js';
import { ArrowPathIcon, ChevronDoubleLeftIcon, ChevronDoubleRightIcon, ChevronLeftIcon, ChevronRightIcon, ArrowDownTrayIcon, ClockIcon, CloudArrowUpIcon, DocumentTextIcon, ReceiptPercentIcon, Squares2X2Icon, TrashIcon, XCircleIcon, XMarkIcon } from '../components/icons.jsx';

// ---------------------------------------------------------------------------
// License Overview
//
// 1. Upload — แยก 3 แท็บตามไฟล์ที่ผู้ใช้ทำงานจริง (Import / Export / Renewal)
//    ไม่บังคับรวมไฟล์ และใช้ API เดิมของ Import / Export ทุกอย่าง
// 2. Overview — ดู Import + Export ในตารางเดียว กรองได้ตามประเภทและสถานะ
// 3. Detail — กดดูใบไหนก็เห็นประวัติการต่ออายุทั้งโซ่ (เลขใบเก่า → เลขใบใหม่)
// ---------------------------------------------------------------------------

const DASH = '—';

// คอลัมน์ NO. ในชีตต่ออายุเขียนว่า "Completed 01" สำหรับกลุ่มที่ปิดงานแล้ว
// สถานะปิดงานมีป้ายบอกอยู่แล้ว ในช่องลำดับจึงเหลือแค่ตัวเลขพอ
// autoCloseHint: ข้อความเตือนใบที่หมดอายุแล้วแต่ยังไม่ถึงคิวปิด
//
// อ่านต่อจากบรรทัด "หมดอายุไปแล้ว N วัน" ด้านบน จึงขึ้นต้นด้วยเงื่อนไข
// ("หากไม่ต่ออายุ...") เพื่อบอกว่าผู้ใช้ต้องทำอะไรภายในเมื่อไหร่ ไม่ใช่แค่
// บรรยายว่าระบบจะทำอะไร
function autoCloseHint(days) {
  if (!(days > 0)) return '';
  // "ภายใน 1 วัน" ตีความได้สองแบบ (วันนี้ หรือพรุ่งนี้) จึงเขียนให้ชัดไปเลย
  if (days === 1) return 'หากไม่ต่ออายุภายในวันนี้ ใบอนุญาตจะปิดอัตโนมัติ';
  return `หากไม่ต่ออายุภายใน ${days} วัน ใบอนุญาตจะปิดอัตโนมัติ`;
}
function groupNoLabel(v) {
  const s = String(v || '').trim();
  if (!s) return '';
  const m = s.match(/^completed\s*(.*)$/i);
  return m ? m[1].trim() || s : s;
}

const EXPORT_TYPE_OPTIONS = [{
  value: 'ALL',
  label: 'ใบนำเข้า + ใบนำออก'
}, {
  value: 'IMPORT',
  label: 'ใบนำเข้า'
}, {
  value: 'EXPORT',
  label: 'ใบนำออก'
}];

// แต่ละแท็บผูกกับ API ของไฟล์ประเภทนั้น — ของเดิมไม่ถูกแตะ
//
// ไม่มีตัวเลือก Export License แล้ว ไฟล์ Serial Allocation ไม่ได้ถูกใช้ในหน้านี้
// เลขเครื่องที่ไปเชื่อมกับ WH / MFG สร้างจาก Planning · Engine · WH · MasterData
// ไฟล์นั้นเป็นแค่คอลัมน์อ้างอิงเสริม ไม่ใช่ต้นทางของเลขเครื่อง
// ถ้าวันหนึ่งต้องอัปจริง ยังทำได้ที่หน้าใบอนุญาตนำออก
const UPLOAD_TABS = [{
  key: 'import',
  label: 'License (Import / Export)',
  icon: <DocumentTextIcon className="size-4" />,
  dropLabel: 'อัปโหลดบัญชีใบอนุญาตนำเข้า (Import.xlsx)',
  preview: previewImportLicense,
  upload: uploadImportLicense
}, {
  key: 'renewal',
  label: 'Renewal License',
  icon: <ArrowPathIcon className="size-4" />,
  dropLabel: 'อัปโหลดไฟล์ต่ออายุ (Renewal.xlsx)',
  preview: previewLicenseRenewalHistory,
  upload: uploadLicenseRenewalHistory
}];
// ป้ายสถานะ — บอกจำนวนวันในป้ายเลย
// ใกล้หมดอายุ N วัน / เลยกำหนด N วัน / ปกติ N วัน
function statusBadge(status, daysLeft) {
  const label = licenseDaysLabel(status, daysLeft);
  return <span className={LICENSE_STATUS_CLASS[status] || 'il-badge il-badge-muted'}>
      {label}
    </span>;
}

// ---------------------------------------------------------------------------
// ส่วนอัปโหลด — เลือกประเภทไฟล์จาก dropdown เดียว แล้วอัปโหลดได้เลย
//
// ไฟล์ทั้งสองประเภทยังยิงเข้า API เดิมของแต่ละฝั่งเหมือนเดิมทุกอย่าง
// เปลี่ยนแค่วิธีเลือกให้เหลือขั้นตอนเดียว: เลือกประเภท → วางไฟล์ → อัปโหลด
// ---------------------------------------------------------------------------
function UploadPanel({
  onUploaded
}) {
  const [kindKey, setKindKey] = useState('import');
  const [file, setFile] = useState(null);
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState(null);
  const active = UPLOAD_TABS.find(t => t.key === kindKey) || UPLOAD_TABS[0];
  const options = UPLOAD_TABS.map(t => ({
    value: t.key,
    label: t.label
  }));

  // เปลี่ยนประเภทไฟล์ = ล้างไฟล์ที่เลือกไว้ทิ้ง
  // กันอัปโหลดไฟล์ Import เข้า API ของ Export เพราะลืมเปลี่ยน
  function switchKind(key) {
    setKindKey(key);
    setFile(null);
    setResult(null);
  }
  async function run(kind) {
    if (!file || busy) return;
    setBusy(true);
    setResult(null);
    try {
      const data = kind === 'preview' ? await active.preview(file) : await active.upload(file);
      setResult({
        kind,
        data
      });
      if (kind === 'upload') {
        toastSuccess(data?.message || 'อัปโหลดสำเร็จ');
        setFile(null);
        await onUploaded?.();
      }
    } catch (err) {
      setResult({
        kind,
        error: err.message || 'ไม่สำเร็จ'
      });
      if (kind === 'upload') toastError(err.message || 'อัปโหลดไม่สำเร็จ');
    } finally {
      setBusy(false);
    }
  }
  const skippedCount = Number(result?.data?.skipped) || 0;
  return <div className="wh-upload-card lo-upload-card">
      <div className="lo-upload-pick">
        <label className="lo-upload-pick-label" htmlFor="lo-upload-kind">
          <CloudArrowUpIcon className="size-4" /> Upload License
        </label>
        <div className="lo-upload-pick-select">
          <SelectField id="lo-upload-kind" value={kindKey} onChange={switchKind} options={options} disabled={busy} />
        </div>
      </div>

      <div className="fdz-row">
        <FileDropZone file={file} onSelect={f => {
        setFile(f);
        setResult(null);
      }} accept=".xlsx,.xls,.csv" label={active.dropLabel} disabled={busy} />
        <button className="wh-modal-cancel" onClick={() => run('preview')} disabled={busy || !file}>
          {busy ? 'กำลังทำงาน...' : 'ตรวจสอบก่อนอัปโหลด'}
        </button>
        <button className="wh-issue-btn" onClick={() => run('upload')} disabled={busy || !file}>
          {busy ? 'กำลังอัปโหลด...' : 'อัปโหลด'}
        </button>
      </div>

      {result?.error && <p className="upload-card-msg upload-card-msg-err wh-upload-msg">{result.error}</p>}

      {/* สรุปผลบรรทัดเดียว — ไม่ไล่รายการปัญหาทีละแถว
          ไฟล์จริงมีแถวที่ข้ามได้เป็นพันแถว (เช่น เครื่องที่ไม่มี IT Controller)
          ถ้าขึ้นทุกบรรทัดจะยาวจนกลบข้อความที่สำคัญจริง ๆ */}
      {result?.data && !result.error && result.data.headerFound !== false && <p className="upload-card-msg upload-card-msg-ok wh-upload-msg">
          {result.data.message || (result.kind === 'preview' ? 'ตรวจไฟล์แล้ว' : 'อัปโหลดสำเร็จ')}
          {/* หน้า Renewal ข้อความจาก backend บอกจำนวน "ข้าม" ไว้ในตัวเองอยู่แล้ว (เช่น
              "ประวัติการต่ออายุใหม่: ไม่มีในไฟล์นี้") ถ้าโชว์ซ้ำอีกบรรทัดจะงงว่าทำไมบอก 2 รอบ */}
          {skippedCount > 0 && kindKey !== 'renewal' && <span className="lo-upload-skip"> ข้าม {skippedCount} แถวที่ใช้ไม่ได้</span>}
        </p>}

      {/* ตรวจสอบก่อนอัปโหลด — กางให้เห็นว่าแถวไหนจะถูกเพิ่ม อัปเดต หรือลบ
          ใช้ตัวเดียวกับหน้าใบอนุญาตนำเข้า/นำออก จะได้อ่านแบบเดียวกันทั้งระบบ */}
      {result?.kind === 'preview' && result?.data && !result.error && (result.data.summary ? <ChangePreview result={result.data} /> : <PreviewResult result={result.data} />)}
    </div>;
}

// ---------------------------------------------------------------------------
// Renewal History — เลขใบเก่า → เลขใบใหม่ ทุกครั้งที่ต่ออายุ
// ---------------------------------------------------------------------------
// buildChainRows: ทำเครื่องหมายว่าแถวไหนเป็นจุดเริ่มของสายประเทศใหม่
//
// ใบที่แบ่งโควต้าหลายประเทศ แต่ละประเทศเดินต่ออายุของตัวเอง ครั้งที่จึงเริ่มนับใหม่
// ถ้าปล่อยให้ทุกแถวติดกันหมด จะอ่านไม่ออกว่า "ครั้งที่ 1" อันไหนเป็นของประเทศไหน
// จึงขีดเส้นคั่นตรงที่ประเทศเปลี่ยน และทำชื่อประเทศที่ซ้ำกับแถวบนให้จางลง
//
// ใบที่มีประเทศเดียวทั้งตารางไม่ต้องขีดอะไร จะได้ไม่รกสายตา
function buildChainRows(steps) {
  let prev = null;
  let groups = 0;
  const list = steps.map(s => {
    if (s.isNote) return {
      step: s,
      groupStart: false,
      sameAsPrev: false
    };
    const key = countryKey(s.country);
    if (prev === null || key !== prev) groups++;
    const groupStart = prev !== null && key !== prev;
    const sameAsPrev = prev !== null && key === prev;
    prev = key;
    return {
      step: s,
      groupStart,
      sameAsPrev
    };
  });
  if (groups <= 1) return list.map(r => ({
    ...r,
    groupStart: false,
    sameAsPrev: false
  }));
  return list;
}

function LicenseDetailModal({
  target,
  onClose,
  onChanged
}) {
  const [data, setData] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [issueDate, setIssueDate] = useState('');
  const [savingDate, setSavingDate] = useState(false);
  useEffect(() => {
    if (!target) return;
    let cancelled = false;
    (async () => {
      setLoading(true);
      setError('');
      try {
        const res = await getLicenseDetail({
          type: target.licenseType,
          licenseNo: target.currentLicenseNo
        });
        if (!cancelled) setData(res);
      } catch (err) {
        if (!cancelled) setError(err.message || 'โหลดรายละเอียดไม่สำเร็จ');
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [target]);
  if (!target) return null;
  const detail = data?.detail || target;
  const steps = data?.steps || [];
  const ledgerRows = Number(data?.ledgerRows ?? 0);

  const chainRows = buildChainRows(steps);

  // ใบที่แบ่งโควต้าหลายประเทศ ตัวเลขสรุปใบเดียวเล่าได้ไม่ครบ ต้องกางรายประเทศ
  const multiBranch = (detail.branches?.length ?? 0) > 1;

  // ไฟล์บัญชีหมายเลขเครื่องหลายไฟล์ไม่มีคอลัมน์วันที่ออกใบอนุญาต
  // เมื่อไม่มีวันที่ ระบบคำนวณวันหมดอายุไม่ได้ และไม่มีอะไรให้แจ้งเตือน
  // จึงเปิดให้กรอกเองตรงนี้ กรอกครั้งเดียวได้ทั้งใบ
  async function saveIssueDate() {
    if (!issueDate || savingDate || !detail) return;
    setSavingDate(true);
    try {
      const res = await setLicenseIssueDate({
        type: detail.licenseType,
        licenseNo: detail.currentLicenseNo,
        issueDate
      });
      toastSuccess(res?.message || 'บันทึกแล้ว');
      setIssueDate('');
      await onChanged?.();
      onClose?.();
    } catch (err) {
      toastError(err.message || 'บันทึกไม่สำเร็จ');
    } finally {
      setSavingDate(false);
    }
  }
  return <div className="wh-modal-overlay" onClick={onClose}>
      <div className="wh-modal wh-detail-modal lo-detail-modal" onClick={e => e.stopPropagation()}>
        <div className="wh-detail-header">
          <span className="wh-detail-header-icon">
            <DocumentTextIcon className="size-5" />
          </span>
          <div>
            <h3 className="wh-modal-title">ใบอนุญาต</h3>
            <span className="wh-detail-header-sub">
              {LICENSE_TYPE_LABEL[detail.licenseType] || detail.licenseType}
            </span>
          </div>
          <button type="button" className="wh-detail-close" onClick={onClose} aria-label="ปิด">
            <XMarkIcon className="size-4" />
          </button>
        </div>

        {/* แบ่งเป็นสามกลุ่มตามสิ่งที่คนมาหา: ตัวใบ · อายุ · จำนวน
            ของเดิมเรียงต่อกันรวดเดียว เลขใบปัจจุบันกับเลขใบต้นฉบับจึงอยู่คนละมุมจอ
            ทั้งที่เป็นข้อมูลคู่กันที่คนเอามาเทียบกันบ่อยที่สุด */}
        <div className="lo-detail-cols">
        <div className="wh-detail-section lo-sec">
          <div className="wh-detail-section-title">ใบอนุญาต</div>
          <div className="wh-detail-grid lo-detail-grid">
            <div className="wh-detail-item">
              <span className="wh-detail-label">เลขใบอนุญาตปัจจุบัน</span>
              {/* ใบที่แบ่งโควต้าหลายประเทศไม่มี "ใบปัจจุบัน" ใบเดียว
                  แต่ละประเทศถือใบนำออกของตัวเองคนละใบ ต้องบอกให้ครบ */}
              {multiBranch ? <span className="wh-detail-value lo-current-list">
                  {detail.branches.map((b, i) => <span className="lo-current-row" key={b.exportLicenseNo || i}>
                      <span className="lo-current-country">{countryDisplay(b.country)}</span>
                      <span className="mono">{b.exportLicenseNo || DASH}</span>
                    </span>)}
                </span> : <span className="wh-detail-value mono">{detail.currentLicenseNo || DASH}</span>}
            </div>
            <div className="wh-detail-item">
              <span className="wh-detail-label">เลขใบอนุญาตต้นฉบับ</span>
              <span className="wh-detail-value mono">{detail.originalLicenseNo || DASH}</span>
            </div>
            <div className="wh-detail-item">
              <span className="wh-detail-label">ประเภทใบอนุญาต</span>
              <span className="wh-detail-value">
                {LICENSE_TYPE_LABEL[detail.licenseType] || detail.licenseType}
              </span>
            </div>
            <div className="wh-detail-item">
              <span className="wh-detail-label">ประเทศ</span>
              <span className="wh-detail-value">
                {detail.country ? countryDisplay(detail.country) : DASH}
              </span>
            </div>
          </div>
        </div>

        <div className="wh-detail-section lo-sec">
          <div className="wh-detail-section-title">อายุและการต่ออายุ</div>
          <div className="wh-detail-grid lo-detail-grid">
            <div className="wh-detail-item">
              <span className="wh-detail-label">สถานะปัจจุบัน</span>
              <span className="wh-detail-value">{statusBadge(detail.status, detail.daysLeft)}</span>
            </div>
            <div className="wh-detail-item">
              <span className="wh-detail-label">วันหมดอายุปัจจุบัน</span>
              <span className="wh-detail-value">
                {formatThaiDate(detail.expiryDate)}
                {/* วันที่ด้านบนคือสายที่ใกล้หมดที่สุด สายอื่นหมดคนละวัน */}
                {multiBranch && <span className="lo-branch-note">
                    {detail.branches.map(b => `${countryDisplay(b.country)} ${b.expireDate ? formatThaiDate(b.expireDate) : DASH}`).join(' · ')}
                  </span>}
              </span>
            </div>
            <div className="wh-detail-item">
              <span className="wh-detail-label">จำนวนครั้งที่ต่ออายุ</span>
              <span className="wh-detail-value">
                <strong>{detail.renewalCount ?? 0}</strong> ครั้ง
                {/* แต่ละสายต่อไม่เท่ากัน ตัวเลขด้านบนคือสายที่ยาวที่สุด */}
                {multiBranch && <span className="lo-branch-note">
                    {detail.branches.map(b => `${countryDisplay(b.country)} ${b.renewalCount ?? 0}`).join(' · ')}
                  </span>}
              </span>
            </div>
            {detail.leadDate && <div className="wh-detail-item">
              <span className="wh-detail-label">ครบกำหนดยื่นต่ออายุ</span>
              <span className={'wh-detail-value' + (detail.leadDaysLeft < 0 ? ' lo-lead-over' : '')}>
                {formatThaiDate(detail.leadDate)}
                <span className="lo-lead-note">
                  {detail.leadDaysLeft < 0 ? `เลยกำหนดยื่นมาแล้ว ${Math.abs(detail.leadDaysLeft)} วัน` : `ยื่นล่วงหน้า 15 วันทำการ · เหลือ ${detail.leadDaysLeft} วัน`}
                </span>
              </span>
            </div>}
          </div>
        </div>

        <div className="wh-detail-section lo-sec">
          <div className="wh-detail-section-title">จำนวนเครื่อง</div>
          <div className="wh-detail-grid lo-detail-grid">
            <div className="wh-detail-item">
              <span className="wh-detail-label">จำนวนเครื่อง</span>
              <span className="wh-detail-value">
                {detail.items > 0 ? <>
                    <strong>{detail.items}</strong> เครื่อง
                    {detail.completedItems > 0 && ` (เสร็จสิ้น ${detail.completedItems})`}
                  </> : detail.quota > 0 ? <>
                    <strong>{detail.quota}</strong> เครื่อง
                  </> : DASH}
              </span>
            </div>

            {/* ตัวเลขโควต้าจากชีต "ต่ออายุ" — มีเฉพาะใบที่อยู่ในตารางทะเบียน */}
            {detail.hasLedger && <>
              <div className="wh-detail-item">
                <span className="wh-detail-label">ลำดับในชีตต่ออายุ</span>
                <span className="wh-detail-value">{groupNoLabel(detail.groupNo) || DASH}</span>
              </div>
              <div className="wh-detail-item">
                <span className="wh-detail-label">สต็อกใบอนุญาตส่งออก</span>
                <span className="wh-detail-value">
                  <strong>{detail.quota ?? 0}</strong> เครื่อง
                </span>
              </div>
              <div className="wh-detail-item">
                <span className="wh-detail-label">จำนวนคงเหลือ</span>
                <span className={'wh-detail-value' + (detail.remain < 0 ? ' lo-remain-over' : '')}>
                  <strong>{detail.remain ?? 0}</strong> เครื่อง
                  {detail.remain < 0 && <span className="lo-done-count lo-remain-over">ใช้เกินโควต้า</span>}
                </span>
              </div>

            </>}
          </div>
        </div>
        </div>

        {/* ใบนำเข้าใบเดียวแบ่งโควต้าออกหลายประเทศพร้อมกันได้
            ยอดรวมอย่างเดียวไม่พอ ต้องเห็นว่าแต่ละประเทศได้เท่าไหร่
            รายการนี้มีสี่ช่องต่อบรรทัด กว้างเกินคอลัมน์เดียว จึงแยกมาเต็มความกว้าง */}
        {detail.hasLedger && detail.branches?.length > 1 && <div className="wh-detail-section lo-branch-section">
            <div className="wh-detail-section-title">แบ่งตามประเทศ</div>
            <div className="lo-branch-list">
              {detail.branches.map((b, i) => <div className="lo-branch-row" key={b.exportLicenseNo || i}>
                  <span className="lo-branch-country">{countryDisplay(b.country) || DASH}</span>
                  <span className="lo-branch-no mono">{b.exportLicenseNo || DASH}</span>
                  <span className="lo-branch-num">
                    <strong>{b.remain ?? 0}</strong>
                    <span className="lo-branch-unit">/ {b.stock ?? 0} เครื่อง</span>
                  </span>
                  {/* ช่อง REMAIN บอกว่าเหลือของบนใบนี้กี่เครื่อง
                      0 = ตัดออกหมดแล้ว · มากกว่า 0 = ของที่ขายไม่ออก ค้างอยู่บนใบ */}
                  <span className="lo-done-count">
                    {(b.remain ?? 0) <= 0 ? 'นำออกหมดแล้ว' : 'รายการคงค้าง'}
                  </span>
                </div>)}
            </div>
          </div>}

        {/* ไม่มีวันหมดอายุ = ไฟล์ไม่มีวันที่ออกใบ — ให้กรอกเองได้ ไม่งั้นแจ้งเตือนไม่ทำงาน */}
        {!detail.expiryDate && detail.items > 0 && <div className="lo-issue-date">
            <span className="lo-issue-date-label">
              ใบนี้ยังไม่มีวันหมดอายุ เพราะไฟล์ไม่มีวันที่ออกใบอนุญาต
            </span>
            <div className="lo-issue-date-row">
              <div className="lo-issue-date-field">
                <DatePickerField value={issueDate} onChange={setIssueDate} placeholder="— เลือกวันที่ออกใบ —" />
              </div>
              <button className="wh-issue-btn" onClick={saveIssueDate} disabled={!issueDate || savingDate}>
                {savingDate ? 'กำลังบันทึก...' : 'บันทึกและคำนวณวันหมดอายุ'}
              </button>
            </div>
            <span className="lo-issue-date-hint">
              {detail.licenseType === 'IMPORT' ? 'ใบนำเข้า: วันหมดอายุ = วันที่ออกใบ + 6 เดือน' : 'ใบนำออก: วันหมดอายุ = วันที่ออกใบ + 1 เดือน'}
            </span>
          </div>}



        <div className="wh-detail-divider" />

        <div className="wh-detail-section lo-chain-section">
          <span className="wh-detail-section-title">
            <ArrowPathIcon className="size-4" /> ประวัติการต่ออายุ
          </span>

          {loading && <p className="wh-empty-cell">กำลังโหลด...</p>}
          {error && <p className="form-error">{error}</p>}

          {/* STOCK / คงเหลือ อยู่ในชีตต่ออายุ ไม่ได้อยู่ในประวัติการต่ออายุ
              ถ้าขึ้นขีดทั้งตาราง ต้องแยกให้ออกว่าเป็นเพราะตารางทะเบียนว่าง
              หรือเป็นเพราะใบชุดนี้ไม่มีในตารางทะเบียน — บอกสาเหตุไปเลย ไม่ให้เดาเอง */}
          {!loading && !error && steps.length > 0 && !steps.some(s => s.hasLedger) && <p className="lo-steps-hint">
              {ledgerRows === 0 ? 'ยังไม่มีข้อมูลชีตต่ออายุในระบบเลย (ตารางทะเบียน 0 แถว) — อัปโหลดไฟล์อีกครั้ง แล้ว สต็อกใบอนุญาตส่งออก และคงเหลือจะขึ้นตัวเลข' : `ตารางทะเบียนมี ${ledgerRows} แถว แต่ไม่พบเลขใบชุดนี้อยู่ในนั้น — ตรวจว่าไฟล์ที่อัปล่าสุดมีใบเหล่านี้อยู่ในชีตต่ออายุหรือไม่`}
            </p>}

          {!loading && !error && <div className="wh-table-card lo-chain-scroll">
              <table className="wh-table">
                <thead>
                  <tr>
                    <th>ครั้ง</th>
                    <th>ประเทศ</th>
                    <th>เลขใบเดิม</th>
                    <th>เลขใบใหม่</th>
                    <th>วันที่ต่ออายุ</th>
                    <th>วันหมดอายุใหม่</th>
                    <th className="lo-th-num">สต็อกใบอนุญาตส่งออก</th>
                    <th className="lo-th-num">คงเหลือ</th>
                  </tr>
                </thead>
                <tbody>
                  {chainRows.map(({ step: s, groupStart, sameAsPrev }, i) => s.isNote ?
                  /* แถวที่ผู้ใช้จดข้อความแทรกไว้ในไฟล์ — ไม่ใช่การต่ออายุ แต่ต้องเห็น */
                  <tr key={i} className="lo-note-row">
                      <td className="wh-cell-head">หมายเหตุ</td>
                      <td colSpan={7}>{s.note}</td>
                    </tr> : <tr key={i} className={groupStart ? 'lo-chain-group-start' : undefined}>
                      <td className="wh-cell-head">{s.round === 0 ? 'ต้นฉบับ' : s.round}</td>
                      {/* ใบนำเข้าใบเดียวแบ่งโควต้าได้หลายประเทศ แต่ละประเทศเดินต่ออายุของตัวเอง
                          ประวัติจึงมีใบของสองประเทศปนกัน ต้องบอกว่าแถวไหนของประเทศไหน
                          ไม่งั้นตัวเลขสต็อก/คงเหลือ อ่านไม่ออกว่าเป็นของใคร */}
                      <td className={sameAsPrev ? 'lo-chain-country-repeat' : 'lo-chain-country'}>
                        {s.country ? countryDisplay(s.country) : DASH}
                      </td>
                      <td className="il-mono">{s.oldLicenseNo || DASH}</td>
                      <td className="il-mono">{s.newLicenseNo || DASH}</td>
                      <td>{s.renewalDate ? formatThaiDate(s.renewalDate) : DASH}</td>
                      <td>{s.expireDate ? formatThaiDate(s.expireDate) : DASH}</td>
                      <td className="lo-cell-num">{s.hasLedger ? s.stock : DASH}</td>
                      <td className={'lo-cell-num' + (s.hasLedger && s.remain < 0 ? ' lo-remain-over' : '')}>
                        {s.hasLedger ? s.remain : DASH}
                      </td>
                    </tr>)}
                  {chainRows.length === 0 && <tr>
                      <td colSpan={8} className="wh-empty-cell">
                        ยังไม่มีประวัติการต่ออายุของใบนี้
                      </td>
                    </tr>}
                </tbody>
              </table>
            </div>}
        </div>

        <div className="wh-modal-actions">
          <button className="wh-modal-cancel" onClick={onClose}>
            ปิด
          </button>
        </div>
      </div>
    </div>;
}

// ---------------------------------------------------------------------------
// ล้างข้อมูลทั้งหมด — เลือกก่อนว่าจะล้างฝั่งไหน แล้วค่อยยืนยันอีกชั้น
//
// เป็นงานที่กู้คืนไม่ได้ จึงบังคับให้เลือกขอบเขตอย่างชัดเจน
// ไม่ทำเป็นปุ่มเดียวกดแล้วลบทันที
// ---------------------------------------------------------------------------
function ClearDataModal({
  open,
  busy,
  onClose,
  onClear
}) {
  const [scope, setScope] = useState(CLEAR_SCOPE.IMPORT);
  if (!open) return null;
  const active = CLEAR_SCOPE_OPTIONS.find(o => o.value === scope);
  return <div className="wh-modal-overlay" onClick={busy ? undefined : onClose}>
      <div className="wh-modal wh-detail-modal lo-clear-modal" onClick={e => e.stopPropagation()}>
        <button type="button" className="wh-detail-close" onClick={onClose} aria-label="ปิด" disabled={busy}>
          <XMarkIcon className="size-4" />
        </button>

        <div className="wh-detail-header">
          <span className="wh-detail-header-icon lo-clear-icon">
            <TrashIcon className="size-5" />
          </span>
          <div>
            <h3 className="wh-modal-title">ลบข้อมูลใบอนุญาต</h3>
          </div>
        </div>

        <div className="lo-clear-options">
          {CLEAR_SCOPE_OPTIONS.map(o => <button key={o.value} type="button" className={'lo-clear-option' + (o.value === scope ? ' lo-clear-option-active' : '')} onClick={() => setScope(o.value)} disabled={busy}>
              <span className="lo-clear-option-radio" aria-hidden="true" />
              <span className="lo-clear-option-text">
                <span className="lo-clear-option-label">
                  {o.label}
                </span>
              </span>
            </button>)}
        </div>

        <div className="wh-modal-actions">
          <button className="wh-modal-cancel" onClick={onClose} disabled={busy}>
            ยกเลิก
          </button>
          <button className="tsf-action-btn lo-clear-confirm" onClick={() => onClear(active)} disabled={busy}>
            <TrashIcon className="size-4" />
            {busy ? 'กำลังลบ...' : 'ลบข้อมูล'}
          </button>
        </div>
      </div>
    </div>;
}

// ---------------------------------------------------------------------------
// หน้าหลัก
// ---------------------------------------------------------------------------

export default function LicenseOverviewPage() {
  const [rows, setRows] = useState([]);
  const [summary, setSummary] = useState(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [typeFilter, setTypeFilter] = useState(TYPE_FILTER_ALL);
  const [statusFilter, setStatusFilter] = useState(STATUS_FILTER_ALL);
  const [search, setSearch] = useState('');
  const [country, setCountry] = useState(COUNTRY_ALL);
  const [pageSize, setPageSize] = useState(10);
  const [page, setPage] = useState(1);
  const [detailTarget, setDetailTarget] = useState(null);
  const [deletingKey, setDeletingKey] = useState(null);
  const [clearOpen, setClearOpen] = useState(false);
  const [clearing, setClearing] = useState(false);
  const [sources, setSources] = useState(null);
  const [exportPeriod, setExportPeriod] = useState('all');
  const [exportAnchor, setExportAnchor] = useState('');
  const [exportType, setExportType] = useState('ALL');
  const [exporting, setExporting] = useState(false);
  const load = useCallback(async () => {
    setLoading(true);
    setLoadError('');
    try {
      const overview = await getLicenseOverview();
      setRows(overview?.rows || []);
      setSummary(overview?.summary || null);
      setSources(overview?.sources || null);
    } catch (err) {
      setLoadError(err.message || 'โหลดข้อมูลไม่สำเร็จ');
    } finally {
      setLoading(false);
    }
  }, []);
  useEffect(() => {
    load();
  }, [load]);
  useEffect(() => {
    setPage(1);
  }, [typeFilter, statusFilter, search, pageSize, country]);

  // กรองฝั่ง client เพื่อให้สลับแท็บได้ทันทีโดยไม่ต้องยิง API ใหม่
  const filtered = useMemo(() => {
    // ตัวกรองทั้งสามชั้นทำงานร่วมกัน: ประเภทใบ + สถานะ + ประเทศ
    let list = rows.filter(r => matchTypeFilter(r, typeFilter) && matchStatusFilter(r, statusFilter));
    if (country === COUNTRY_MULTI) {
      // ใบที่ครอบหลายประเทศในใบเดียว ยังไม่ได้แตกสายตามประเทศ
      list = list.filter(r => isMultiCountry(r.country));
    } else if (country !== COUNTRY_ALL) {
      list = list.filter(r => countryKeys(r.country).includes(country));
    }
    const term = search.trim().toLowerCase();
    if (term) {
      list = list.filter(r => [r.currentLicenseNo, r.originalLicenseNo, r.country, r.model, ...(r.previousLicenseNos || [])].some(v => String(v || '').toLowerCase().includes(term)));
    }
    return list;
  }, [rows, typeFilter, statusFilter, search, country]);
  // ตัวเลือกประเทศ สร้างจากข้อมูลที่มีจริงในระบบ พร้อมจำนวนใบของแต่ละประเทศ
  // "ไม่ระบุประเทศ" ดันไว้ท้ายสุดเสมอ เพราะไม่ใช่ชื่อประเทศ
  const countryOptions = useMemo(() => {
    const tally = new Map();
    let multi = 0;
    for (const r of rows) {
      for (const key of countryKeys(r.country)) {
        tally.set(key, (tally.get(key) || 0) + 1);
      }
      if (isMultiCountry(r.country)) multi++;
    }
    const named = Array.from(tally.keys()).filter(k => k !== COUNTRY_NONE).sort((a, b) => a.localeCompare(b));
    const list = [{
      value: COUNTRY_ALL,
      label: `ทุกประเทศ (${rows.length})`
    }, ...named.map(k => ({
      value: k,
      label: `${countryLabel(k)} (${tally.get(k)})`
    }))];
    // วางต่อจากรายประเทศ ก่อน "ไม่ระบุประเทศ" ที่ต้องอยู่ท้ายสุดเสมอ
    if (multi > 0) {
      list.push({
        value: COUNTRY_MULTI,
        label: `${COUNTRY_MULTI_LABEL} (${multi})`
      });
    }
    if (tally.has(COUNTRY_NONE)) {
      list.push({
        value: COUNTRY_NONE,
        label: `${countryLabel(COUNTRY_NONE)} (${tally.get(COUNTRY_NONE)})`
      });
    }
    return list;
  }, [rows]);

  // ประเทศที่เลือกไว้อาจหายไปหลังลบข้อมูล — ถอยกลับเป็นทุกประเทศ ไม่งั้นตารางจะว่างโดยไม่รู้สาเหตุ
  useEffect(() => {
    if (country !== COUNTRY_ALL && !countryOptions.some(o => o.value === country)) {
      setCountry(COUNTRY_ALL);
    }
  }, [countryOptions, country]);

  // จำนวนในแต่ละตัวเลือก คิดจากตัวกรองอีกชั้นที่เลือกไว้แล้ว
  //
  // เลือก "ใบนำเข้า" ไว้ ตัวเลือกสถานะจะบอกจำนวนเฉพาะใบนำเข้า
  // เลือก "ใกล้หมดอายุ" ไว้ ตัวเลือกประเภทก็จะบอกจำนวนเฉพาะใบที่ใกล้หมดอายุ
  // ตัวเลขจึงตรงกับสิ่งที่จะได้เห็นจริงหลังกด ไม่ใช่ยอดรวมทั้งระบบ
  const typeOptions = useMemo(() => TYPE_FILTER_OPTIONS.map(o => ({
    value: o.value,
    label: `${o.label} (${rows.filter(r => matchTypeFilter(r, o.value) && matchStatusFilter(r, statusFilter)).length})`
  })), [rows, statusFilter]);

  const statusOptions = useMemo(() => STATUS_FILTER_OPTIONS.map(o => ({
    value: o.value,
    label: `${o.label} (${rows.filter(r => matchTypeFilter(r, typeFilter) && matchStatusFilter(r, o.value)).length})`
  })), [rows, typeFilter]);

  const totalPages = Math.max(1, Math.ceil(filtered.length / pageSize));
  const paged = filtered.slice((page - 1) * pageSize, page * pageSize);
  function goToPage(p) {
    setPage(Math.min(Math.max(1, p), totalPages));
  }

  // ลบใบอนุญาต 1 รายการ — ลบทั้งโซ่ (รายการเครื่อง + ประวัติต่ออายุ + ทะเบียน)
  // ถ้าลบแค่บางส่วน แถวเดิมจะโผล่กลับมาใหม่ เพราะภาพรวมสร้างจากหลายตารางรวมกัน
  async function handleDelete(row) {
    const key = `${row.licenseType}-${row.currentLicenseNo}`;
    if (deletingKey) return;
    const ok = await confirmDelete({
      title: `ลบใบอนุญาต ${row.currentLicenseNo}?`,
      confirmText: 'ลบใบอนุญาต'
    });
    if (!ok) return;
    setDeletingKey(key);
    try {
      const res = await deleteLicenseOverviewEntry({
        type: row.licenseType,
        licenseNo: row.currentLicenseNo
      });
      toastSuccess(res?.message || 'ลบแล้ว');
      // แถวที่สแกนผ่านไปแล้วจะลบไม่ได้ — ต้องบอกผู้ใช้ ไม่ใช่เงียบ ๆ
      if (res?.skipped?.length > 0) {
        toastError(`มี ${res.skipped.length} รายการที่สแกนผ่านแล้ว ลบไม่ได้ — ยังอยู่ในระบบ`);
      }
      await load();
    } catch (err) {
      toastError(err.message || 'ลบไม่สำเร็จ');
    } finally {
      setDeletingKey(null);
    }
  }

  // ขอบเขตวันที่ของข้อมูลจริง — ใช้จำกัดช่วงที่ปฏิทินเลือกได้
  const issueDateBounds = useMemo(() => {
    let min = '';
    let max = '';
    for (const r of rows) {
      if (!r.issueDate) continue;
      const d = String(r.issueDate).slice(0, 10);
      if (!min || d < min) min = d;
      if (!max || d > max) max = d;
    }
    return {
      min,
      max
    };
  }, [rows]);

  // รายการที่จะถูกส่งออกตามเงื่อนไขปัจจุบัน — ใช้โชว์จำนวนก่อนกดจริง
  const exportRows = useMemo(() => {
    let list = rows;
    if (exportType !== 'ALL') list = list.filter(r => r.licenseType === exportType);
    if (exportPeriod !== 'all') {
      list = list.filter(r => r.issueDate && inPeriod(r.issueDate, exportPeriod, exportAnchor));
    }
    return list;
  }, [rows, exportType, exportPeriod, exportAnchor]);
  const exportPreviewCount = exportRows.length;

  // ---------------------------------------------------------------------------
  // Export Excel — 1 ประเทศ = 1 ชีต เหมือนไฟล์เดิมที่ทีมใช้อยู่
  // กรองตามวันที่ออกใบอนุญาต เพราะเป็นวันที่ที่ใช้อ้างอิงกันจริงในงานเอกสาร
  // ---------------------------------------------------------------------------
  function handleExportXlsx() {
    if (exporting) return;

    // ใบที่ไม่มีวันที่ออกใบอนุญาตจะไม่เข้าช่วงใดเลย — ตรงกับพฤติกรรมของ Export เดิม
    const list = exportRows;
    if (list.length === 0) {
      toastError('ไม่มีข้อมูลตามเงื่อนไขที่เลือก');
      return;
    }

    setExporting(true);
    try {
      // จัดกลุ่มตามประเทศ หนึ่งแถวลงหนึ่งชีตเท่านั้น
      //
      //   ประเทศเดียว -> ชีตของประเทศนั้น
      //   หลายประเทศ  -> ชีต "หลายประเทศ" (ใบที่ยังไม่ได้แตกสายตามประเทศ)
      //   ไม่ได้กรอก   -> ชีต "ไม่ระบุประเทศ"
      //
      // ใบที่ครอบหลายประเทศไปรวมไว้ชีตเดียว ไม่ใส่ซ้ำลงทุกชีตของประเทศที่ครอบคลุม
      // ไม่งั้นยอดรวมข้ามชีตจะนับใบเดียวกันหลายครั้ง
      const groups = new Map();
      for (const r of list) {
        const keys = countryKeys(r.country);
        const key = keys.length > 1 ? COUNTRY_MULTI : keys[0];
        if (!groups.has(key)) groups.set(key, []);
        groups.get(key).push(r);
      }
      // เรียงชีต: ประเทศเดี่ยวตามตัวอักษร แล้วหลายประเทศ และไม่ระบุประเทศท้ายสุด
      const names = Array.from(groups.keys()).filter(k => k !== COUNTRY_NONE && k !== COUNTRY_MULTI).sort((a, b) => a.localeCompare(b));
      if (groups.has(COUNTRY_MULTI)) names.push(COUNTRY_MULTI);
      if (groups.has(COUNTRY_NONE)) names.push(COUNTRY_NONE);

      const columns = [{
        key: 'item',
        header: 'Item',
        type: 'center',
        width: 8
      }, {
        key: 'licenseType',
        header: 'Type',
        type: 'center',
        width: 12
      }, {
        key: 'licenseNo',
        header: 'Current License No.',
        type: 'text',
        width: 22
      }, {
        key: 'originalNo',
        header: 'Original License No.',
        type: 'text',
        width: 22
      }, {
        key: 'country',
        header: 'Country',
        type: 'center',
        width: 16
      }, {
        key: 'model',
        header: 'Model',
        type: 'text',
        width: 16
      }, {
        key: 'issueDate',
        header: 'Issue Date',
        type: 'center',
        width: 18
      }, {
        key: 'expiryDate',
        header: 'Expiry Date',
        type: 'center',
        width: 18
      }, {
        key: 'leadDate',
        header: 'Renewal Submission Due',
        type: 'center',
        width: 22
      }, {
        key: 'daysLeft',
        header: 'Status / Days',
        type: 'center',
        width: 22
      }, {
        key: 'quota',
        header: 'Total',
        type: 'center',
        width: 14
      }, {
        key: 'stock',
        header: 'Stock Export License',
        type: 'center',
        width: 20
      }, {
        key: 'remain',
        header: 'Remain',
        type: 'center',
        width: 14
      }, {
        key: 'renewalCount',
        header: 'Renewal Count',
        type: 'center',
        width: 18
      }, {
        key: 'lastRenewal',
        header: 'Last Renewal',
        type: 'center',
        width: 16
      }];

      const sheets = names.map(key => {
        const items = [...groups.get(key)].sort((a, b) => {
          const ta = a.issueDate ? new Date(a.issueDate).getTime() : Infinity;
          const tb = b.issueDate ? new Date(b.issueDate).getTime() : Infinity;
          return ta - tb;
        });
        return {
          sheetName: key === COUNTRY_MULTI ? COUNTRY_MULTI_LABEL : countryLabel(key),
          columns,
          rows: items.map((r, i) => ({
            // ใบที่หมดอายุแล้วและยังไม่ปิดงาน ให้เน้นสีในไฟล์ จะได้เห็นทันที
            __danger: r.status === 'EXPIRED',
            item: i + 1,
            licenseType: r.licenseType === 'IMPORT' ? 'Import' : 'Export',
            licenseNo: r.currentLicenseNo || DASH,
            originalNo: r.originalLicenseNo || DASH,
            // ชีตประเทศเดี่ยวใช้ชื่อประเทศของชีตนั้น ส่วนชีต "หลายประเทศ"
            // ต้องกางชื่อประเทศทั้งหมดของใบให้เห็น ไม่งั้นไม่รู้ว่าแถวนี้ครอบคลุมที่ไหนบ้าง
            country: key === COUNTRY_MULTI ? countryDisplay(r.country) : countryLabel(key),
            model: r.model || DASH,
            issueDate: r.issueDate ? formatThaiDate(r.issueDate) : DASH,
            expiryDate: r.expiryDate ? formatThaiDate(r.expiryDate) : DASH,
            leadDate: r.leadDate ? formatThaiDate(r.leadDate) : DASH,
            daysLeft: licenseDaysLabel(r.status, r.daysLeft),
            quota: r.quota ?? 0,
            stock: r.stock ?? 0,
            remain: r.remain ?? 0,
            renewalCount: r.renewalCount ?? 0,
            lastRenewal: r.lastRenewalDate ? formatThaiDate(r.lastRenewalDate) : DASH
          }))
        };
      });

      const blob = buildStyledXlsxWorkbookBlob({
        sheets
      });
      const tag = periodFileTag(exportPeriod, exportAnchor);
      downloadBlob(blob, `License-by-country-${tag}.xlsx`);
      const scope = exportPeriod === 'all' ? '' : ` — ช่วง ${periodRangeLabel(exportPeriod, exportAnchor)}`;
      toastSuccess(`Export สำเร็จ — ${names.length} ประเทศ (${list.length} ใบ)${scope}`);
    } catch (err) {
      toastError(err.message || 'Export ไม่สำเร็จ');
    } finally {
      setExporting(false);
    }
  }

  // ล้างข้อมูลทั้งหมดตามขอบเขตที่เลือก — ยืนยันอีกชั้นก่อนยิงจริง
  async function handleClear(option) {
    if (!option || clearing) return;
    const ok = await confirmDelete({
      title: option.label + '?',
      confirmText: 'ลบข้อมูล'
    });
    if (!ok) return;
    setClearing(true);
    try {
      const res = await clearLicenseOverview(option.value);
      toastSuccess(res?.message || 'ล้างข้อมูลแล้ว');
      setClearOpen(false);
      await load();
      // ลบไปแล้วแต่ตารางยังมีแถวเหลือ = ใบเหล่านั้นมาจากไฟล์อื่นที่ยังไม่ได้ล้าง
      // ต้องบอกตรง ๆ ไม่งั้นจะดูเหมือนปุ่มลบไม่ทำงาน
      if (res?.remaining > 0 && option.value !== CLEAR_SCOPE.ALL) {
        toastError(`ยังเหลือ ${res.remaining} ใบในตาราง เพราะมาจากไฟล์อื่นที่ยังไม่ได้ลบ — เลือก "ลบข้อมูลทั้งหมด" ถ้าต้องการให้ตารางว่างเปล่า`);
      }
    } catch (err) {
      toastError(err.message || 'ล้างข้อมูลไม่สำเร็จ');
    } finally {
      setClearing(false);
    }
  }

  return <AppShell navItems={WH_NAV_ITEMS} roleLabel="Warehouse">
      <div className="wh-heading-row">
        <div>
          <h2 className="wh-title">License</h2>
        </div>
        <div className="lo-head-actions">
          <button className="wh-modal-cancel" onClick={load} disabled={loading}>
            <ArrowPathIcon className="size-4" /> รีเฟรช
          </button>
          <button className="wh-modal-cancel lo-clear-btn" onClick={() => setClearOpen(true)} disabled={loading || clearing}>
            <TrashIcon className="size-4" /> ล้างข้อมูล
          </button>
        </div>
      </div>

      {loadError && <p className="form-error" role="alert">
          {loadError}
        </p>}

      <UploadPanel onUploaded={load} />

      {/* Export Excel — 1 ประเทศ = 1 ชีต เหมือนไฟล์เดิมที่ทีมใช้อยู่ */}
      <div className="wh-upload-card lo-export-card">
        <span className="lo-export-title">
          <ArrowDownTrayIcon className="size-4" /> Export Excel
        </span>

        <PeriodRangePicker mode={exportPeriod} onModeChange={m => {
        setExportPeriod(m);
        // เปลี่ยนโหมดแล้วยังไม่ได้เลือกวัน — ตั้งวันอ้างอิงเป็นวันล่าสุดที่มีข้อมูล
        // จะได้เห็นช่วงที่มีของจริงทันที ไม่ต้องเลื่อนปฏิทินหาเอง
        if (m !== 'all' && !exportAnchor) setExportAnchor(issueDateBounds.max || '');
      }} anchor={exportAnchor} onAnchorChange={setExportAnchor} min={issueDateBounds.min} max={issueDateBounds.max} label="ช่วงวันที่ออกใบอนุญาต" countLabel={`${exportPreviewCount} ใบ`} onClear={() => {
        setExportPeriod('all');
        setExportAnchor('');
      }} />

        <div className="lo-export-fields">
          <div className="lo-country-filter">
            <span className="lo-country-filter-label">ประเภท</span>
            <div className="lo-country-filter-select">
              <SelectField value={exportType} onChange={setExportType} options={EXPORT_TYPE_OPTIONS} />
            </div>
          </div>
          <button className="wh-issue-btn" onClick={handleExportXlsx} disabled={exporting || loading || exportPreviewCount === 0}>
            <ArrowDownTrayIcon className="size-4" />
            {exporting ? 'กำลังสร้างไฟล์...' : `Export Excel (${exportPreviewCount} ใบ)`}
          </button>
        </div>
      </div>

      {summary && <div className="dash-stats-row wh-stats-row il-stats-row-5 lo-stats-row">
          {[{
        label: 'ใบอนุญาตทั้งหมด',
        value: summary.total,
        color: 'dash-icon-blue',
        icon: <Squares2X2Icon className="size-4" />
      }, {
        label: 'Import',
        value: summary.import,
        color: 'dash-icon-green',
        icon: <DocumentTextIcon className="size-4" />
      }, {
        label: 'Export',
        value: summary.export,
        color: 'dash-icon-orange',
        icon: <ReceiptPercentIcon className="size-4" />
      }, {
        label: 'ใกล้หมดอายุ',
        value: summary.nearExpiry,
        color: 'dash-icon-yellow',
        icon: <ClockIcon className="size-4" />
      }, {
        label: 'หมดอายุแล้ว',
        value: summary.expired,
        color: 'dash-icon-red',
        icon: <XCircleIcon className="size-4" />
      }].map(s => <div className="dash-stat-card" key={s.label}>
              <div className="dash-stat-label">
                <span>{s.label}</span>
                <span className={`dash-stat-icon ${s.color}`}>
                  {s.icon}
                </span>
              </div>
              <div className="dash-stat-value">{s.value ?? 0}</div>
            </div>)}
        </div>}

      <div className="tsf-history-toolbar">
        <div className="wh-history-filters">
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
            รายการต่อหน้า
          </div>
          <div className="lo-country-filter">
            <span className="lo-country-filter-label">ประเภท</span>
            <div className="lo-country-filter-select">
              <SelectField value={typeFilter} onChange={setTypeFilter} options={typeOptions} />
            </div>
          </div>
          <div className="lo-country-filter">
            <span className="lo-country-filter-label">สถานะ</span>
            <div className="lo-country-filter-select">
              <SelectField value={statusFilter} onChange={setStatusFilter} options={statusOptions} />
            </div>
          </div>
          <div className="lo-country-filter">
            <span className="lo-country-filter-label">ประเทศ</span>
            <div className="lo-country-filter-select">
              <SelectField value={country} onChange={setCountry} options={countryOptions} searchable={countryOptions.length > 8} searchPlaceholder="พิมพ์ชื่อประเทศ..." />
            </div>
          </div>
        </div>
        <input className="wh-search" type="text" placeholder="ค้นหาเลขใบอนุญาต / ประเทศ / รุ่น" value={search} onChange={e => setSearch(e.target.value)} />
      </div>

      <div className="wh-table-card">
        <table className="wh-table">
          <thead>
            <tr>
              <th>ลำดับ</th>
              <th>ประเภท</th>
              <th>เลขใบอนุญาตปัจจุบัน</th>
              <th>ประเทศ</th>
              <th>จำนวนเครื่อง</th>
              <th>วันที่ออกใบอนุญาต</th>
              <th>วันหมดอายุ</th>
              <th>ครบกำหนดยื่นต่ออายุ</th>
              <th>สถานะ / จำนวนวัน</th>
              <th>ต่ออายุ</th>
              <th>ต่ออายุล่าสุด</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {loading && <tr>
                <td colSpan={12} className="wh-empty-cell">
                  กำลังโหลดข้อมูล...
                </td>
              </tr>}
            {!loading && paged.map((r, i) => {
              const rowKey = `${r.licenseType}-${r.currentLicenseNo}`;
              const busy = deletingKey === rowKey;
              return <tr key={rowKey}>
                  <td className="wh-cell-head" data-label="ลำดับ">
                    {(page - 1) * pageSize + i + 1}
                  </td>
                  <td data-label="ประเภท">{LICENSE_TYPE_LABEL[r.licenseType] || r.licenseType}</td>
                  <td className="il-mono" data-label="เลขใบอนุญาตปัจจุบัน">
                    <strong>{r.currentLicenseNo}</strong>
                    {r.groupNo && <span className={'lo-group-chip' + (r.status === 'COMPLETED' ? ' lo-group-chip-done' : '')} title="คอลัมน์ NO. ในชีตต่ออายุ">{groupNoLabel(r.groupNo)}</span>}
                    {r.renewalCount > 0 && <span className="lo-from">จาก {r.originalLicenseNo}</span>}
                  </td>
                  <td data-label="ประเทศ">{r.country ? countryDisplay(r.country) : DASH}</td>
                  <td className="lo-cell-num" data-label="จำนวนเครื่อง">
                    {r.items > 0 ? <>
                        <strong>{r.items}</strong> เครื่อง
                        {r.completedItems > 0 && <span className="lo-done-count">เสร็จสิ้น {r.completedItems}/{r.items}</span>}
                      </> :
                  /* ใบที่รู้จักจากชีตต่ออายุอย่างเดียว ยังไม่มีไฟล์รายการเครื่อง
                     ใช้ TOTAL (โควต้า) เป็นจำนวนเครื่องแทน พร้อมบอกที่มาไว้ใต้ตัวเลข */
                  r.quota > 0 ? <>
                        <strong>{r.quota}</strong> เครื่อง
                      </> : DASH}
                  </td>
                  <td data-label="วันที่ออกใบอนุญาต">{formatThaiDate(r.issueDate)}</td>
                  <td data-label="วันหมดอายุ">{formatThaiDate(r.expiryDate)}</td>
                  <td data-label="ครบกำหนดยื่นต่ออายุ">
                    {r.leadDate ? <>
                        {formatThaiDate(r.leadDate)}
                        <span className={'lo-lead-note' + (r.leadDaysLeft < 0 ? ' lo-lead-over' : '')}>
                          {r.leadDaysLeft < 0 ? `เลยกำหนดยื่น ${Math.abs(r.leadDaysLeft)} วัน` : r.leadDaysLeft === 0 ? 'ครบกำหนดยื่นวันนี้' : `เหลือ ${r.leadDaysLeft} วัน`}
                        </span>
                      </> : DASH}
                  </td>
                  <td data-label="สถานะ / จำนวนวัน">
                    {statusBadge(r.status, r.daysLeft)}
                    {licenseDaysHint(r.status, r.daysLeft) && <span className="lo-days">{licenseDaysHint(r.status, r.daysLeft)}</span>}
                    {/* ใบที่ระบบปิดให้เองเพราะหมดอายุแล้วไม่มีใครต่อ — ต้องบอกเหตุผลไว้
                        ไม่งั้นผู้ใช้จะงงว่าทำไมใบขึ้นว่าเสร็จสิ้นทั้งที่ไม่เคยกดปิด */}
                    {r.autoClosed && <span className="lo-days" title={r.autoCloseReason || 'ระบบปิดให้อัตโนมัติ เนื่องจากหมดอายุแล้วไม่ได้ต่ออายุ'}>
                        ปิดอัตโนมัติ (ไม่ได้ต่ออายุ)
                      </span>}
                    {/* ใบที่หมดอายุแล้วแต่ยังไม่ถึงคิวปิด ต้องบอกไว้ว่าเหลืออีกกี่วัน
                        ไม่งั้นผู้ใช้จะถามว่า "หมดอายุไปแล้ว ทำไมยังไม่เสร็จสิ้น" */}
                    {!r.autoClosed && r.autoCloseInDays > 0 && <span className="lo-days" title="ครบกำหนดแล้วระบบจะปิดใบนี้เป็นเสร็จสิ้น และหยุดแจ้งเตือน — ถ้าต่ออายุภายหลัง เพียงนำเข้าข้อมูลใบใหม่ รายการจะกลับมาแจ้งเตือนอีกครั้ง">
                        {autoCloseHint(r.autoCloseInDays)}
                      </span>}
                  </td>
                  <td data-label="ต่ออายุ">{r.renewalCount ?? 0} ครั้ง</td>
                  <td data-label="ต่ออายุล่าสุด">
                    {r.lastRenewalDate ? formatThaiDate(r.lastRenewalDate) : DASH}
                  </td>
                  <td className="wh-cell-action">
                    <div className="lo-row-actions">
                      <button className="tsf-action-btn" onClick={() => setDetailTarget(r)} disabled={busy}>
                        รายละเอียด
                      </button>
                      <button className="tsf-action-btn lo-delete-btn" onClick={() => handleDelete(r)} disabled={busy} title={`ลบใบอนุญาต ${r.currentLicenseNo}`}>
                        <TrashIcon className="size-4" />
                        <span>{busy ? 'กำลังลบ...' : 'ลบ'}</span>
                      </button>
                    </div>
                  </td>
                </tr>;
            })}
            {!loading && filtered.length === 0 && <tr>
                <td colSpan={12} className="wh-empty-cell">
                  {rows.length === 0 ? 'ยังไม่มีข้อมูลใบอนุญาต — อัปโหลดไฟล์ด้านบนก่อน' : 'ไม่พบรายการตามตัวกรอง'}
                </td>
              </tr>}
          </tbody>
        </table>
      </div>

      {!loading && filtered.length > 0 && <div className="tsf-pagination">
          <span className="wh-subtitle" style={{
        fontSize: 13
      }}>
            แสดง {(page - 1) * pageSize + 1}–{Math.min(page * pageSize, filtered.length)} จาก{' '}
            {filtered.length} รายการ
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

      <LicenseDetailModal target={detailTarget} onClose={() => setDetailTarget(null)} onChanged={load} />

      <ClearDataModal open={clearOpen} busy={clearing} onClose={() => !clearing && setClearOpen(false)} onClear={handleClear} />
    </AppShell>;
}