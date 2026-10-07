import { useEffect, useMemo, useState } from 'react';
import AppShell from '../../components/AppShell.jsx';
import SelectField from '../../components/Selectfield.jsx';
import PartTag, { partTagLabel } from '../../components/Parttag.jsx';
import PeriodRangePicker from '../../components/PeriodRangePicker.jsx';
import { inPeriod, periodRangeLabel, periodFileTag } from '../../lib/dateRange.js';
import { ArrowDownTrayIcon, CheckCircleIcon, XMarkIcon } from '../../components/icons.jsx';
import { getQAWHRows, getQAMFGRows } from '../../api/qaConfirmed.js';
import { getMachinePlans, indexMachinePlans, lookupMachinePlan } from '../../api/machinePlans.js';
import { API_BASE_URL } from '../../api/client.js';
import { jsPDF } from 'jspdf';
import { autoTable } from 'jspdf-autotable';
import { SARABUN_REGULAR_BASE64, SARABUN_BOLD_BASE64 } from '../../lib/sarabunFont.js';
import { buildStyledXlsxWorkbookBlob, downloadBlob } from '../../lib/xlsx.js';
const navItems = [{
  to: '/qa',
  label: 'ตรวจสอบ QA',
  icon: <CheckCircleIcon className="size-4" />
}];
const COMPONENT_SHEET_ORDER = ['ITC', 'CV', 'SM', 'MP', 'PH', 'EN', 'CW'];
const SOURCES = [{
  value: 'wh',
  label: 'WH'
}, {
  value: 'mfg',
  label: 'MFG'
}];
const NO_MODEL = '(ไม่ระบุรายการ)';

// คอลัมน์ของแต่ละตาราง — ใช้ร่วมกันทั้งตารางบนจอ / PDF / Excel
const COLUMNS = {
  // WH: ของที่จ่ายออกจากคลังตามแผน Planning WH / Master Data
  wh: [{
    key: 'machineNo',
    header: 'MC#',
    xlsx: 'text',
    mono: true
  }, {
    key: 'componentLabel',
    header: 'ชนิดพาร์ท',
    xlsx: 'center',
    width: 16
  }, {
    key: 'partNo',
    header: 'P/N',
    xlsx: 'text',
    mono: true
  }, {
    key: 'serialNo',
    header: 'S/N',
    xlsx: 'text',
    mono: true
  }, {
    key: 'by',
    header: 'จ่ายโดย',
    xlsx: 'text'
  }, {
    key: 'dateLabel',
    header: 'วันที่สแกน',
    xlsx: 'center',
    width: 16
  }, {
    key: 'status',
    header: 'Status',
    xlsx: 'center',
    width: 12
  }],
  // MFG: รายการที่ยืนยันประกอบแล้ว
  mfg: [{
    key: 'machineNo',
    header: 'MC#',
    xlsx: 'text',
    mono: true
  }, {
    key: 'componentLabel',
    header: 'รายการ',
    xlsx: 'center',
    width: 16
  }, {
    key: 'specCode',
    header: 'Spec Code',
    xlsx: 'center',
    width: 20
  }, {
    key: 'itDevice',
    header: 'IT device',
    xlsx: 'center',
    width: 22
  }, {
    key: 'partNo',
    header: 'P/N',
    xlsx: 'text',
    mono: true
  }, {
    key: 'serialNo',
    header: 'S/N',
    xlsx: 'text',
    mono: true
  }, {
    key: 'by',
    header: 'ประกอบโดย',
    xlsx: 'text'
  }, {
    key: 'dateLabel',
    header: 'วันที่ประกอบ',
    xlsx: 'center',
    width: 16
  }, {
    key: 'photo',
    header: 'รูปถ่าย',
    xlsx: 'image',
    width: 14,
    photo: true
  }, {
    key: 'status',
    header: 'Status',
    xlsx: 'center',
    width: 12
  }]
};
const dash = v => v && String(v).trim() !== '' ? v : '—';
const pad2 = n => String(n).padStart(2, '0');
function registerThaiFont(doc) {
  doc.addFileToVFS('Sarabun-Regular.ttf', SARABUN_REGULAR_BASE64);
  doc.addFont('Sarabun-Regular.ttf', 'Sarabun', 'normal');
  doc.addFileToVFS('Sarabun-Bold.ttf', SARABUN_BOLD_BASE64);
  doc.addFont('Sarabun-Bold.ttf', 'Sarabun', 'bold');
  doc.setFont('Sarabun', 'normal');
}
async function fetchImageAsDataURL(url) {
  try {
    const res = await fetch(url);
    if (!res.ok) return null;
    const blob = await res.blob();
    return await new Promise(resolve => {
      const reader = new FileReader();
      reader.onloadend = () => resolve(reader.result);
      reader.onerror = () => resolve(null);
      reader.readAsDataURL(blob);
    });
  } catch {
    return null;
  }
}
async function fetchImageForXlsx(url) {
  try {
    const res = await fetch(url);
    if (!res.ok) return null;
    const blob = await res.blob();
    let ext = '';
    const t = (blob.type || '').toLowerCase();
    if (t.includes('png')) ext = 'png';else if (t.includes('jpeg') || t.includes('jpg')) ext = 'jpeg';
    if (!ext) {
      if (/\.png(\?|$)/i.test(url)) ext = 'png';else if (/\.jpe?g(\?|$)/i.test(url)) ext = 'jpeg';
    }
    if (ext !== 'png' && ext !== 'jpeg') return null;
    const buf = await blob.arrayBuffer();
    return {
      bytes: new Uint8Array(buf),
      ext
    };
  } catch {
    return null;
  }
}
function imageFormatFromDataURL(dataUrl) {
  const m = /^data:image\/(\w+);base64,/.exec(dataUrl || '');
  if (!m) return null;
  const ext = m[1].toLowerCase();
  if (ext === 'jpg' || ext === 'jpeg') return 'JPEG';
  if (ext === 'png') return 'PNG';
  return null;
}
function toYMD(d) {
  return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`;
}
function thaiDateLabel(ymd) {
  const [y, m, d] = ymd.split('-').map(Number);
  return new Date(y, m - 1, d).toLocaleDateString('th-TH', {
    year: 'numeric',
    month: 'long',
    day: 'numeric'
  });
}
function savePdf(doc, filename) {
  const blob = doc.output('blob');
  const url = URL.createObjectURL(blob);
  const isMobile = /Android|iPhone|iPad|iPod/i.test(navigator.userAgent || '');
  if (isMobile) {
    const win = window.open(url, '_blank');
    if (!win) {
      const a = document.createElement('a');
      a.href = url;
      a.download = filename;
      document.body.appendChild(a);
      a.click();
      a.remove();
    }
  } else {
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    a.remove();
  }
  setTimeout(() => URL.revokeObjectURL(url), 60_000);
}
const fmtNum = n => Number(n || 0).toLocaleString('en-US');
function specItDevice(row) {
  if (!row?.SpecDetail) return '';
  try {
    const spec = JSON.parse(row.SpecDetail);
    const item = (spec?.items || []).find(it => it.field === 'itDevice');
    return item ? item.plan || item.qr || '' : '';
  } catch {
    return '';
  }
}
function safeSheetName(name, used) {
  let base = String(name || 'อื่นๆ').replace(/[\[\]:*?/\\]/g, ' ').trim().slice(0, 31) || 'อื่นๆ';
  let unique = base;
  let n = 2;
  while (used.has(unique)) {
    unique = `${base.slice(0, 27)} (${n})`;
    n += 1;
  }
  used.add(unique);
  return unique;
}
// แบ่งชีต Excel: WH แยกตามชนิดพาร์ท, MFG แยกตาม Model รถ
function groupForExcel(source, list) {
  const groups = new Map();
  list.forEach(r => {
    const key = source === 'mfg' ? r.componentLabel || NO_MODEL : r.component || 'OTHER';
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key).push(r);
  });
  let keys;
  if (source !== 'mfg') {
    const known = COMPONENT_SHEET_ORDER.filter(c => groups.has(c));
    const other = [...groups.keys()].filter(c => !COMPONENT_SHEET_ORDER.includes(c)).sort();
    keys = [...known, ...other];
  } else {
    keys = [...groups.keys()].filter(k => k !== NO_MODEL).sort((a, b) => a.localeCompare(b));
    if (groups.has(NO_MODEL)) keys.push(NO_MODEL);
  }
  const used = new Set();
  return keys.map(key => {
    const rows = groups.get(key);
    const label = source === 'mfg' ? key : partTagLabel(key) || rows[0]?.componentLabel || key;
    return {
      key,
      sheetName: safeSheetName(label, used),
      rows
    };
  });
}
export default function QAMachineList() {
  const [source, setSource] = useState('wh');
  const [whRaw, setWhRaw] = useState([]);
  const [mfgRaw, setMfgRaw] = useState([]);
  const [planIndex, setPlanIndex] = useState(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [photoView, setPhotoView] = useState(null);
  const [search, setSearch] = useState('');
  const [compFilter, setCompFilter] = useState('all');
  const [pageSize, setPageSize] = useState(10);
  const [page, setPage] = useState(1);
  const [exportingPDF, setExportingPDF] = useState(false);
  const [downloading, setDownloading] = useState(false);
  const [periodMode, setPeriodMode] = useState('all');
  const [periodAnchor, setPeriodAnchor] = useState('');
  async function loadAll() {
    setLoading(true);
    setLoadError('');
    try {
      const [wh, mfg, plans] = await Promise.all([getQAWHRows(), getQAMFGRows(), getMachinePlans().catch(() => null)]);
      setWhRaw(Array.isArray(wh) ? wh : []);
      setMfgRaw(Array.isArray(mfg) ? mfg : []);
      setPlanIndex(plans ? indexMachinePlans(plans?.rows || []) : null);
    } catch (err) {
      setLoadError(err.message || 'โหลดข้อมูลไม่สำเร็จ');
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => {
    loadAll();
  }, []);
  useEffect(() => {
    function handleFocus() {
      loadAll();
    }
    window.addEventListener('focus', handleFocus);
    return () => window.removeEventListener('focus', handleFocus);
  }, []);
  useEffect(() => {
    setPage(1);
  }, [source, search, pageSize, periodMode, periodAnchor, compFilter]);

  // WH: เฉพาะที่สแกนผ่าน (MATCH)
  const whRows = useMemo(() => whRaw.filter(r => r.MatchStatus === 'MATCH').map(r => ({
    key: `wh-${r.ID}`,
    date: r.CheckedDatetime,
    dateLabel: r.CheckedDatetime ? thaiDateLabel(toYMD(new Date(r.CheckedDatetime))) : '—',
    component: String(r.PartType || '').toUpperCase(),
    componentLabel: partTagLabel(r.PartType) || r.PartType || '—',
    machineNo: r.MachineNo,
    partNo: r.PN,
    serialNo: r.SN,
    by: r.CheckedBy,
    status: 'MATCHED'
  })), [whRaw]);

  // MFG: เฉพาะที่ประกอบผ่าน (MATCHED)
  const mfgRows = useMemo(() => mfgRaw.filter(r => r.Status === 'MATCHED').map(r => {
    const asm = lookupMachinePlan(planIndex, r.MachineNo, r.ITControllerNo);
    return {
      key: `mfg-${r.ID}`,
      date: r.CheckDate,
      dateLabel: r.CheckDate ? thaiDateLabel(toYMD(new Date(r.CheckDate))) : '—',
      machineNo: r.MachineNo,
      component: String(r.Component || '').toUpperCase(),
      componentLabel: r.ComponentLabel || partTagLabel(r.Component) || r.Component || '—',
      specCode: r.SpecCode || asm?.specCode || '',
      itDevice: r.ITDevice || asm?.itDevice || specItDevice(r),
      partNo: r.PartNo,
      serialNo: r.SerialNo,
      country: r.Country || asm?.country || '',
      by: r.CreatedBy,
      photoURL: r.PhotoURL,
      status: 'MATCHED'
    };
  }), [mfgRaw, planIndex]);

  const sourceRows = source === 'wh' ? whRows : mfgRows;
  const columns = COLUMNS[source];
  const sourceLabel = source === 'wh' ? 'WH' : 'MFG';
  const dateBounds = useMemo(() => {
    let min = null;
    let max = null;
    sourceRows.forEach(r => {
      if (!r.date) return;
      const ymd = toYMD(new Date(r.date));
      if (min === null || ymd < min) min = ymd;
      if (max === null || ymd > max) max = ymd;
    });
    return {
      min,
      max
    };
  }, [sourceRows]);
  function clearDateFilter() {
    setPeriodMode('all');
    setPeriodAnchor('');
  }
  function handlePeriodModeChange(next) {
    setPeriodMode(next);
    if (next !== 'all' && !periodAnchor) {
      setPeriodAnchor(dateBounds.max || toYMD(new Date()));
    }
  }
  const periodLabel = periodMode === 'all' ? 'ทั้งหมด' : periodRangeLabel(periodMode, periodAnchor);
  const periodTag = periodFileTag(periodMode, periodAnchor);
  // ตัวเลือกของตัวกรองชนิดพาร์ท — เรียงตามลำดับมาตรฐาน แล้วค่อยชนิดอื่น
  const compOptions = useMemo(() => {
    const present = new Set(sourceRows.map(r => r.component).filter(Boolean));
    const known = COMPONENT_SHEET_ORDER.filter(c => present.has(c));
    const other = [...present].filter(c => !COMPONENT_SHEET_ORDER.includes(c)).sort();
    return [{
      value: 'all',
      label: 'ทุกรายการ'
    }, ...[...known, ...other].map(c => ({
      value: c,
      label: partTagLabel(c) || c
    }))];
  }, [sourceRows]);
  useEffect(() => {
    if (compFilter !== 'all' && !compOptions.some(o => o.value === compFilter)) {
      setCompFilter('all');
    }
  }, [compOptions, compFilter]);
  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    return sourceRows.filter(r => {
      if (compFilter !== 'all' && r.component !== compFilter) return false;
      if (periodMode !== 'all') {
        if (!r.date || !inPeriod(r.date, periodMode, periodAnchor)) return false;
      }
      if (q) {
        return columns.some(c => String(r[c.key] ?? '').toLowerCase().includes(q)) || String(r.component || '').toLowerCase().includes(q);
      }
      return true;
    }).sort((a, b) => new Date(a.date || 0) - new Date(b.date || 0));
  }, [sourceRows, columns, search, periodMode, periodAnchor, compFilter]);
  const totalPages = Math.max(1, Math.ceil(filtered.length / pageSize));
  const pageRows = filtered.slice((page - 1) * pageSize, page * pageSize);
  const machineCount = useMemo(() => new Set(filtered.map(r => r.machineNo || r.no).filter(Boolean)).size, [filtered]);
  function cellText(r, c) {
    if (c.photo) return '';
    return dash(r[c.key]);
  }
  async function handleExportPDF() {
    const list = filtered;
    if (!list.length || exportingPDF) return;
    setExportingPDF(true);
    try {
      const photoIdx = columns.findIndex(c => c.photo) + 1;
      const photoDataUrls = photoIdx > 0 ? await Promise.all(list.map(r => r.photoURL ? fetchImageAsDataURL(`${API_BASE_URL}${r.photoURL}`) : Promise.resolve(null))) : [];
      const doc = new jsPDF({
        orientation: 'landscape',
        unit: 'mm',
        format: 'a4'
      });
      registerThaiFont(doc);
      const now = new Date();
      const printedStr = `${now.toLocaleDateString('th-TH', {
        year: 'numeric',
        month: 'long',
        day: 'numeric'
      })} ${now.toLocaleTimeString('th-TH', {
        hour: '2-digit',
        minute: '2-digit'
      })}`;
      const head = [['ITEM', ...columns.map(c => c.header)]];
      const body = list.map((r, i) => [String(i + 1), ...columns.map(c => cellText(r, c))]);
      const compTitle = compFilter === 'all' ? '' : ` — ${partTagLabel(compFilter) || compFilter}`;
      const title = (source === 'wh' ? 'QA Check Sheet — WH (จ่ายของตามแผน)' : 'QA Check Sheet — MFG (ประกอบ)') + compTitle;
      const drawHeader = () => {
        doc.setFont('Sarabun', 'bold');
        doc.setFontSize(16);
        doc.setTextColor(15, 23, 42);
        doc.text(title, 10, 14);
        doc.setFont('Sarabun', 'normal');
        doc.setFontSize(9);
        doc.setTextColor(71, 85, 105);
        const pageWidth = doc.internal.pageSize.getWidth();
        doc.text(`ช่วงวันที่: ${periodLabel}`, pageWidth - 10, 12, {
          align: 'right'
        });
        doc.text(`วันที่พิมพ์: ${printedStr}`, pageWidth - 10, 17, {
          align: 'right'
        });
        doc.text(`จำนวน: ${list.length} รายการ`, pageWidth - 10, 22, {
          align: 'right'
        });
      };
      const columnStyles = {
        0: {
          halign: 'center',
          cellWidth: 10
        }
      };
      if (photoIdx > 0) columnStyles[photoIdx] = {
        halign: 'center',
        cellWidth: 16,
        minCellHeight: 14
      };
      autoTable(doc, {
        head,
        body,
        startY: 26,
        margin: {
          top: 26,
          left: 8,
          right: 8,
          bottom: 12
        },
        styles: {
          font: 'Sarabun',
          fontSize: 8,
          cellPadding: 1.5,
          lineColor: [148, 163, 184],
          lineWidth: 0.1,
          valign: 'middle'
        },
        headStyles: {
          font: 'Sarabun',
          fontStyle: 'bold',
          fillColor: [241, 245, 249],
          textColor: [15, 23, 42],
          halign: 'center'
        },
        columnStyles,
        didDrawPage: drawHeader,
        didDrawCell: data => {
          if (photoIdx <= 0 || data.section !== 'body' || data.column.index !== photoIdx) return;
          const dataUrl = photoDataUrls[data.row.index];
          if (!dataUrl) return;
          const fmt = imageFormatFromDataURL(dataUrl);
          if (!fmt) return;
          try {
            const pad = 1;
            const size = Math.min(data.cell.height, data.cell.width) - pad * 2;
            const x = data.cell.x + (data.cell.width - size) / 2;
            const y = data.cell.y + (data.cell.height - size) / 2;
            doc.addImage(dataUrl, fmt, x, y, size, size);
          } catch {}
        }
      });
      const pageWidth = doc.internal.pageSize.getWidth();
      const pageHeight = doc.internal.pageSize.getHeight();
      let signY = doc.lastAutoTable.finalY + 18;
      if (signY + 14 > pageHeight - 10) {
        doc.addPage();
        drawHeader();
        signY = 40;
      }
      doc.setFont('Sarabun', 'normal');
      doc.setFontSize(9);
      doc.setTextColor(15, 23, 42);
      [{
        label: 'ผู้ตรวจสอบ (QA)',
        cx: pageWidth * 0.2
      }, {
        label: 'ผู้อนุมัติ',
        cx: pageWidth * 0.5
      }, {
        label: 'วันที่',
        cx: pageWidth * 0.8
      }].forEach(c => {
        doc.line(c.cx - 25, signY, c.cx + 25, signY);
        doc.text(c.label, c.cx, signY + 5, {
          align: 'center'
        });
      });
      savePdf(doc, `QA-CheckSheet-${sourceLabel}-${periodTag}.pdf`);
    } catch (err) {
      console.error(err);
      alert('สร้าง PDF ไม่สำเร็จ กรุณาลองใหม่');
    } finally {
      setExportingPDF(false);
    }
  }

  // Excel: กดแล้วดาวน์โหลดทันที (WH แยกชีตตามชนิดพาร์ท / MFG แยกชีตตาม Model รถ)
  async function handleExportExcel() {
    if (!filtered.length || downloading) return;
    setDownloading(true);
    try {
      const hasPhoto = columns.some(c => c.photo);
      const sheets = [];
      for (const g of groupForExcel(source, filtered)) {
        const photos = hasPhoto ? await Promise.all(g.rows.map(r => r.photoURL ? fetchImageForXlsx(`${API_BASE_URL}${r.photoURL}`) : Promise.resolve(null))) : [];
        const xlsxColumns = [{
          key: 'item',
          header: 'ITEM',
          type: 'number',
          width: 8
        }, ...columns.map(c => ({
          key: c.key,
          header: c.header,
          type: c.xlsx,
          ...(c.width ? {
            width: c.width
          } : {})
        }))];
        const rows = g.rows.map((r, i) => {
          const out = {
            item: i + 1
          };
          columns.forEach(c => {
            out[c.key] = c.photo ? photos[i] || null : dash(r[c.key]);
          });
          return out;
        });
        sheets.push({
          sheetName: g.sheetName,
          columns: xlsxColumns,
          rows
        });
      }
      downloadBlob(buildStyledXlsxWorkbookBlob({
        sheets
      }), `QA-${sourceLabel}-${periodTag}.xlsx`);
    } catch (err) {
      console.error(err);
      alert('สร้าง Excel ไม่สำเร็จ กรุณาลองใหม่');
    } finally {
      setDownloading(false);
    }
  }
  function renderCell(r, c) {
    if (c.key === 'componentLabel') return r.component ? <PartTag code={r.component} label={r.componentLabel} /> : '—';
    if (c.photo) {
      return r.photoURL ? <button type="button" className="qa-photo-thumb" onClick={() => setPhotoView(r.photoURL)} title="ดูรูปถ่าย">
          <img src={`${API_BASE_URL}${r.photoURL}`} alt="รูปถ่ายป้าย" loading="lazy" />
        </button> : '—';
    }
    if (c.key === 'status') return <span className="il-badge il-badge-ok">MATCHED</span>;
    return dash(r[c.key]);
  }
  return <AppShell navItems={navItems} roleLabel="QA">
      <div className="wh-heading-row">
        <div>
          <h2 className="wh-title">QA</h2>
        </div>
      </div>

      <div className="vr-tabs qa-source-tabs" role="tablist">
        {SOURCES.map(opt => <button key={opt.value} type="button" role="tab" aria-selected={source === opt.value} className={'vr-tab' + (source === opt.value ? ' vr-tab-active' : '')} onClick={() => setSource(opt.value)}>
            {opt.label}
            <span className="qa-source-count">{fmtNum(opt.value === 'wh' ? whRows.length : mfgRows.length)}</span>
          </button>)}
      </div>

      <div className="qa-filter-card">
        <div className="qa-filter-top">
          <PeriodRangePicker mode={periodMode} onModeChange={handlePeriodModeChange} anchor={periodAnchor} onAnchorChange={setPeriodAnchor} min={dateBounds.min} max={dateBounds.max} label={source === 'wh' ? 'ช่วงวันที่สแกน (WH)' : 'ช่วงวันที่ประกอบ (MFG)'} countLabel={`${filtered.length} รายการ`} onClear={clearDateFilter} />
        </div>

        <div className="qa-export-actions">
          <button className="qa-download-btn qa-export-btn" onClick={handleExportPDF} disabled={loading || filtered.length === 0 || exportingPDF} title={filtered.length === 0 ? 'ไม่มีรายการให้ออก Check Sheet' : `ดาวน์โหลด Check Sheet ${sourceLabel} — ช่วง ${periodLabel}`}>
            <ArrowDownTrayIcon className="size-4" />
            {exportingPDF ? 'กำลังสร้าง PDF...' : `Export PDF (Check Sheet ${sourceLabel})`}
          </button>
          <button className="qa-download-btn qa-export-btn qa-export-btn-excel" onClick={handleExportExcel} disabled={loading || filtered.length === 0 || downloading} title={filtered.length === 0 ? 'ไม่มีรายการให้ออก Excel' : source === 'mfg' ? 'Excel แยกชีตตามรายการ' : 'Excel แยกชีตตามชนิดพาร์ท'}>
            <ArrowDownTrayIcon className="size-4" />
            {downloading ? 'กำลังสร้าง Excel...' : `Export Excel ${sourceLabel}`}
          </button>
        </div>
      </div>
      <p className="qa-stat-sub" style={{
      marginTop: 10,
      marginBottom: 16
    }}>
        {periodMode === 'all' ? `${sourceLabel}: รายการที่ Matched ทั้งหมด ${filtered.length} รายการ / ${machineCount} เครื่อง (เลือกช่วงเพื่อออกเฉพาะรายวัน/สัปดาห์/เดือน/ปี)` : `${sourceLabel}: กำลังกรองช่วง ${periodLabel} — พบ ${filtered.length} รายการ (Export PDF/Excel จะได้เฉพาะช่วงนี้)`}
      </p>

      {loadError && <p className="form-error" role="alert">
          {loadError}
        </p>}

      <div className="wh-table-card" style={{
      overflowX: 'auto'
    }}>
        <div className="qa-table-toolbar">
          <div className="qa-table-toolbar-left">
            แสดง{' '}
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
            }]} />
            </div>{' '}
            รายการต่อหน้า
          </div>
          <div className="qa-table-toolbar-right">
            <div className="wh-filter-field">
              <span className="wh-filter-label">รายการ</span>
              <SelectField value={compFilter} onChange={setCompFilter} options={compOptions} />
            </div>
            <input className="wh-search" type="text" placeholder="ค้นหา MC# / รายการ / P/N / S/N..."  value={search} onChange={e => setSearch(e.target.value)} />
          </div>
        </div>

        <table className="wh-table">
          <thead>
            <tr>
              <th>ITEM</th>
              {columns.map(c => <th key={c.key}>{c.header}</th>)}
            </tr>
          </thead>
          <tbody>
            {loading && <tr>
                <td colSpan={columns.length + 1} className="wh-empty-cell">
                  กำลังโหลดข้อมูล...
                </td>
              </tr>}
            {!loading && pageRows.map((r, i) => <tr key={r.key}>
                <td className="wh-cell-head" data-label="ITEM">
                  <strong>{(page - 1) * pageSize + i + 1}</strong>
                </td>
                {columns.map(c => <td key={c.key} data-label={c.header} className={c.mono ? 'il-mono' : undefined}>
                    {renderCell(r, c)}
                  </td>)}
              </tr>)}
            {!loading && filtered.length === 0 && <tr>
                <td colSpan={columns.length + 1} className="wh-empty-cell">
                  {sourceRows.length === 0 ? `ยังไม่มีรายการที่ Matched ของ ${sourceLabel}` : 'ไม่พบรายการที่ค้นหา'}
                </td>
              </tr>}
          </tbody>
        </table>

        {!loading && filtered.length > 0 && <div className="qa-pagination">
            <button disabled={page <= 1} onClick={() => setPage(p => Math.max(1, p - 1))}>
              ก่อนหน้า
            </button>
            <span>
              หน้า {page} / {totalPages}
            </span>
            <button disabled={page >= totalPages} onClick={() => setPage(p => Math.min(totalPages, p + 1))}>
              ถัดไป
            </button>
          </div>}
      </div>

      {photoView && <div className="wh-modal-overlay" onClick={() => setPhotoView(null)}>
          <div className="wh-modal" style={{
        maxWidth: 520,
        textAlign: 'center'
      }} onClick={e => e.stopPropagation()}>
            <div className="wh-modal-actions" style={{
          justifyContent: 'flex-end',
          marginTop: 0
        }}>
              <button className="wh-modal-cancel" onClick={() => setPhotoView(null)} aria-label="ปิด">
                <XMarkIcon className="size-4" />
              </button>
            </div>
            <img src={`${API_BASE_URL}${photoView}`} alt="รูปถ่ายป้าย" style={{
          maxWidth: '100%',
          borderRadius: 8
        }} />
          </div>
        </div>}
    </AppShell>;
}
