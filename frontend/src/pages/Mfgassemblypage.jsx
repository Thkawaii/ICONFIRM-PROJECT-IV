import { useEffect, useMemo, useRef, useState } from 'react';
import { getMFGAssemblies, deleteMFGAssembly, uploadMFGAssemblyPhoto } from '../api/mfgAssembly.js';
import { getFlowMachineBySerial, confirmFlowAssembly, flowComponentLabel } from '../api/whFlow.js';
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
// บาร์โค้ดบนกล่องใส่ P/N กับ S/N ไว้ด้วยกัน คั่นด้วยช่องว่าง
//   "YN22E00849FA 878280001602"
// ขั้นสแกน S/N เอาวรรคที่สอง ถ้ามีค่าเดียวก็ใช้ค่านั้น
function serialToken(v) {
  const t = String(v || '').trim().split(/[\s,;|]+/).filter(Boolean);
  return t[1] || t[0] || '';
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

  // รับค่าจากเครื่องสแกนที่ยิงเข้ามาตอนยังไม่ได้โฟกัสช่องกรอก
  //
  // ของเดิมดักที่ window keydown แล้วสะสมตัวอักษรจาก e.key ทีละตัว
  // ซึ่งใช้ได้กับเครื่องสแกน USB ที่เลียนแบบคีย์บอร์ดจริงเท่านั้น
  //
  // เครื่อง Android (CipherLab RS36/RS38) ยัดข้อความเข้าช่องผ่าน InputConnection
  // keydown ที่ได้จึงเป็น e.key = "Unidentified" (keyCode 229) ความยาวไม่ใช่ 1
  // ตัวสะสมเลยว่างตลอด ยิงบาร์โค้ดแล้วไม่มีอะไรเกิดขึ้น
  //
  // แก้โดยวางช่องรับค่าที่มองไม่เห็นไว้ แล้วโฟกัสมันไว้เสมอ
  // ค่าที่เครื่องยิงมาจะตกลงช่องนี้ไม่ว่าจะส่งมาทางไหน แล้วอ่านจาก event "input"
  // ซึ่งยิงเสมอทั้งคีย์บอร์ดจริง IME ของ Android และการวาง
  useEffect(() => {
    const sink = document.createElement('input');
    // ต้องอยู่ในหน้าจอจริง ไม่ใช่ display:none เพราะช่องที่ซ่อนแบบนั้นโฟกัสไม่ได้
    // และ Android จะไม่ส่งค่าเข้ามา จึงดันออกนอกจอแทน
    sink.setAttribute('aria-hidden', 'true');
    sink.setAttribute('tabindex', '-1');
    sink.setAttribute('autocomplete', 'off');
    sink.setAttribute('autocorrect', 'off');
    sink.setAttribute('autocapitalize', 'off');
    sink.setAttribute('spellcheck', 'false');
    // inputmode none = ไม่เรียกคีย์บอร์ดบนจอขึ้นมาบังหน้าจอ
    sink.setAttribute('inputmode', 'none');
    sink.style.cssText = 'position:fixed;left:-9999px;top:0;width:1px;height:1px;opacity:0;';
    document.body.appendChild(sink);

    let idleTimer = null;
    const flush = () => {
      const code = sink.value.trim();
      sink.value = '';
      if (code.length >= 2 && !busyRef.current) fireRef.current(code);
    };

    const onInput = () => {
      if (idleTimer) clearTimeout(idleTimer);
      // รอให้เงียบก่อน เพราะค่ายาว ๆ ทยอยเข้าเป็นช่วง ไม่ได้มาทีเดียวจบ
      idleTimer = setTimeout(flush, 250);
    };
    const onKeydown = e => {
      if (e.key === 'Enter') {
        if (idleTimer) clearTimeout(idleTimer);
        flush();
      }
    };
    sink.addEventListener('input', onInput);
    sink.addEventListener('keydown', onKeydown);

    // โฟกัสช่องนี้ไว้เมื่อผู้ใช้ไม่ได้กรอกอะไรอยู่
    // ถ้าแย่งโฟกัสตลอดเวลา คนจะพิมพ์ในช่องค้นหาไม่ได้เลย
    const keepFocus = () => {
      if (busyRef.current) return;
      const el = document.activeElement;
      const tag = (el?.tagName || '').toLowerCase();
      if (tag === 'input' || tag === 'textarea' || tag === 'select') return;
      if (el?.closest?.('[data-scan-ignore]')) return;
      sink.focus({
        preventScroll: true
      });
    };
    keepFocus();
    const focusTimer = setInterval(keepFocus, 700);
    document.addEventListener('click', keepFocus);

    return () => {
      clearInterval(focusTimer);
      document.removeEventListener('click', keepFocus);
      if (idleTimer) clearTimeout(idleTimer);
      sink.removeEventListener('input', onInput);
      sink.removeEventListener('keydown', onKeydown);
      sink.remove();
    };
  }, []);

  // ขั้นที่ 1 — Scan Kanban: เทียบ MC# + Product Spec + P/N กับ master_data
  //             แล้วอ่านของที่ WH จ่ายมาให้เครื่องนี้
  // MFG ยิง S/N ของกล่อง แล้วระบบย้อนไปหาว่า WH จ่ายให้เครื่องไหน
  //
  // เปลี่ยนจากเดิมที่ยิง QR บน Kanban เพราะ
  //   กล่องมีบาร์โค้ดติดมาอยู่แล้ว ไม่ต้องพิมพ์ Kanban เพิ่ม
  //   ของที่ต้องยืนยันคือกล่องที่ถืออยู่ ไม่ใช่กระดาษที่แขวนกับเครื่อง
  //
  // บาร์โค้ดบนกล่องยิงได้ "YN22E00849FA 878280001602"
  // ขั้นนี้เอาวรรคหลังซึ่งเป็น S/N (ดู serialToken)
  //
  // ระบบไม่ตัดสินถูกผิดให้ แค่ดึงข้อมูลที่ WH จ่ายมาแสดง
  // แล้วให้คนดูเองว่าตรงกับของในมือไหม
  async function runSerialScan(presetCode = '') {
    if (busyRef.current) return;
    busyRef.current = true;
    setScanBusy(true);
    try {
      let raw = String(presetCode || '').trim();
      if (!raw) {
        raw = await scanStep({
          title: 'สแกน S/N',
          confirmText: 'ตรวจสอบ'
        });
        if (!raw) return;
      }
      const serialNo = serialToken(raw);
      if (!serialNo) return;

      scanLoading('กำลังอ่านของที่ WH จ่ายมา...');
      let res = null;
      try {
        res = await getFlowMachineBySerial(serialNo);
      } catch (err) {
        scanClose();
        await scanErrorAlert(friendlyError(err, 'ยังไม่มีการจ่ายของสำหรับ S/N นี้'));
        return;
      }
      scanClose();

      const machine = res?.machine;
      if (!machine?.machineNo) {
        await scanErrorAlert('ยังไม่มีการจ่ายของสำหรับ S/N นี้');
        return;
      }
      await runItemChecks(machine, serialNo);
    } finally {
      busyRef.current = false;
      setScanBusy(false);
      await loadRows();
    }
  }

  fireRef.current = code => runSerialScan(code);

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
    for (const item of pending) {
      // CW / CV / SM / MP / PH — ระบบเทียบ Part# ให้แล้วตั้งแต่ตอน WH สแกน
      // MFG จึงไม่ต้องตัดสินซ้ำ เหลือแค่ถ่ายรูปของจริงเก็บไว้
      const autoChecked = item.source === 'cw_cv_its';
      const ok = await scanConfirmAlert({
        title: itemTitle(machine, item),
        html: itemHtml(machine, item),
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
  // หัวกล่อง — IT Controller ใช้ชื่อรุ่นอุปกรณ์จาก Planning PCP แทนคำว่า "IT Controller"
  //
  // ค่ามาจากชีต Specification sheet ช่อง IT device เช่น
  //   IT(Satellite, iridium)
  //   IT(Mobile4G, high speed-H)
  //   IT(Mobile4G, normal speed)
  //
  // ค่าพวกนี้มีจุลภาคอยู่ข้างใน แต่ไม่มีปัญหา เพราะอ่านจากเซลล์ในไฟล์
  // ไม่ได้อ่านจาก QR ที่ใช้จุลภาคเป็นตัวคั่น
  //
  // ถ้าแผนไม่ได้ระบุรุ่นมา ใช้ชื่อชิ้นส่วนตามเดิม
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
  

  // ดึงค่า Part# ที่ติดมากับ QR ของ Kanban
  // หมายเหตุ: QR มี P/N ของ Counter Weight ช่องเดียว (ช่องที่ 7 เขียนเป็น "P/N_น้ำหนัก")
  // ส่วน CV / SM / MP / PH ไม่มีอยู่บน Kanban จึงดึงมาแสดงไม่ได้
  

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
  
  // น้ำหนักเดียวกันไหม — แปลงหน่วยให้ก่อนเทียบ (กติกาเดียวกับฝั่ง backend)
  //   6.2T = 6200KG  ← คนละหน่วย แต่น้ำหนักเท่ากัน
  //   6.2T ≠ 6.2KG   ← ต่างกัน 1000 เท่า
  // ยอมคลาดเคลื่อนได้ไม่ถึง 5 กก.
  
  
  function itemHtml(machine, item, kanban = {}) {
    // cell = ช่อง label + ค่า 1 ช่อง · line = 1 บรรทัดเต็ม · pair = 2 ช่องในบรรทัดเดียว
    // สถานะติดไว้ที่ตัวค่าและตัวไอคอนเอง ไม่ใช่ที่แถว
    // เพราะแถวหนึ่งมีสองช่อง (Part# กับ Weight) ที่ผลไม่จำเป็นต้องเหมือนกัน
    // ของเดิมอ่านสถานะจากแถว ✗ ของช่อง Weight จึงถูกระบายเป็นสีเขียวตามช่อง Part#
    const cell = (label, value, mono = true, state = '') => `
        <span class="mfg-check-label">${escapeHtml(label)}</span>
        <span class="mfg-check-value${mono ? ' mono' : ''}${state ? ' mfg-check-value-' + state : ''}">${escapeHtml(value || '—')}${
          state ? `<i class="mfg-check-icon mfg-check-icon-${state}">${state === 'ok' ? '✓' : '✗'}</i>` : ''
        }</span>`;
    const line = (label, value, mono = true, state = '') => `
      <div class="mfg-check-row${state ? ' mfg-check-' + state : ''}">
        ${cell(label, value, mono, state)}
      </div>`;

    // แสดงอย่างเดียว ไม่เทียบ ไม่ย้อมสี
    //
    // เลิกยิง Kanban แล้ว จึงไม่มีค่าจากอีกแหล่งมาเทียบ
    // ระบบเอาของที่ WH จ่ายมาวางให้ดู แล้วคนเช็คกับของในมือเอง
    // ถ้าย้อมสีทั้งที่ไม่ได้เทียบอะไร จะกลายเป็นชี้นำผิด ๆ
    //
    // ป้ายเขียนว่า "(WH จ่าย)" ทุกช่อง เพื่อบอกชัดว่านี่คือค่าที่บันทึกไว้
    // ไม่ใช่ค่าที่อ่านจากของในมือ คนจะได้รู้ว่าต้องเอาไปเทียบเอง
    return `
      <div class="mfg-check-box">
        ${line('MC# (Machine No.)', machine.machineNo)}
        ${line('ลูกค้า / ประเทศ', machine.customer, false)}
        ${line(itemPartLabel(item), item.issuedPartNo)}
        ${line(itemSerialLabel(item), item.issuedSerialNo)}
        ${line('จ่ายโดย', item.issuedBy, false)}
      </div>`;
  }

  // ป้ายของช่อง Part# และ S/N
  //
  // IT Controller ใช้คำว่า IT# กับ S/N# ตามที่หน้างานเรียก
  // ชิ้นอื่นใช้ Part# กับ S/N ตามเดิม
  function itemPartLabel(item) {
    return item.component === 'ITC' ? 'IT# (WH จ่าย)' : 'Part# (WH จ่าย)';
  }
  function itemSerialLabel(item) {
    return item.component === 'ITC' ? 'S/N# (WH จ่าย)' : 'S/N (WH จ่าย)';
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
    // เรียงใหม่สุดขึ้นก่อน ให้ตรงกับหน้า WH
    //
    // ของเดิมเรียงจากเก่าไปใหม่ (a.ID - b.ID) ของที่เพิ่งสแกนจึงไปต่อท้ายสุด
    // พอตารางแบ่งหน้าละ 10 รายการ สแกนครบ 100 ครั้งแล้วรายการล่าสุด
    // จะไปอยู่หน้า 11 คนสแกนเสร็จต้องกดข้ามไปดูว่าบันทึกติดไหม
    //
    // หน้า WH เรียง checked_datetime desc อยู่แล้ว สองหน้าจึงเคยเรียงสวนกัน
    // ทั้งที่เป็นงานต่อเนื่องกัน คนเดียวกันใช้ทั้งสองหน้า
    //
    // ใช้ ID ไม่ใช่เวลาที่บันทึก เพราะสแกนรัวในวินาทีเดียวกันได้
    // แล้วเวลาจะซ้ำจนเรียงไม่คงที่ ส่วน ID เพิ่มทีละหนึ่งเสมอ
    return [...list].sort((a, b) => b.ID - a.ID);
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
        <div className="pc-barcode-card pc-card-mc" role="button" tabIndex={0} title="Scan Here" onClick={() => !scanBusy && runSerialScan()} onKeyDown={e => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          if (!scanBusy) runSerialScan();
        }
      }}>
          {/* ป้ายมุมบอกว่ายิงอะไร — เปลี่ยนจาก Kanban เป็น S/N ตามวิธีทำงานใหม่
              ถ้าปล่อยคำว่า Kanban ไว้ หน้างานจะไปหยิบกระดาษ Kanban มายิง */}
          <span className="pc-barcode-kind">S/N</span>
          <div className="pc-barcode-icon">
            <QrCodeIcon />
          </div>
          <div className="pc-barcode-title">{scanBusy ? 'กำลังตรวจสอบ...' : 'Scan Here'}</div>
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

      {/* กล่องดูรูป
          ต้องมี class wh-photo-overlay เพื่อยก z-index ให้สูงกว่ากล่องอื่น
          เพราะกล่องรายละเอียดการประกอบ render ทีหลังในไฟล์นี้
          ถ้า z-index เท่ากัน ตัวที่มาทีหลังจะทับ รูปเลยไปอยู่ข้างหลัง */}
      {photoView && <div className="wh-modal-overlay wh-photo-overlay" onClick={() => setPhotoView(null)}>
          <div className="wh-photo-frame" onClick={e => e.stopPropagation()}>
            <button className="wh-photo-close" onClick={() => setPhotoView(null)} aria-label="ปิด">
              <XMarkIcon className="size-5" />
            </button>
            <img src={`${API_BASE_URL}${photoView}`} alt="รูปถ่าย" />
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