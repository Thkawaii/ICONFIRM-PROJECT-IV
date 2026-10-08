import { useEffect, useMemo, useRef, useState } from 'react';
import { getMFGAssemblies, deleteMFGAssembly, uploadMFGAssemblyPhoto } from '../api/mfgAssembly.js';
import { scanFlowKanban, confirmFlowAssembly, flowComponentLabel } from '../api/whFlow.js';
import { API_BASE_URL } from '../api/client.js';
import { confirmDelete, toastSuccess, toastError } from '../lib/toast.js';
import { inPeriod } from '../lib/dateRange.js';
import PeriodRangePicker from '../components/PeriodRangePicker.jsx';
import { scanStep, scanLoading, scanClose, scanCloseWait, scanSuccessToast, scanErrorAlert, scanConfirmAlert, scanPhotoCapture } from '../lib/scanPopup.js';
import { ChevronDoubleLeftIcon, ChevronDoubleRightIcon, ChevronLeftIcon, ChevronRightIcon, CameraIcon, ArrowUpTrayIcon, ArrowsRightLeftIcon, CubeIcon, DocumentTextIcon, QrCodeIcon, WrenchScrewdriverIcon, XMarkIcon } from '../components/icons.jsx';
import AppShell from '../components/AppShell.jsx';
import SelectField from '../components/Selectfield.jsx';

// ---------------------------------------------------------------------------
// ขั้นตอน MFG
//   1. Scan MC# ที่ QR code (Kanban)
//        ระบบเอา MC# และ Product Spec บน Kanban ไปเทียบ master_data
//        ว่ามี Product Spec นี้จริงไหม และ P/N ที่ติดมากับ Kanban ตรงกันไหม
//        (ถ้ามีไฟล์ Planning MFG อยู่ในระบบ จะเอามาสอบทานเพิ่มอีกชั้น — ไม่มีก็ข้าม)
//   2. ระบบแสดง Part# ที่ WH จ่าย → MFG ตรวจว่าถูกต้องไหม
//   3. Scan S/N# (CW / CV / SM / MP / PH)
//   4. ถ่ายรูป & save
//   ไม่ตรงขั้นไหน → "ข้อมูลไม่ถูกต้อง" พร้อมสาเหตุ และชื่อ WH ที่จ่ายของ
// ---------------------------------------------------------------------------

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
export const MFG_NAV_ITEMS = [{
  to: '/mfg-assembly',
  label: 'MFG Assembly',
  icon: <ArrowsRightLeftIcon className="size-4" />,
  roles: ['MFG']
}];
const STATUS_META = {
  MATCHED: {
    label: 'MATCHED',
    cls: 'il-badge il-badge-ok'
  },
  NOT_MATCHED: {
    label: 'NOT_MATCHED',
    cls: 'il-badge il-badge-bad'
  },
  DUPLICATE: {
    label: 'DUPLICATE',
    cls: 'il-badge il-badge-warn'
  },
  RETIRED_FORMAT: {
    label: 'รูปแบบเดิมถูกยกเลิก',
    cls: 'il-badge il-badge-bad'
  }
};
const DASH = '—';
function fmtDate(value) {
  if (!value) return DASH;
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return DASH;
  const buddhistYear = d.getFullYear() + 543;
  const pad = n => String(n).padStart(2, '0');
  return `${d.getDate()}/${d.getMonth() + 1}/${buddhistYear} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}
function todayYMD() {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}
export default function MFGAssemblyPage() {
  const [rows, setRows] = useState([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [search, setSearch] = useState('');
  // กรองตามรายการ (IT Controller / Engine / CW ...) — ว่าง = ทุกรายการ
  const [partFilter, setPartFilter] = useState('');
  const [periodMode, setPeriodMode] = useState('all');
  const [periodAnchor, setPeriodAnchor] = useState('');
  const [pageSize, setPageSize] = useState(10);
  const [page, setPage] = useState(1);
  const [scanBusy, setScanBusy] = useState(false);
  const [photoView, setPhotoView] = useState(null);
  const [photoBusy, setPhotoBusy] = useState(false);
  const [photoEditRow, setPhotoEditRow] = useState(null);
  const [detailRow, setDetailRow] = useState(null);
  const photoFileInputRef = useRef(null);
  const pendingPhotoRowIdRef = useRef(null);
  const busyRef = useRef(false);
  const fireRef = useRef(() => {});
  function handlePeriodModeChange(next) {
    setPeriodMode(next);
    if (next !== 'all' && !periodAnchor) setPeriodAnchor(todayYMD());
  }
  function clearPeriod() {
    setPeriodMode('all');
    setPeriodAnchor('');
  }
  // ข้อความจากเซิร์ฟเวอร์มาก่อนเสมอ — จะเตือนเรื่อง rebuild เฉพาะตอนที่ยิง API แล้วไม่มี endpoint จริง ๆ
  function friendlyError(err, fallback) {
    const fromServer = String(err?.data?.message || '').trim();
    if (fromServer) return fromServer;
    if (err?.status === 404 || err?.status === 405) {
      return 'ยังไม่พบ API ที่ฝั่งเซิร์ฟเวอร์ — ต้อง rebuild แล้ว restart backend ก่อน';
    }
    return err?.message || fallback;
  }
  async function loadRows() {
    setLoading(true);
    setLoadError('');
    try {
      const list = await getMFGAssemblies();
      setRows(list || []);
    } catch (err) {
      setLoadError(friendlyError(err, 'โหลดข้อมูลไม่สำเร็จ'));
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => {
    loadRows();
  }, []);
  useEffect(() => {
    setPage(1);
  }, [search, partFilter, pageSize, periodMode, periodAnchor]);

  // รับค่าจากเครื่องสแกนที่ยิงเข้ามาตรง ๆ (ไม่ได้โฟกัสช่องกรอก)
  useEffect(() => {
    let buffer = '';
    let flushTimer = null;
    function fireBuffered() {
      const code = buffer.trim();
      buffer = '';
      if (code.length >= 2 && !busyRef.current) fireRef.current(code);
    }
    function onKeydown(e) {
      if (busyRef.current) return;
      const tag = (e.target?.tagName || '').toLowerCase();
      if (tag === 'input' || tag === 'textarea' || tag === 'select') return;
      if (e.target?.closest?.('[data-scan-ignore]')) return;
      if (e.key === 'Enter') {
        if (flushTimer) clearTimeout(flushTimer);
        fireBuffered();
        return;
      }
      if (e.key && e.key.length === 1) {
        buffer += e.key;
        if (buffer.length >= 2) e.preventDefault();
        if (flushTimer) clearTimeout(flushTimer);
        flushTimer = setTimeout(fireBuffered, 120);
      }
    }
    window.addEventListener('keydown', onKeydown);
    return () => {
      window.removeEventListener('keydown', onKeydown);
      if (flushTimer) clearTimeout(flushTimer);
    };
  }, []);

  // ขั้นที่ 1 — Scan Kanban: เทียบ MC# + Product Spec + P/N กับ master_data
  //             แล้วอ่านของที่ WH จ่ายมาให้เครื่องนี้
  async function runKanbanScan(presetCode = '') {
    if (busyRef.current) return;
    busyRef.current = true;
    setScanBusy(true);
    try {
      let qrCode = String(presetCode || '').trim();
      if (!qrCode) {
        qrCode = await scanStep({
          title: 'Scan Kanban',
          confirmText: 'ตรวจสอบ'
        });
        if (!qrCode) return;
        qrCode = qrCode.trim();
      }
      scanLoading('กำลังอ่านของที่ WH จ่ายมา...');
      let res = null;
      try {
        res = await scanFlowKanban({
          qrCode
        });
      } catch (err) {
        scanClose();
        await scanErrorAlert(friendlyError(err, 'ตรวจสอบ Kanban ไม่สำเร็จ'));
        return;
      }
      scanClose();
      if (res && res.found === false) {
        // แสดงแค่ "ข้อมูลไม่ถูกต้อง" พอ — res.detail เป็นคำอธิบายทางเทคนิคที่ยาวมาก
        // (ชื่อไฟล์แผน, Product Spec, master_data) คนหน้างานอ่านไม่ทันและไม่ได้ใช้ตัดสินใจ
        await scanErrorAlert(res.message || 'ข้อมูลไม่ถูกต้อง');
        return;
      }
      // Product Spec ถูก แต่ master_data ยังไม่มีรุ่นนี้ — เตือนแล้วทำงานต่อ
      // (CW / CV / SM / MP / PH ของเครื่องนี้จะยังยืนยันไม่ได้ ส่วน IT / Engine ทำต่อได้)
      if (res?.specWarning) {
        await scanErrorAlert(res.specCheck?.message || 'ไม่พบ Product Spec นี้ใน master_data', {
          hint: res.specCheck?.detail || ''
        });
      }
      await runItemChecks(res?.machine, qrCode);
    } finally {
      busyRef.current = false;
      setScanBusy(false);
      await loadRows();
    }
  }
  fireRef.current = code => runKanbanScan(code);

  // ขั้นที่ 2-4 — ไล่ถาม MFG ทีละรายการว่า Part# ที่ WH จ่ายมาถูกต้องไหม
  //   ถูกต้อง → (CW / CV / SM / MP / PH: สแกน S/N# ก่อน) → ถ่ายรูป → บันทึก
  //   ไม่ตรง  → แจ้ง "ข้อมูลไม่ถูกต้อง" พร้อมชื่อ WH ที่จ่ายของ
  async function runItemChecks(machine, qrCode) {
    if (!machine?.machineNo) {
      await scanErrorAlert('ข้อมูลไม่ถูกต้อง กรุณาติดต่อ WH');
      return;
    }
    const items = machine.items || [];
    const pending = items.filter(it => it.issued && !it.mfgConfirmed);
    if (pending.length === 0) {
      if (!items.some(it => it.issued)) {
        await scanErrorAlert(`WH ยังไม่จ่ายของให้เครื่อง ${machine.machineNo} — กรุณาติดต่อ WH`);
      } else {
        scanSuccessToast(`เครื่อง ${machine.machineNo} ยืนยันไปแล้ว`);
      }
      return;
    }
    const kanban = parseKanbanParts(qrCode);
    for (const item of pending) {
      // CW / CV / SM / MP / PH — ระบบเทียบ Part# ให้แล้วตั้งแต่ตอน WH สแกน
      // MFG จึงไม่ต้องตัดสินซ้ำ เหลือแค่ถ่ายรูปของจริงเก็บไว้
      const autoChecked = item.source === 'cw_cv_its';
      const ok = await scanConfirmAlert({
        title: itemTitle(machine, item),
        html: itemHtml(machine, item, kanban),
        confirmText: autoChecked ? 'ถ่ายรูป' : 'ถูกต้อง',
        denyText: autoChecked ? null : 'ไม่ตรง',
        cancelText: autoChecked ? 'ปิด' : null
      });
      if (ok === null) break;
      if (ok === 'no') {
        await scanErrorAlert(contactWH('ข้อมูลไม่ถูกต้อง', item.issuedBy));
        continue;
      }
      await confirmOneItem(machine, item, qrCode);
    }
  }
  // IT Controller: ใช้ชนิด IT ของเครื่องเป็นหัวข้อ เช่น IT(Satellite, iridium)
  // รายการอื่นใช้ชื่อรายการตามปกติ เช่น Engine / Counter Weight
  function itemTitle(machine, item) {
    if (item.component === 'ITC' && machine.itDevice) return machine.itDevice;
    return item.label;
  }
  // QR บน Kanban ใช้ "," เป็นตัวคั่นช่อง ถ้าน้ำหนักถ่วงเขียนคั่นหลักพัน
  // (เช่น LB60C00210P1_6,200KG) จุลภาคนั้นจะถูกนับเป็นตัวคั่นช่องไปด้วย
  // ทำให้อ่านน้ำหนักได้แค่ "6" และช่องที่เหลือเลื่อนไปทั้งแถว — ต่อกลับให้ก่อน
  // เงื่อนไขแคบมาก เพื่อไม่ให้ไปโดนช่องอื่นที่ขึ้นต้นด้วยตัวเลข เช่น "800mm HD grouser shoe"
  const WEIGHT_HEAD = /[_#][0-9]{1,3}$/;
  const WEIGHT_TAIL = /^[0-9]{3}\s*(KGS?|KILOGRAMS?|TONN?E?S?|MT|T|G|LBS?)?$/i;
  function mergeThousandSeparator(fields) {
    const out = [];
    for (let i = 0; i < fields.length; i++) {
      let cur = fields[i];
      while (i + 1 < fields.length && WEIGHT_HEAD.test(String(cur).trim()) && WEIGHT_TAIL.test(String(fields[i + 1]).trim())) {
        cur = String(cur).trim() + String(fields[i + 1]).trim();
        i++;
      }
      out.push(cur);
    }
    return out;
  }

  // ดึงค่า Part# ที่ติดมากับ QR ของ Kanban
  // หมายเหตุ: QR มี P/N ของ Counter Weight ช่องเดียว (ช่องที่ 7 เขียนเป็น "P/N_น้ำหนัก")
  // ส่วน CV / SM / MP / PH ไม่มีอยู่บน Kanban จึงดึงมาแสดงไม่ได้
  function parseKanbanParts(qrCode) {
    const f = mergeThousandSeparator(String(qrCode || '').split(','));
    if (f.length < 7) return {};
    // ช่อง CW เขียนติดกันสองอย่าง: "รหัส P/N" _ "น้ำหนักถ่วง" เช่น YN60C00942P1_4.3T
    const cw = String(f[6] || '').trim();
    const cut = cw.search(/[_#]/);
    return {
      CW: cut > 0 ? cw.slice(0, cut).trim() : cw,
      CW_WEIGHT: cut > 0 ? cw.slice(cut + 1).trim() : ''
    };
  }

  // หน่วยน้ำหนักที่เขียนได้หลายแบบ → หน่วยมาตรฐาน (กติกาเดียวกับ backend)
  // ไม่เขียนหน่วย = ตัน เพราะคอลัมน์ใน master_data คือ Weight (Tons)
  const WEIGHT_UNITS = {
    '': 'T',
    T: 'T',
    TON: 'T',
    TONS: 'T',
    TONNE: 'T',
    TONNES: 'T',
    MT: 'T',
    KG: 'KG',
    KGS: 'KG',
    KGM: 'KG',
    KILOGRAM: 'KG',
    KILOGRAMS: 'KG',
    G: 'G',
    GRAM: 'G',
    GRAMS: 'G',
    LB: 'LB',
    LBS: 'LB',
    POUND: 'LB',
    POUNDS: 'LB'
  };

  // อ่านน้ำหนักเป็น { value, unit } — รับได้ทั้ง "4.3T", "4.3 Tons", "6.2KG", "4,3"
  // "6,200" → "6200" (คั่นหลักพัน) / "4,3" → "4.3" (จุดทศนิยมแบบยุโรป)
  function normalizeNumberSeparators(v) {
    let s = String(v || '');
    // วนจนไม่เหลือตัวคั่นหลักพัน (เช่น "1,000,000")
    for (;;) {
      const next = s.replace(/(\d),(\d{3})(?!\d)/, '$1$2');
      if (next === s) break;
      s = next;
    }
    return s.replace(/,/g, '.');
  }
  function parseWeight(v) {
    const s = normalizeNumberSeparators(String(v || '')).trim().toUpperCase();
    const m = s.match(/([0-9]*\.?[0-9]+)\s*([A-Z]*)/);
    if (!m) return null;
    const value = parseFloat(m[1]);
    if (Number.isNaN(value)) return null;
    const raw = m[2] || '';
    return {
      value,
      unit: WEIGHT_UNITS[raw] || raw
    };
  }
  // อัตราแปลงหน่วย → กิโลกรัม (หน่วยที่ไม่อยู่ในตารางนี้ = แปลงไม่ได้)
  const WEIGHT_TO_KG = {
    T: 1000,
    KG: 1,
    G: 0.001,
    LB: 0.45359237
  };
  function weightInKg(v) {
    const w = parseWeight(v);
    if (!w) return null;
    const rate = WEIGHT_TO_KG[w.unit];
    if (!rate) return null;
    return w.value * rate;
  }
  // ตัวเลขเท่ากันไหม โดยไม่สนหน่วย — ใช้แยกกรณี "ตัวเลขเดียวกันแต่คนละหน่วย"
  function sameWeightNumber(a, b) {
    const x = parseWeight(a);
    const y = parseWeight(b);
    if (!x || !y) return false;
    return Math.abs(x.value - y.value) < 0.005;
  }
  // น้ำหนักเดียวกันไหม — แปลงหน่วยให้ก่อนเทียบ (กติกาเดียวกับฝั่ง backend)
  //   6.2T = 6200KG  ← คนละหน่วย แต่น้ำหนักเท่ากัน
  //   6.2T ≠ 6.2KG   ← ต่างกัน 1000 เท่า
  // ยอมคลาดเคลื่อนได้ไม่ถึง 5 กก.
  function sameTons(a, b) {
    const ka = weightInKg(a);
    const kb = weightInKg(b);
    if (ka !== null && kb !== null) return Math.abs(ka - kb) < 5;

    // หน่วยที่ระบบไม่รู้จัก — ต้องเขียนเหมือนกันเป๊ะและตัวเลขเท่ากัน
    const x = parseWeight(a);
    const y = parseWeight(b);
    if (!x || !y || x.unit !== y.unit) return false;
    return Math.abs(x.value - y.value) < 0.005;
  }
  function samePartNo(a, b) {
    const clean = v => {
      const s = String(v || '').trim().toUpperCase();
      const cut = s.search(/[_#]/);
      return (cut > 0 ? s.slice(0, cut) : s).replace(/[^A-Z0-9]/g, '');
    };
    const x = clean(a);
    return x !== '' && x === clean(b);
  }
  function itemHtml(machine, item, kanban = {}) {
    // cell = ช่อง label + ค่า 1 ช่อง · line = 1 บรรทัดเต็ม · pair = 2 ช่องในบรรทัดเดียว
    const cell = (label, value, mono = true, state = '') => `
        <span class="mfg-check-label">${escapeHtml(label)}</span>
        <span class="mfg-check-value${mono ? ' mono' : ''}">${escapeHtml(value || '—')}${
          state ? `<i class="mfg-check-icon">${state === 'ok' ? '✓' : '✗'}</i>` : ''
        }</span>`;
    const line = (label, value, mono = true, state = '') => `
      <div class="mfg-check-row${state ? ' mfg-check-' + state : ''}">
        ${cell(label, value, mono, state)}
      </div>`;
    // Part# กับ Weight เป็นการตรวจของชิ้นเดียวกัน วางคู่กันในบรรทัดเดียวอ่านง่ายกว่า
    const pair = (a, b) => `
      <div class="mfg-check-row mfg-check-pair${a.state ? ' mfg-check-' + a.state : ''}">
        <div class="mfg-check-cell">${cell(a.label, a.value, true, a.state)}</div>
        <div class="mfg-check-cell">${cell(b.label, b.value, true, b.state)}</div>
      </div>`;

    const specCode = machine.partSpecCode || machine.specCode || '';
    const fromMaster = item.source === 'cw_cv_its';

    // ตอน WH สแกน ระบบเทียบกับ master_data ไปแล้ว จึงไม่ต้องโชว์ซ้ำ
    // บรรทัดที่สองเอาค่าจาก Kanban มาเทียบแทน ให้เป็นการตรวจคนละแหล่ง
    const kanbanPartNo = kanban[item.component] || '';
    let state = '';
    if (fromMaster && kanbanPartNo) {
      state = samePartNo(item.issuedPartNo, kanbanPartNo) ? 'ok' : 'bad';
    }

    // Counter Weight เทียบน้ำหนักถ่วงอีกชั้น รหัส P/N ตรงอย่างเดียวไม่พอ
    // เพราะ CW รหัสเดียวกันมีได้หลายน้ำหนัก
    const kanbanWeight = item.component === 'CW' ? kanban.CW_WEIGHT || '' : '';
    const masterWeight = item.expectedWeightTons || '';
    let weightState = '';
    if (masterWeight) {
      weightState = kanbanWeight && sameTons(kanbanWeight, masterWeight) ? 'ok' : 'bad';
    }
    // ตัวเลขเท่ากันแต่คนละหน่วย (เช่น 6.2KG กับ 6.2T = ต่างกัน 1000 เท่า) — บอกให้ชัด
    // ไม่งั้นคนอ่านจะมองผ่านว่า "ก็ 6.2 เหมือนกัน"
    const wrongUnit = weightState === 'bad' && sameWeightNumber(kanbanWeight, masterWeight);
    const kanbanWeightLabel = wrongUnit ? 'Weight (Kanban) — คนละหน่วย' : 'Weight (Kanban)';

    return `
      <div class="mfg-check-box">
        ${line('MC#', machine.machineNo)}
        ${specCode ? line('Product Spec', specCode) : ''}
        ${line('ลูกค้า / ประเทศ', machine.customer, false)}
        ${masterWeight ? pair({
          label: fromMaster ? 'Part# (WH จ่าย)' : 'P/N',
          value: item.issuedPartNo,
          state
        }, {
          label: 'Weight (master_data)',
          value: masterWeight,
          state: weightState
        }) : line(fromMaster ? 'Part# (WH จ่าย)' : 'P/N', item.issuedPartNo, true, state)}
        ${masterWeight ? pair({
          label: 'Part# (Kanban)',
          value: kanbanPartNo,
          state
        }, {
          label: kanbanWeightLabel,
          value: kanbanWeight || 'ไม่ได้เขียนน้ำหนักมา',
          state: weightState
        }) : fromMaster && kanbanPartNo ? line('Part# (Kanban)', kanbanPartNo, true, state) : ''}
        ${item.issuedSerialNo ? line('S/N', item.issuedSerialNo) : ''}
        ${line('จ่ายโดย', item.issuedBy, false)}
      </div>`;
  }
  function contactWH(message, issuedBy) {
    const who = String(issuedBy || '').trim();
    return who ? `${message} กรุณาติดต่อ WH: ${who}` : `${message} กรุณาติดต่อ WH`;
  }
  async function confirmOneItem(machine, item, qrCode) {
    const machineNo = machine.machineNo;
    try {
      let serialNo = '';
      if (item.mfgNeedsSerial) {
        // ขั้นที่ 3 — สแกน S/N# จากตัวของจริงที่กำลังประกอบ
        serialNo = firstToken(await scanStep({
          title: `${itemTitle(machine, item)} — สแกน S/N#`,
          html: `<div class="scan-popup-hint">MC#: <b>${escapeHtml(machineNo)}</b> / Part#: <b>${escapeHtml(item.issuedPartNo || '-')}</b></div>`,
          confirmText: 'บันทึก'
        }));
        if (!serialNo) return;
      }
      scanLoading('กำลังบันทึก...');
      const res = await confirmFlowAssembly({
        machineNo,
        component: item.component,
        qrCode,
        serialNo,
        confirmed: true
      });
      scanClose();
      if (!res?.matched) {
        if (res?.customerMismatch || res?.specMismatch || res?.partMismatch) {
          // ลูกค้า / ประเทศ, Product Spec หรือ P/N บน Kanban ไม่ตรง
          await scanErrorAlert(res.message || 'ข้อมูลไม่ถูกต้อง');
          return;
        }
        await scanErrorAlert(contactWH(res?.message || 'ข้อมูลไม่ถูกต้อง', res?.issuedBy || item.issuedBy));
        return;
      }

      // ถ่ายรูปแนบรายการที่เพิ่งยืนยัน
      const rowId = res?.row?.ID;
      if (rowId) {
        await scanCloseWait();
        const photoBlob = await scanPhotoCapture({
          title: 'ถ่ายรูปของที่ประกอบ',
          html: `<div class="scan-popup-hint">MC#: <b>${escapeHtml(machineNo)}</b> / ${escapeHtml(itemTitle(machine, item))}</div>`
        });
        if (photoBlob) {
          scanLoading('กำลังบันทึกรูป...');
          try {
            await uploadMFGAssemblyPhoto(rowId, photoBlob);
          } catch (e) {
            scanClose();
            await scanErrorAlert('บันทึกรูปไม่สำเร็จ: ' + (e.message || ''));
          }
        }
      }
      scanClose();
      scanSuccessToast(res.message || 'บันทึกข้อมูลสำเร็จ');
    } catch (err) {
      scanClose();
      await scanErrorAlert(friendlyError(err, 'บันทึกไม่สำเร็จ'));
    }
  }

  async function applyPhotoUpload(id, fileOrBlob) {
    if (!id || photoBusy) return;
    setPhotoBusy(true);
    scanLoading('กำลังบันทึกรูป...');
    try {
      await uploadMFGAssemblyPhoto(id, fileOrBlob);
      scanClose();
      toastSuccess('บันทึกรูปถ่ายแล้ว');
      await loadRows();
    } catch (err) {
      scanClose();
      await scanErrorAlert('บันทึกรูปไม่สำเร็จ: ' + (err.message || ''));
    } finally {
      setPhotoBusy(false);
    }
  }
  async function handleRetakePhoto(row) {
    if (!row?.ID || photoBusy) return;
    const photoBlob = await scanPhotoCapture({
      title: row.PhotoURL ? 'ถ่ายรูปใหม่' : 'ถ่ายรูปของที่ประกอบ',
      html: `<div class="scan-popup-hint">MC#: <b>${escapeHtml(row.MachineNo || '-')}</b> / ${escapeHtml(flowComponentLabel(row.Component))}</div>`
    });
    if (!photoBlob) return;
    await applyPhotoUpload(row.ID, photoBlob);
  }
  function handleUploadPhotoClick(row) {
    if (!row?.ID || photoBusy) return;
    pendingPhotoRowIdRef.current = row.ID;
    photoFileInputRef.current?.click();
  }
  async function handleUploadPhotoChange(e) {
    const file = e.target.files?.[0];
    const targetId = pendingPhotoRowIdRef.current;
    e.target.value = '';
    pendingPhotoRowIdRef.current = null;
    if (!file || !targetId) return;
    await applyPhotoUpload(targetId, file);
  }
  async function handleDelete(row) {
    const label = `${row.MachineNo || '#' + row.ID} — ${flowComponentLabel(row.Component)}`;
    const ok = await confirmDelete({
      text: `ลบรายการ ${label}? กู้คืนไม่ได้`
    });
    if (!ok) return;
    try {
      await deleteMFGAssembly(row.ID);
      toastSuccess(`ลบรายการ ${label} แล้ว`);
      await loadRows();
    } catch (err) {
      toastError(err.message || 'ลบไม่สำเร็จ');
    }
  }
  const filtered = useMemo(() => {
    let list = rows.filter(r => (r.Status || '') === 'MATCHED');
    if (periodMode !== 'all') {
      list = list.filter(r => r.CheckDate && inPeriod(r.CheckDate, periodMode, periodAnchor));
    }
    if (partFilter) {
      list = list.filter(r => r.Component === partFilter);
    }
    const term = search.trim().toLowerCase();
    if (term) {
      list = list.filter(r => [r.MachineNo, r.Component, flowComponentLabel(r.Component), r.PartNo, r.SerialNo, r.Country, r.CreatedBy].some(v => String(v || '').toLowerCase().includes(term)));
    }
    return [...list].sort((a, b) => a.ID - b.ID);
  }, [rows, search, partFilter, periodMode, periodAnchor]);

  // รายการที่มีจริงในตาราง — ไม่ฮาร์ดโค้ด เพิ่ม component ใหม่แล้วขึ้นเอง
  const partOptions = useMemo(() => {
    const seen = new Set();
    for (const r of rows) {
      if ((r.Status || '') === 'MATCHED' && r.Component) seen.add(r.Component);
    }
    return [{
      value: '',
      label: 'ทุกรายการ'
    }, ...[...seen].map(v => ({
      value: v,
      label: flowComponentLabel(v) || v
    }))];
  }, [rows]);
  const totalPages = Math.max(1, Math.ceil(filtered.length / pageSize));
  const paged = filtered.slice((page - 1) * pageSize, page * pageSize);
  function goToPage(p) {
    setPage(Math.min(Math.max(1, p), totalPages));
  }
  // .wmx / .wmx-mfg = ขอบเขตของ CSS มือถือเฉพาะหน้า MFG (ดู WhMfgMobile.css)
  return <AppShell navItems={MFG_NAV_ITEMS} roleLabel="MFG">
      <div className="wmx wmx-mfg">
      <div className="wh-heading-row">
        <div>
          <h2 className="wh-title">Matching Assembly</h2>
        </div>
      </div>

      <div className="pc-barcode-grid pc-barcode-grid--single">
        <div className="pc-barcode-card pc-card-mc" role="button" tabIndex={0} title="Scan Kanban" onClick={() => !scanBusy && runKanbanScan()} onKeyDown={e => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          if (!scanBusy) runKanbanScan();
        }
      }}>
          <span className="pc-barcode-kind">Kanban</span>
          <div className="pc-barcode-icon">
            <QrCodeIcon />
          </div>
          <div className="pc-barcode-title">{scanBusy ? 'กำลังตรวจสอบ...' : 'Scan Kanban'}</div>
          <span className="pc-barcode-cta">{scanBusy ? 'กำลังตรวจสอบ...' : 'กดที่นี่เพื่อเริ่มสแกน'}</span>
        </div>
      </div>

      <div className="prp-card">
        <PeriodRangePicker mode={periodMode} onModeChange={handlePeriodModeChange} anchor={periodAnchor} onAnchorChange={setPeriodAnchor} label="ช่วงวันที่ตรวจสอบ (Check Date)" countLabel={`${filtered.length} รายการ`} onClear={clearPeriod} />
      </div>

      {loadError && <p className="form-error" role="alert">
          {loadError}
        </p>}

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
        <div className="mfg-search-actions">
          <div className="wh-part-filter">
            <SelectField value={partFilter} onChange={setPartFilter} options={partOptions} />
          </div>
          <input className="wh-search" type="text" placeholder="ค้นหา MC# / รายการ / P/N / S/N / ผู้ยืนยัน" value={search} onChange={e => setSearch(e.target.value)} />
        </div>
      </div>

      <div className="wh-table-card">
        <table className="wh-table">
          <thead>
            <tr>
              <th>Item</th>
              <th>Date Ass'y</th>
              <th>MC#</th>
              <th>รายการ</th>
              <th>P/N</th>
              <th>S/N</th>
              <th>Check Date</th>
              <th>Check By</th>
              <th>รูปถ่าย</th>
              <th>Status</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {loading && <tr>
                <td colSpan={11} className="wh-empty-cell">
                  กำลังโหลดข้อมูล...
                </td>
              </tr>}
            {!loading && paged.map((a, idx) => {
            const meta = STATUS_META[a.Status] || {
              label: a.Status || DASH,
              cls: 'il-badge il-badge-muted'
            };
            return <tr key={a.ID}>
                    <td className="wh-cell-head" data-label="Item">
                      <strong>{(page - 1) * pageSize + idx + 1}</strong>
                    </td>
                    <td data-label="Date Ass'y">{fmtDate(a.DateAssembly)}</td>
                    <td className="il-mono" data-label="MC#">
                      {a.MachineNo || DASH}
                    </td>
                    <td data-label="รายการ">{flowComponentLabel(a.Component)}</td>
                    <td className="il-mono" data-label="P/N">
                      {a.PartNo || DASH}
                    </td>
                    <td className="il-mono" data-label="S/N">
                      {a.SerialNo || DASH}
                    </td>
                    <td data-label="Check Date">{fmtDate(a.CheckDate)}</td>
                    <td data-label="Check By">{a.CreatedBy || DASH}</td>
                    <td data-label="รูปถ่าย">
                      {a.PhotoURL ? <button type="button" className="wh-photo-thumb" onClick={() => setPhotoView(a.PhotoURL)} title="คลิกเพื่อขยาย">
                          <img src={`${API_BASE_URL}${a.PhotoURL}`} alt="รูปถ่าย" loading="lazy" />
                        </button> : <span className="il-badge il-badge-muted">ไม่มีรูป</span>}
                    </td>
                    <td data-label="Status">
                      <span className={meta.cls}>{meta.label}</span>
                    </td>
                    <td className="wh-cell-action">
                      <button className="tsf-action-btn" onClick={() => setDetailRow(a)}>
                        รายละเอียด
                      </button>
                      <button className="tsf-action-btn tsf-action-btn-warn" onClick={() => setPhotoEditRow(a)} disabled={photoBusy}>
                        แก้ไขรูป
                      </button>
                      {a.Status !== 'MATCHED' && <button className="tsf-action-btn tsf-action-btn-danger" onClick={() => handleDelete(a)}>
                          ลบ
                        </button>}
                    </td>
                  </tr>;
          })}
            {!loading && filtered.length === 0 && <tr>
                <td colSpan={11} className="wh-empty-cell">
                  {rows.length === 0 ? 'ยังไม่มีรายการ' : 'ไม่พบรายการที่ค้นหา'}
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

      {photoEditRow && <div className="wh-modal-overlay" onClick={() => setPhotoEditRow(null)}>
          <div className="wh-modal mfg-photo-modal" onClick={e => e.stopPropagation()}>
            <h3 className="wh-modal-title">{photoEditRow.PhotoURL ? 'แก้ไขรูป' : 'เพิ่มรูป'}</h3>

            <div className="mfg-photo-info">
              <div className="mfg-photo-info-row">
                <span className="mfg-photo-info-label">MC#</span>
                <span className="mfg-photo-info-value">{photoEditRow.MachineNo || DASH}</span>
              </div>
              <div className="mfg-photo-info-row">
                <span className="mfg-photo-info-label">รายการ</span>
                <span className="mfg-photo-info-value">
                  {flowComponentLabel(photoEditRow.Component)}
                </span>
              </div>
            </div>

            <div className="mfg-photo-choices">
              <button type="button" className="mfg-photo-choice" disabled={photoBusy} onClick={async () => {
            const row = photoEditRow;
            setPhotoEditRow(null);
            await handleRetakePhoto(row);
          }}>
                <CameraIcon className="size-5" />
                <span className="mfg-photo-choice-text">
                  <span className="mfg-photo-choice-title">ถ่ายรูปใหม่</span>
                </span>
              </button>
              <button type="button" className="mfg-photo-choice" disabled={photoBusy} onClick={() => {
            const row = photoEditRow;
            setPhotoEditRow(null);
            handleUploadPhotoClick(row);
          }}>
                <ArrowUpTrayIcon className="size-5" />
                <span className="mfg-photo-choice-text">
                  <span className="mfg-photo-choice-title">อัปโหลดรูป</span>
                </span>
              </button>
            </div>

            <div className="wh-modal-actions">
              <button className="wh-modal-cancel" onClick={() => setPhotoEditRow(null)}>
                ยกเลิก
              </button>
            </div>
          </div>
        </div>}

      {photoView && <div className="wh-modal-overlay" onClick={() => setPhotoView(null)}>
          <div className="wh-modal wh-photo-modal" onClick={e => e.stopPropagation()}>
            <h3 className="wh-modal-title">รูปถ่าย</h3>
            <div className="wh-photo-modal-img">
              <img src={`${API_BASE_URL}${photoView}`} alt="รูปถ่าย" />
            </div>
            <div className="wh-modal-actions">
              <button className="wh-modal-cancel" onClick={() => setPhotoView(null)}>
                ปิด
              </button>
            </div>
          </div>
        </div>}

      {detailRow && <div className="wh-modal-overlay" onClick={() => setDetailRow(null)}>
          <div className="wh-modal wh-detail-modal" onClick={e => e.stopPropagation()}>
            <button type="button" className="wh-detail-close" onClick={() => setDetailRow(null)} aria-label="ปิด">
              <XMarkIcon className="size-4" />
            </button>

            <div className="wh-detail-header">
              <span className="wh-detail-header-icon">
                <WrenchScrewdriverIcon className="size-5" />
              </span>
              <div>
                <h3 className="wh-modal-title">รายละเอียดการประกอบ</h3>
                <span className="wh-detail-header-sub">
                  {flowComponentLabel(detailRow.Component)}
                </span>
              </div>
            </div>

            <div className="wh-detail-section">
              <span className="wh-detail-section-title">
                <DocumentTextIcon className="size-4" /> ข้อมูลเครื่อง
              </span>
              <div className="wh-detail-grid">
                <div className="wh-detail-item">
                  <span className="wh-detail-label">MC#</span>
                  <span className="wh-detail-value mono">{detailRow.MachineNo || DASH}</span>
                </div>
                <div className="wh-detail-item">
                  <span className="wh-detail-label">Spec code</span>
                  <span className="wh-detail-value mono">{detailRow.SpecCode || DASH}</span>
                </div>
                <div className="wh-detail-item">
                  <span className="wh-detail-label">IT device</span>
                  <span className="wh-detail-value">{detailRow.ITDevice || DASH}</span>
                </div>
                <div className="wh-detail-item">
                  <span className="wh-detail-label">ลูกค้า</span>
                  <span className="wh-detail-value">{detailRow.Country || DASH}</span>
                </div>
              </div>
            </div>

            <div className="wh-detail-divider" />

            <div className="wh-detail-section">
              <span className="wh-detail-section-title">
                <CubeIcon className="size-4" /> ของที่ประกอบ
              </span>
              <div className="wh-detail-grid">
                <div className="wh-detail-item">
                  <span className="wh-detail-label">รายการ</span>
                  <span className="wh-detail-value">
                    {flowComponentLabel(detailRow.Component)}
                  </span>
                </div>
                <div className="wh-detail-item">
                  <span className="wh-detail-label">P/N</span>
                  <span className="wh-detail-value mono">{detailRow.PartNo || DASH}</span>
                </div>
                <div className="wh-detail-item">
                  <span className="wh-detail-label">S/N</span>
                  <span className="wh-detail-value mono">{detailRow.SerialNo || DASH}</span>
                </div>
                <div className="wh-detail-item">
                  <span className="wh-detail-label">จ่ายโดย (WH)</span>
                  <span className="wh-detail-value">{detailRow.WHCheckedBy || DASH}</span>
                </div>
                <div className="wh-detail-item">
                  <span className="wh-detail-label">Status</span>
                  <span className="wh-detail-value">{detailRow.Status || DASH}</span>
                </div>
                <div className="wh-detail-item">
                  <span className="wh-detail-label">วันที่ประกอบ</span>
                  <span className="wh-detail-value">{fmtDate(detailRow.DateAssembly)}</span>
                </div>
              </div>
            </div>

            {detailRow.PhotoURL && <>
                <div className="wh-detail-divider" />
                <div className="wh-detail-section">
                  <span className="wh-detail-section-title">
                    <CameraIcon className="size-4" /> รูปถ่าย
                  </span>
                  <button type="button" className="wh-photo-thumb" onClick={() => setPhotoView(detailRow.PhotoURL)} title="คลิกเพื่อขยาย">
                    <img src={`${API_BASE_URL}${detailRow.PhotoURL}`} alt="รูปถ่าย" />
                  </button>
                </div>
              </>}

            <div className="wh-detail-meta">
              <span>ยืนยันโดย {detailRow.CreatedBy || DASH}</span>
              <span>{fmtDate(detailRow.CheckDate)}</span>
            </div>

            <div className="wh-modal-actions">
              <button className="wh-modal-cancel" onClick={() => setDetailRow(null)}>
                ปิด
              </button>
            </div>
          </div>
        </div>}

      <input ref={photoFileInputRef} type="file" accept="image/*" style={{
      display: 'none'
    }} onChange={handleUploadPhotoChange} />
      </div>
    </AppShell>;
}