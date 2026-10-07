import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import SelectField from './Selectfield.jsx';
import PeriodRangePicker from './PeriodRangePicker.jsx';
import { getFlowMachines, getFlowMachine, getFlowIssues, issueFlowPart } from '../api/whFlow.js';
import { scanStep, scanLoading, scanClose, scanSuccessToast, scanErrorAlert } from '../lib/scanPopup.js';
import { inPeriod } from '../lib/dateRange.js';
import { toastError, toastSuccess } from '../lib/toast.js';
import { CheckCircleIcon, ChevronDoubleLeftIcon, ChevronDoubleRightIcon, ChevronLeftIcon, ChevronRightIcon, CubeIcon, DocumentTextIcon, XMarkIcon } from './icons.jsx';

// ---------------------------------------------------------------------------
// ขั้นตอน WH
//   1. กด option Machine (MC#) → เลือกเลขเครื่อง (เครื่องที่จ่ายครบแล้วมีไอคอนติ๊กถูกท้ายเลข)
//   2. เด้ง popup ให้สแกน Part# — ระบบดูเองว่ารายการนั้นต้องสแกนกี่ขั้น
//        CW / CV / SM / MP / Pump HYD → สแกน Part# ขั้นเดียว แล้วเทียบกับ master_data
//                                       ตาม Product Spec ของเครื่องนั้น
//        IT Controller / Engine        → สแกน P/N แล้วต่อด้วย S/N (เทียบไฟล์ Planning WH)
//   3. ระบบหาเองว่าเป็นของรายการไหน แล้วเทียบ
//        ตรง    → "บันทึกข้อมูลสำเร็จ" แล้วขึ้นในตารางประวัติด้านล่าง
//        ไม่ตรง → "ข้อมูลไม่ถูกต้อง" พร้อมบอกสาเหตุ ไม่บันทึก
//   ตารางประวัติจะว่างตอนเริ่มต้น และมีข้อมูลเมื่อจ่ายของสำเร็จเท่านั้น
// ---------------------------------------------------------------------------

const DASH = '—';
function escapeHtml(v) {
  return String(v ?? '').replace(/[&<>"']/g, ch => ({
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#39;'
  })[ch]);
}
function firstToken(v) {
  if (!v) return '';
  return String(v).trim().split(/\s+/)[0] || '';
}
function fmtDate(value) {
  if (!value) return DASH;
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return DASH;
  const pad = n => String(n).padStart(2, '0');
  return `${d.getDate()}/${d.getMonth() + 1}/${d.getFullYear() + 543} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}
function todayYMD() {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}
export default function WHIssuePanel() {
  const [machines, setMachines] = useState([]);
  const [issues, setIssues] = useState([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [selected, setSelected] = useState(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [search, setSearch] = useState('');
  // กรองตามรายการ (IT Controller / Engine / CW ...) — ว่าง = ทุกรายการ
  const [partFilter, setPartFilter] = useState('');
  const [periodMode, setPeriodMode] = useState('all');
  const [periodAnchor, setPeriodAnchor] = useState('');
  const [pageSize, setPageSize] = useState(10);
  const [page, setPage] = useState(1);
  const [detailRow, setDetailRow] = useState(null);
  const busyRef = useRef(false);
  function handlePeriodModeChange(next) {
    setPeriodMode(next);
    if (next !== 'all' && !periodAnchor) setPeriodAnchor(todayYMD());
  }
  function clearPeriod() {
    setPeriodMode('all');
    setPeriodAnchor('');
  }
  const loadAll = useCallback(async (silent = false) => {
    if (!silent) {
      setLoading(true);
      setLoadError('');
    }
    try {
      const [ms, is] = await Promise.all([getFlowMachines(), getFlowIssues()]);
      setMachines(ms?.rows || []);
      setIssues(is?.rows || []);
    } catch (err) {
      if (!silent) setLoadError(err.message || 'โหลดข้อมูลไม่สำเร็จ');
    } finally {
      if (!silent) setLoading(false);
    }
  }, []);
  useEffect(() => {
    loadAll();
  }, [loadAll]);
  useEffect(() => {
    setPage(1);
  }, [search, partFilter, pageSize, periodMode, periodAnchor]);
  async function openMachine(machineNo, autoScan = false) {
    setDetailLoading(true);
    try {
      const data = await getFlowMachine(machineNo);
      setSelected(data);
      if (autoScan) await runQuickScan(data);
      return data;
    } catch (err) {
      toastError(err.message || 'โหลดแผนของเครื่องนี้ไม่สำเร็จ');
      return null;
    } finally {
      setDetailLoading(false);
    }
  }
  async function afterIssue(nextMachine) {
    if (nextMachine) setSelected(nextMachine);
    await loadAll(true);
  }

  // ระบบดูเองว่ารายการที่สแกนมาต้องสแกนกี่ขั้น
  //   IT Controller / Engine (Planning WH) — มีทั้ง P/N และ S/N → สแกน 2 ขั้น
  //   CW / CV / SM / MP / Pump HYD (master_data) — มีแต่ Part# → สแกนขั้นเดียว
  function normCode(v) {
    return String(v || '').toUpperCase().replace(/[^A-Z0-9]/g, '');
  }
  function planItemOf(machine, code) {
    const key = normCode(code);
    if (!key) return null;
    return (machine?.items || []).find(it => it.expectedPartNo && normCode(it.expectedPartNo) === key) || null;
  }

  // เลือก MC# แล้วสแกน Part# — ระบบหาเองว่าเป็นของรายการไหน และต้องสแกน S/N ต่อไหม
  async function runQuickScan(machine) {
    if (!machine?.machineNo || busyRef.current) return;
    const planned = (machine.items || []).filter(it => it.status !== 'NO_PLAN');
    if (planned.length === 0) {
      await scanErrorAlert('เครื่องนี้ยังไม่มีรายการให้จ่าย', {
        hint: 'ตรวจว่าอัปโหลด master_data (CW_CV_ITS) และ Planning WH ของเครื่องนี้แล้วหรือยัง'
      });
      return;
    }
    busyRef.current = true;
    let okMsg = '';
    try {
      const mcHint = `<div class="scan-popup-hint">MC#: <b>${escapeHtml(machine.machineNo)}</b></div>`;
      const partNo = firstToken(await scanStep({
        title: 'สแกน Part#',
        html: mcHint,
        confirmText: 'ต่อไป'
      }));
      if (!partNo) return;

      const item = planItemOf(machine, partNo);
      // CW / CV / SM / MP / Pump HYD — master_data มีแต่ Part# จึงเก็บ S/N ว่างไว้
      // (MFG เป็นคนสแกน S/N# ตอนประกอบ) ไม่เอา Part# ไปใส่ซ้ำในช่อง S/N
      let serialNo = '';
      if (item?.expectedSerialNo) {
        // รายการนี้ไฟล์ Planning WH กำหนด S/N ไว้ด้วย → ต้องสแกนต่ออีกขั้น
        serialNo = firstToken(await scanStep({
          title: `${item.label} — สแกน S/N`,
          html: `${mcHint}<div class="scan-popup-hint">P/N: <b>${escapeHtml(partNo)}</b></div>`,
          confirmText: 'บันทึก',
          validate: v => firstToken(v) === partNo ? 'ค่า S/N ซ้ำกับ P/N' : undefined
        }));
        if (!serialNo) return;
      }

      scanLoading('กำลังตรวจสอบ...');
      const res = await issueFlowPart({
        machineNo: machine.machineNo,
        component: item?.component || '',
        partNo,
        serialNo
      });
      scanClose();
      if (res?.matched) {
        okMsg = res.message || 'บันทึกข้อมูลสำเร็จ';
        await afterIssue(res.machine);
      } else {
        // แสดงแค่ "ข้อมูลไม่ถูกต้อง" พอ — res.detail เป็นข้อมูลทางเทคนิคที่ยาวมาก
        // (Part#, Product Spec, S/N ของทุกชิ้น) คนหน้างานอ่านไม่ทันและไม่ได้ใช้ตัดสินใจ
        await scanErrorAlert(res?.message || 'ข้อมูลไม่ถูกต้อง');
      }
    } catch (err) {
      scanClose();
      await scanErrorAlert(err.message || 'ข้อมูลไม่ถูกต้อง');
    } finally {
      busyRef.current = false;
      scanClose();
    }
    if (okMsg) scanSuccessToast(okMsg);
  }

  // จ่ายครบทุกรายการแล้ว → ขึ้นไอคอนติ๊กถูกท้ายเลขเครื่องใน dropdown
  const machineOptions = useMemo(() => machines.map(m => ({
    value: m.machineNo,
    label: m.machineNo,
    suffix: m.total > 0 && m.issued >= m.total ? <CheckCircleIcon className="size-4 wh-mc-complete" /> : undefined
  })), [machines]);
  const filtered = useMemo(() => {
    let list = issues;
    if (periodMode !== 'all') {
      list = list.filter(r => r.issuedAt && inPeriod(r.issuedAt, periodMode, periodAnchor));
    }
    if (partFilter) {
      list = list.filter(r => (r.component || r.label) === partFilter);
    }
    const term = search.trim().toLowerCase();
    if (term) {
      list = list.filter(r => [r.machineNo, r.label, r.partNo, r.serialNo, r.lotNo, r.customer, r.specCode, r.issuedBy].some(v => String(v || '').toLowerCase().includes(term)));
    }
    return list;
  }, [issues, search, partFilter, periodMode, periodAnchor]);
  // รายการที่มีจริงในประวัติ — ไม่ฮาร์ดโค้ด เพิ่ม component ใหม่แล้วขึ้นเอง
  const partOptions = useMemo(() => {
    const seen = new Map();
    for (const r of issues) {
      const key = r.component || r.label;
      if (key && !seen.has(key)) seen.set(key, r.label || key);
    }
    return [{
      value: '',
      label: 'ทุกรายการ'
    }, ...[...seen].map(([value, label]) => ({
      value,
      label
    }))];
  }, [issues]);
  const totalPages = Math.max(1, Math.ceil(filtered.length / pageSize));
  const paged = filtered.slice((page - 1) * pageSize, page * pageSize);
  function goToPage(p) {
    setPage(Math.min(Math.max(1, p), totalPages));
  }
  return <>
      {loadError && <p className="form-error" role="alert">
          {loadError}
        </p>}

      <div className="upload-panel-field" style={{
      maxWidth: 440,
      margin: '4px 0 18px'
    }}>
        <label className="upload-panel-label" htmlFor="wh-machine-select">
          Machine (MC#)
        </label>
        <SelectField id="wh-machine-select" searchable searchPlaceholder="พิมพ์เลขเครื่องเพื่อค้นหา..." value={selected?.machineNo || ''} disabled={loading || detailLoading || machineOptions.length === 0} placeholder={machineOptions.length === 0 ? 'ยังไม่มีข้อมูลเครื่อง' : '— เลือก MC# —'} options={machineOptions} onChange={v => v && openMachine(v, true)} />
      </div>

      <div className="wh-heading-row" style={{
      marginTop: 28
    }}>
        <div>
          <h2 className="wh-title" style={{
          fontSize: 19
        }}>
            ประวัติการแสกน ({filtered.length})
          </h2>
        </div>
      </div>

      <div className="prp-card">
        <PeriodRangePicker mode={periodMode} onModeChange={handlePeriodModeChange} anchor={periodAnchor} onAnchorChange={setPeriodAnchor} label="ช่วงวันที่ตรวจสอบ (Check Date)" countLabel={`${filtered.length} รายการ`} onClear={clearPeriod} />
      </div>

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
        </div>
        <div className="wh-search-group">
          <div className="wh-part-filter">
            <SelectField value={partFilter} onChange={setPartFilter} options={partOptions} />
          </div>
          <input className="wh-search" type="text" placeholder="ค้นหา MC# / รายการ / P/N / S/N / ผู้จ่าย" value={search} onChange={e => setSearch(e.target.value)} />
        </div>
      </div>

      <div className="wh-table-card">
        <table className="wh-table">
          <thead>
            <tr>
              <th>ITEM</th>
              <th>Check Date</th>
              <th>MC#</th>
              <th>รายการ</th>
              <th>P/N</th>
              <th>S/N</th>
              <th>ลูกค้า</th>
              <th>ผู้จ่าย</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {loading && <tr>
                <td colSpan={9} className="wh-empty-cell">
                  กำลังโหลดข้อมูล...
                </td>
              </tr>}
            {!loading && paged.map((r, idx) => <tr key={r.id}>
                  <td className="wh-cell-head" data-label="ITEM">
                    {(page - 1) * pageSize + idx + 1}
                  </td>
                  <td data-label="Check Date">{fmtDate(r.issuedAt)}</td>
                  <td className="il-mono" data-label="MC#">
                    <strong>{r.machineNo}</strong>
                  </td>
                  <td data-label="รายการ">{r.label}</td>
                  <td className="il-mono" data-label="P/N">
                    {r.partNo || DASH}
                  </td>
                  <td className="il-mono" data-label="S/N">
                    {r.serialNo || r.mfgSerialNo || DASH}
                  </td>
                  <td data-label="ลูกค้า">{r.customer || DASH}</td>
                  <td data-label="ผู้จ่าย">{r.issuedBy || DASH}</td>
                  <td className="wh-cell-action">
                    {/* สแกนจ่ายไปแล้วถือเป็นหลักฐานการจ่ายของ ลบออกเองไม่ได้ */}
                    <button className="tsf-action-btn" onClick={() => setDetailRow(r)}>
                      รายละเอียด
                    </button>
                  </td>
                </tr>)}
            {!loading && filtered.length === 0 && <tr>
                <td colSpan={9} className="wh-empty-cell">
                  {issues.length === 0 ? 'ยังไม่มีรายการ — เลือก MC# ด้านบนแล้วสแกน' : 'ไม่พบรายการตามตัวกรอง'}
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

      {detailRow && <div className="wh-modal-overlay" onClick={() => setDetailRow(null)}>
          <div className="wh-modal wh-detail-modal" onClick={e => e.stopPropagation()}>
            <button type="button" className="wh-detail-close" onClick={() => setDetailRow(null)} aria-label="ปิด">
              <XMarkIcon className="size-4" />
            </button>

            <div className="wh-detail-header">
              <span className="wh-detail-header-icon">
                <CubeIcon className="size-5" />
              </span>
              <div>
                <h3 className="wh-modal-title">รายละเอียดการจ่ายของ</h3>
                <span className="wh-detail-header-sub">{detailRow.label}</span>
              </div>
            </div>

            <div className="wh-detail-section">
              <span className="wh-detail-section-title">
                <DocumentTextIcon className="size-4" /> ข้อมูลเครื่อง
              </span>
              <div className="wh-detail-grid">
                <div className="wh-detail-item">
                  <span className="wh-detail-label">MC#</span>
                  <span className="wh-detail-value mono">{detailRow.machineNo || DASH}</span>
                </div>
                <div className="wh-detail-item">
                  <span className="wh-detail-label">Spec code</span>
                  <span className="wh-detail-value mono">{detailRow.specCode || DASH}</span>
                </div>
                <div className="wh-detail-item">
                  <span className="wh-detail-label">LOT</span>
                  <span className="wh-detail-value">{detailRow.lotNo || DASH}</span>
                </div>
                <div className="wh-detail-item">
                  <span className="wh-detail-label">ลูกค้า</span>
                  <span className="wh-detail-value">{detailRow.customer || DASH}</span>
                </div>
              </div>
            </div>

            <div className="wh-detail-divider" />

            <div className="wh-detail-section">
              <span className="wh-detail-section-title">
                <CheckCircleIcon className="size-4" /> ของที่จ่าย
              </span>
              <div className="wh-detail-grid">
                <div className="wh-detail-item">
                  <span className="wh-detail-label">รายการ</span>
                  <span className="wh-detail-value">{detailRow.label}</span>
                </div>
                <div className="wh-detail-item">
                  <span className="wh-detail-label">P/N</span>
                  <span className="wh-detail-value mono">{detailRow.partNo || DASH}</span>
                </div>
                <div className="wh-detail-item">
                  <span className="wh-detail-label">S/N</span>
                  <span className="wh-detail-value mono">{detailRow.serialNo || DASH}</span>
                </div>
                <div className="wh-detail-item">
                  <span className="wh-detail-label">สถานะฝั่ง MFG</span>
                  <span className="wh-detail-value">
                    {detailRow.mfgConfirmed ? 'ยืนยันการประกอบแล้ว' : 'รอ MFG ยืนยัน'}
                  </span>
                </div>
                {detailRow.mfgSerialNo && <div className="wh-detail-item">
                    <span className="wh-detail-label">S/N# ที่ MFG สแกน</span>
                    <span className="wh-detail-value mono">{detailRow.mfgSerialNo}</span>
                  </div>}
              </div>
            </div>

            <div className="wh-detail-meta">
              <span>จ่ายโดย {detailRow.issuedBy || DASH}</span>
              <span>{fmtDate(detailRow.issuedAt)}</span>
            </div>

            <div className="wh-modal-actions">
              <button className="wh-modal-cancel" onClick={() => setDetailRow(null)}>
                ปิด
              </button>
            </div>
          </div>
        </div>}
    </>;
}
