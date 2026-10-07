import { useEffect, useMemo, useRef, useState } from 'react';
import AppShell from '../components/AppShell.jsx';
import SelectField from '../components/Selectfield.jsx';
import { getLicenseRenewals, uploadLicenseWorkbook } from '../api/licenseRenewal.js';
import { toastError, toastSuccess } from '../lib/toast.js';
import { CheckCircleIcon, ClockIcon, ExclamationTriangleIcon, XCircleIcon, MinusIcon, CloudArrowUpIcon, ChevronDoubleLeftIcon, ChevronDoubleRightIcon, ChevronLeftIcon, ChevronRightIcon, ChevronDownIcon } from '../components/icons.jsx';
import { WH_NAV_ITEMS } from './Importlicensepage.jsx';

// ---------------------------------------------------------------------------
// ทะเบียนใบอนุญาต — ผู้ใช้ยังทำงานใน Excel ตามเดิม
// อัปโหลดไฟล์เดียว ระบบอ่านทั้ง 3 ชีต (Import / Export / ต่ออายุ)
// แล้วตอบแค่ 3 คำถาม: หมดอายุยัง · ใกล้หมดยัง · ต่ออายุไปกี่ครั้งแล้ว
// ---------------------------------------------------------------------------

const DASH = '—';
const STATUS_META = {
  VALID: {
    label: 'ยังใช้ได้',
    cls: 'il-badge il-badge-ok',
    Icon: CheckCircleIcon
  },
  EXPIRING: {
    label: 'ใกล้หมดอายุ',
    cls: 'il-badge il-badge-warn',
    Icon: ClockIcon
  },
  EXPIRED: {
    label: 'หมดอายุแล้ว',
    cls: 'il-badge il-badge-bad',
    Icon: XCircleIcon
  },
  USED_UP: {
    label: 'โควต้าหมด',
    cls: 'il-badge il-badge-warn',
    Icon: ExclamationTriangleIcon
  },
  OVER_USED: {
    label: 'ใช้เกินโควต้า',
    cls: 'il-badge il-badge-bad',
    Icon: ExclamationTriangleIcon
  },
  NO_LICENSE: {
    label: 'ยังไม่ออกใบนำออก',
    cls: 'il-badge il-badge-muted',
    Icon: MinusIcon
  },
  NO_DATE: {
    label: 'ไม่มีวันหมดอายุ',
    cls: 'il-badge il-badge-muted',
    Icon: MinusIcon
  }
};
const RENEW_STEP_LABEL = {
  NONE: 'ยังไม่ได้ยื่นเรื่อง',
  EMAIL: 'ส่งอีเมลแล้ว · รอชำระเงิน',
  PAYMENT: 'ชำระเงินแล้ว · รอเอกสาร',
  RECEIVED: 'ได้รับเอกสารแล้ว'
};
function statusBadge(status) {
  const m = STATUS_META[status] || STATUS_META.NO_DATE;
  return <span className={m.cls}>
      <m.Icon className="inline size-3.5 align-text-bottom" /> {m.label}
    </span>;
}
function fmtDate(value) {
  if (!value) return DASH;
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return DASH;
  return `${d.getDate()}/${d.getMonth() + 1}/${d.getFullYear() + 543}`;
}
function daysLabel(days) {
  if (days === null || days === undefined) return DASH;
  if (days < 0) return `เลยมา ${Math.abs(days)} วัน`;
  if (days === 0) return 'หมดวันนี้';
  return `เหลือ ${days} วัน`;
}
export default function LicenseLedgerPage() {
  const [groups, setGroups] = useState([]);
  const [rows, setRows] = useState([]);
  const [summary, setSummary] = useState(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [uploading, setUploading] = useState(false);
  const [uploadResult, setUploadResult] = useState(null);
  const [search, setSearch] = useState('');
  const [statusFilter, setStatusFilter] = useState('active');
  const [expanded, setExpanded] = useState(null);
  const [pageSize, setPageSize] = useState(10);
  const [page, setPage] = useState(1);
  const fileRef = useRef(null);
  async function load() {
    setLoading(true);
    setLoadError('');
    try {
      const res = await getLicenseRenewals();
      setGroups(res?.licenses || []);
      setRows(res?.rows || []);
      setSummary(res?.summary || null);
    } catch (err) {
      setLoadError(err.message || 'โหลดข้อมูลไม่สำเร็จ');
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => {
    load();
  }, []);
  useEffect(() => {
    setPage(1);
  }, [search, statusFilter, pageSize]);
  async function handleFile(e) {
    const file = e.target.files?.[0];
    e.target.value = '';
    if (!file) return;
    setUploading(true);
    setUploadResult(null);
    try {
      const results = await uploadLicenseWorkbook(file);
      setUploadResult(results);
      const ran = results.filter(r => !r.skipped);
      const okCount = ran.filter(r => r.ok).length;
      if (ran.length === 0) {
        toastError('ไฟล์นี้ไม่มีชีตที่ระบบอ่านได้ (ใบอนุญาตนำเข้า · นำออก · ทะเบียนต่ออายุ)');
      } else if (okCount === 0) {
        toastError('พบชีตในไฟล์ แต่อ่านข้อมูลเข้าระบบไม่ได้เลย');
      } else {
        toastSuccess(`อัปเดตแล้ว ${okCount} จาก ${ran.length} ชีตที่พบในไฟล์`);
        await load();
      }
    } catch (err) {
      toastError(err.message || 'อัปโหลดไม่สำเร็จ');
    } finally {
      setUploading(false);
    }
  }
  const filtered = useMemo(() => {
    const term = search.trim().toUpperCase();
    return groups.filter(g => {
      if (statusFilter === 'active' && (g.status === 'EXPIRED' || g.status === 'NO_DATE')) return false;
      if (statusFilter === 'attention' && !['EXPIRING', 'USED_UP', 'OVER_USED', 'NO_LICENSE'].includes(g.status)) return false;
      if (statusFilter === 'expired' && g.status !== 'EXPIRED') return false;
      if (term && ![g.importLicenseNo, g.latestExportNo, g.itControllerModel, g.country].some(v => String(v || '').toUpperCase().includes(term))) return false;
      return true;
    });
  }, [groups, search, statusFilter]);
  const totalPages = Math.max(1, Math.ceil(filtered.length / pageSize));
  const paged = filtered.slice((page - 1) * pageSize, page * pageSize);
  function goToPage(p) {
    setPage(Math.min(Math.max(1, p), totalPages));
  }
  function roundsOf(importNo) {
    return rows.filter(r => r.ImportLicenseNo === importNo && r.ExportLicenseNo);
  }
  return <AppShell navItems={WH_NAV_ITEMS} roleLabel="Warehouse">
      <div className="wh-heading-row">
        <div>
          <h2 className="wh-title">ทะเบียนใบอนุญาต</h2>
          <span className="wh-subtitle" style={{
          fontSize: 13
        }}>
            อัปโหลดไฟล์ Excel ระบบอ่านชีตใบอนุญาตนำเข้าและชีตต่ออายุให้เอง ชีตไหนไม่มีก็ข้ามไป
          </span>
        </div>
        <div>
          <button className="wh-issue-btn" disabled={uploading} onClick={() => fileRef.current?.click()}>
            <CloudArrowUpIcon className="inline size-4 align-text-bottom" />{' '}
            {uploading ? 'กำลังอ่านไฟล์...' : 'อัปโหลดไฟล์'}
          </button>
          <input ref={fileRef} type="file" accept=".xlsx,.xlsm,.xls,.xlsb,.csv" style={{
          display: 'none'
        }} onChange={handleFile} />
        </div>
      </div>

      {uploadResult && <div className="wh-table-card" style={{
      padding: 14,
      marginBottom: 16
    }}>
          {/* ไฟล์งานจริงมักแยกเล่ม เล่มละชีต ส่วนที่ไม่มีในไฟล์ไม่ใช่ความผิดพลาด
              จึงแสดงเป็นขีดจาง ๆ ไม่ใช่กากบาทแดง */}
          {uploadResult.map(r => <div key={r.key}>
              <div className={'ll-upload-line' + (r.skipped ? ' ll-upload-skipped' : '')}>
                {r.skipped ? <MinusIcon className="size-4 ll-skip" /> : r.ok ? <CheckCircleIcon className="size-4 ll-ok" /> : <XCircleIcon className="size-4 ll-bad" />}
                <span className="ll-upload-label">{r.label}</span>
                <span className="ll-upload-msg">
                  {r.skipped ? 'ไม่มีชีตนี้ในไฟล์' : r.ok ? r.count !== null ? `${r.count} แถว` : 'สำเร็จ' : r.message}
                </span>
              </div>
              {/* คอลัมน์ที่ระบบไม่รู้จัก — ไม่ใช่ข้อผิดพลาด ข้อมูลถูกเก็บไว้ครบ
                  แต่ต้องบอก ไม่งั้นคนที่เพิ่งเพิ่มคอลัมน์เข้าไปจะไม่รู้ว่าระบบเห็นหรือเปล่า */}
              {r.extraNotice && <div style={{
            margin: '4px 0 8px 24px',
            padding: '7px 10px',
            borderRadius: 6,
            background: '#eff6ff',
            border: '1px solid #bfdbfe',
            color: '#1e40af',
            fontSize: 12.5
          }}>
                  {r.extraNotice}
                </div>}
            </div>)}
        </div>}

      {loadError && <p className="form-error" role="alert">
          {loadError}
        </p>}

      {summary && <div className="il-stat-grid">
          <StatCard label="ใบอนุญาตนำเข้า" value={summary.importLicenses} hint={`${summary.rows} แถวในทะเบียน`} />
          <StatCard label="ใกล้หมดอายุ" value={summary.expiring} tone="warn" hint={`ภายใน 14 วัน`} />
          <StatCard label="โควต้าหมด / ใช้เกิน" value={summary.usedUp + summary.overUsed} tone="warn" hint="ยังไม่หมดอายุแต่ใช้ไม่ได้แล้ว" />
          <StatCard label="หมดอายุแล้ว" value={summary.expired} tone="bad" />
          <StatCard label="ยังไม่ออกใบนำออก" value={summary.noLicense} hint="มีโควต้าแต่ยังไม่ได้ใบ" />
          <StatCard label="อยู่ระหว่างต่ออายุ" value={summary.waitingRenew} hint="ยื่นเรื่องแล้วยังไม่ได้เอกสาร" />
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
            entries per page
          </div>
          <div className="wh-filter-field">
            <span className="wh-filter-label">แสดง</span>
            <SelectField value={statusFilter} onChange={setStatusFilter} options={[{
            value: 'active',
            label: 'ที่ยังใช้งานอยู่'
          }, {
            value: 'attention',
            label: 'ที่ต้องรีบดู'
          }, {
            value: 'expired',
            label: 'หมดอายุแล้ว'
          }, {
            value: 'all',
            label: 'ทั้งหมด'
          }]} />
          </div>
        </div>
        <input className="wh-search" type="text" placeholder="ค้นหา เลขใบนำเข้า / ใบนำออก / รุ่น IT / ประเทศ" value={search} onChange={e => setSearch(e.target.value)} />
      </div>

      <div className="wh-table-card">
        <table className="wh-table">
          <thead>
            <tr>
              <th>ลำดับ</th>
              <th>ใบอนุญาตนำเข้า</th>
              <th>รุ่น IT</th>
              <th>ประเทศ</th>
              <th>โควต้า</th>
              <th>ต่ออายุแล้ว</th>
              <th>ใบนำออกล่าสุด</th>
              <th>หมดอายุ</th>
              <th>คงเหลือ</th>
              <th>สถานะ</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {loading && <tr>
                <td colSpan={11} className="wh-empty-cell">
                  กำลังโหลดข้อมูล...
                </td>
              </tr>}
            {!loading && paged.map((g, i) => {
            const open = expanded === g.importLicenseNo;
            return <FragmentRow key={g.importLicenseNo} group={g} seq={(page - 1) * pageSize + i + 1} open={open} rounds={open ? roundsOf(g.importLicenseNo) : []} onToggle={() => setExpanded(open ? null : g.importLicenseNo)} />;
          })}
            {!loading && filtered.length === 0 && <tr>
                <td colSpan={11} className="wh-empty-cell">
                  {groups.length === 0 ? 'ยังไม่มีข้อมูล — กดปุ่มอัปโหลดไฟล์ด้านบน' : 'ไม่พบใบอนุญาตตามตัวกรอง'}
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
    </AppShell>;
}
function StatCard({
  label,
  value,
  hint,
  tone
}) {
  return <div className={'il-stat-card' + (tone ? ` il-stat-${tone}` : '')}>
      <span className="il-stat-label">{label}</span>
      <span className="il-stat-value">{value ?? 0}</span>
      {hint ? <span className="il-stat-hint">{hint}</span> : null}
    </div>;
}

// 1 ใบอนุญาตนำเข้า + แถวย่อยที่กางออกมาเป็นประวัติการต่ออายุ
function FragmentRow({
  group: g,
  seq,
  open,
  rounds,
  onToggle
}) {
  return <>
      <tr className={open ? 'il-row-hit' : ''}>
        <td className="wh-cell-head" data-label="ลำดับ">{seq}</td>
        <td className="il-mono" data-label="ใบอนุญาตนำเข้า">
          <strong>{g.importLicenseNo}</strong>
        </td>
        <td className="il-mono" data-label="รุ่น IT">
          {g.itControllerModel || DASH}
        </td>
        <td data-label="ประเทศ">{g.country || DASH}</td>
        <td data-label="โควต้า">{g.total || DASH}</td>
        <td data-label="ต่ออายุแล้ว">
          <strong>{g.renewalCount}</strong> ครั้ง
        </td>
        <td className="il-mono" data-label="ใบนำออกล่าสุด">
          {g.latestExportNo || DASH}
        </td>
        <td data-label="หมดอายุ">
          {fmtDate(g.latestExpire)}
          <span className="mfg-plan-hint">{daysLabel(g.daysLeft)}</span>
        </td>
        <td data-label="คงเหลือ">
          {g.latestExportNo ? g.remain : DASH}
          {/* ใบนำเข้าใบเดียวแบ่งโควต้าได้หลายประเทศ ยอดรวมอย่างเดียวใช้ทำงานไม่ได้ */}
          {g.branches?.length > 1 && <span className="mfg-plan-hint">
              {g.branches.map(b => `${b.country || '?'} ${b.remain}`).join(' · ')}
            </span>}
        </td>
        <td data-label="สถานะ">
          {statusBadge(g.status)}
          {g.renewStep && g.renewStep !== 'NONE' ? <span className="mfg-plan-hint">{RENEW_STEP_LABEL[g.renewStep]}</span> : null}
        </td>
        <td className="wh-cell-action">
          {g.renewalCount > 0 && <button className="tsf-action-btn" onClick={onToggle}>
              <ChevronDownIcon className={'inline size-4 align-text-bottom ll-caret' + (open ? ' ll-caret-open' : '')} />{' '}
              ประวัติ
            </button>}
        </td>
      </tr>

      {open && <tr>
          <td colSpan={11} className="ll-rounds-cell">
            <table className="wh-table ll-rounds-table">
              <thead>
                <tr>
                  <th>ประเทศ</th>
                  <th>ครั้งที่</th>
                  <th>เลขใบนำออก</th>
                  <th>วันที่ออก</th>
                  <th>วันหมดอายุ</th>
                  <th>จำนวนบนใบ</th>
                  <th>คงเหลือ</th>
                  <th>ขั้นตอนต่ออายุ</th>
                  <th>สถานะ</th>
                </tr>
              </thead>
              <tbody>
                {/* ครั้งที่นับแยกตามประเทศ สาย INDONESIA ไม่ได้ต่อจากสาย MALAYSIA
                     จึงต้องมีคอลัมน์ประเทศกำกับ ไม่งั้นจะเห็นครั้งที่ 1 ซ้ำสองแถว */}
                {rounds.map(r => <tr key={r.ID}>
                    <td data-label="ประเทศ">{r.Country || DASH}</td>
                    <td data-label="ครั้งที่">{r.RenewalRound}</td>
                    <td className="il-mono" data-label="เลขใบนำออก">
                      {r.ExportLicenseNo}
                    </td>
                    <td data-label="วันที่ออก">{fmtDate(r.IssueDate)}</td>
                    <td data-label="วันหมดอายุ">{fmtDate(r.ExpireDate)}</td>
                    <td data-label="จำนวนบนใบ">{r.Stock}</td>
                    <td data-label="คงเหลือ">{r.Remain}</td>
                    <td data-label="ขั้นตอนต่ออายุ">{RENEW_STEP_LABEL[r.RenewStep] || DASH}</td>
                    <td data-label="สถานะ">{statusBadge(r.Status)}</td>
                  </tr>)}
              </tbody>
            </table>
          </td>
        </tr>}
    </>;
}
