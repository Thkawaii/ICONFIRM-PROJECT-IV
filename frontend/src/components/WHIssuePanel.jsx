import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import SelectField from './Selectfield.jsx';
import PeriodRangePicker from './PeriodRangePicker.jsx';
import { getFlowMachines, getFlowMachine, getFlowIssues, issueFlowPart } from '../api/whFlow.js';
import { scanStep, scanSelect, scanLoading, scanClose, scanSuccessToast, scanErrorAlert, scanConfirmAlert } from '../lib/scanPopup.js';
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
// สติกเกอร์บนกล่อง IT Controller ใส่ P/N กับ S/N ไว้ในบาร์โค้ดเดียวกัน
//
//   ยิงครั้งเดียวได้  "YN22E00849FA 878270007402"
//                      ^^^^^^^^^^^^ P/N   ^^^^^^^^^^^^ S/N
//
// ขั้นสแกน P/N เอาวรรคแรก ขั้นสแกน S/N เอาวรรคที่สอง
// คนหน้างานจึงยิงบาร์โค้ดเดิมซ้ำได้ทั้งสองขั้น ไม่ต้องหาบาร์โค้ดคนละตัว
function scanTokens(v) {
  if (!v) return [];
  return String(v).trim().split(/\s+/).filter(Boolean);
}
function firstToken(v) {
  return scanTokens(v)[0] || '';
}
// วรรคที่สอง ถ้าไม่มีก็ถอยไปใช้วรรคแรก
// เผื่อกรณีที่บาร์โค้ดของ S/N แยกใบมาต่างหาก ซึ่งจะมีค่าเดียว
function serialToken(v) {
  const t = scanTokens(v);
  return t[1] || t[0] || '';
}
// ดึงคู่ค่าที่ชนกันออกจากผลตรวจ มาแสดงเป็นป้ายใต้ข้อความหลัก
//
// backend ส่ง wrong มาเป็นรายการช่องที่ผิด แต่ละช่องมี field / scanned / expected
// เลือกมาเฉพาะกรณีที่ "คนหน้างานมองด้วยตาไม่เห็น" และแก้เองไม่ได้
//
//   ประเทศ           แผนบอกประเทศหนึ่ง ใบอนุญาตบอกอีกประเทศ
//   ของถูกใช้แล้ว     กล่องนี้เคยจ่ายให้เครื่องอื่นไปแล้ว
//
// ส่วน P/N กับ S/N ไม่เอามาแสดง เพราะอ่านจากกล่องตรงหน้าได้เอง
// ถ้าใส่รายละเอียดทุกกรณี คนจะเลิกอ่าน แล้วตอนที่สำคัญจริงจะถูกกดผ่านไป
const WRONG_LABELS = {
  'ประเทศ': ['ใบอนุญาต (LOG)', 'Daily Plan (PCP)'],
  'ของถูกใช้แล้ว': ['เครื่องที่เลือก', 'จ่ายให้เครื่องนี้ไปแล้ว'],
  'P/N': ['S/N', 'นี้ใช้กับ P/N']
};

function mismatchRows(res) {
  const list = Array.isArray(res?.wrong) ? res.wrong : [];
  for (const w of list) {
    const key = String(w?.field || '').trim();
    const labels = WRONG_LABELS[key];
    if (!labels) continue;
    // scanned = ค่าที่เจอตอนสแกน · expected = ค่าที่ควรจะเป็น
    // ประเทศ       scanned คือของใบอนุญาต  expected คือของแผน
    // ของถูกใช้แล้ว scanned คือเครื่องที่เลือก expected คือเครื่องที่ได้ของไป
    // P/N ไม่ตรง — บอกว่า S/N ที่ยิงมาจริง ๆ แล้วคู่กับ P/N ตัวไหน
    //
    //   S/N              878270007402
    //   นี้ใช้กับ P/N     YN22E00849FA
    //
    // ค่าที่ต้องแสดงคือ S/N (จาก serial) กับ P/N ที่ถูกต้อง (จาก expected)
    // ไม่ใช่ P/N ที่ยิงผิดมา เพราะคนเห็นอยู่แล้วว่ายิงอะไรไป
    if (key === 'P/N') {
      const sn = String(w.serial || '').trim();
      const right = String(w.expected || '').trim();
      if (!sn || !right) continue;
      return [{
        label: labels[0],
        value: sn
      }, {
        label: labels[1],
        value: right
      }];
    }

    // ทั้งสองกรณีเรียงเหมือนกัน — scanned ขึ้นก่อน expected ทีหลัง
    //
    //   ประเทศ          scanned = ใบอนุญาต      expected = แผน
    //   ของถูกใช้แล้ว    scanned = เครื่องที่เลือก expected = เครื่องที่ได้ของไป
    const a = String(w.scanned || '').trim();
    const b = String(w.expected || '').trim();
    if (!a && !b) continue;
    return [{
      label: labels[0],
      value: a || '—'
    }, {
      label: labels[1],
      value: b || '—'
    }];
  }
  return undefined;
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
  // หาว่า Part# ที่สแกนมาเป็นของรายการไหนในเครื่องนี้
  //
  // เดิมจับคู่จาก expectedPartNo อย่างเดียว ซึ่งใช้ไม่ได้กับ IT Controller แล้ว
  // เพราะไฟล์ Planning WH ที่เคยกำหนด P/N ไว้ล่วงหน้าเลิกใช้ไปแล้ว
  // ค่านั้นจึงว่าง หาไม่เจอ แล้วส่ง component ว่างไปให้ backend
  // backend หารายการไม่เจอ เลยตอบว่าข้อมูลไม่ถูกต้องทุกครั้ง
  //
  // ตอนนี้หาสองชั้น: ตรงกับค่าที่แผนกำหนดไว้ก่อน ถ้าไม่เจอค่อยดูรายการ
  // ที่ "รอสแกนอยู่และไม่มีค่ากำหนดไว้" ซึ่งก็คือ IT Controller กับ Engine
  function planItemOf(machine, code) {
    const key = normCode(code);
    if (!key) return null;
    const items = machine?.items || [];

    const planned = items.find(it => it.expectedPartNo && normCode(it.expectedPartNo) === key);
    if (planned) return planned;

    // รายการที่ไม่มี P/N กำหนดไว้ — ความถูกต้องไปตรวจที่ backend แทน
    // (IT Controller เทียบกับทะเบียนใบนำเข้า)
    const open = items.filter(it => !it.expectedPartNo && it.status !== 'NO_PLAN' && !it.issued);
    if (open.length === 1) return open[0];

    // มีมากกว่าหนึ่งรายการที่รอสแกน เดาแทนไม่ได้ว่าคนยิงของชิ้นไหน
    // คืน null ให้ผู้เรียกไปถามผู้ใช้ว่าเป็นของชิ้นไหน
    return null;
  }

  // รายการที่รอสแกนและไม่มี P/N กำหนดไว้ — ใช้ตอนต้องถามผู้ใช้ว่ายิงของชิ้นไหน
  function openItemsOf(machine) {
    return (machine?.items || []).filter(it => !it.expectedPartNo && it.status !== 'NO_PLAN' && !it.issued);
  }

  // เลือก MC# แล้วสแกน Part# — ระบบหาเองว่าเป็นของรายการไหน และต้องสแกน S/N ต่อไหม
  async function runQuickScan(machine) {
    if (!machine?.machineNo || busyRef.current) return;
    const planned = (machine.items || []).filter(it => it.status !== 'NO_PLAN');
    if (planned.length === 0) {
      // คนละกรณีกับสแกนไม่ผ่าน — อันนี้เกิดก่อนยิงอะไรเลย
      // บอกสาเหตุไว้ได้ แต่ไม่ใช้ชื่อตารางหรือชื่อไฟล์ที่เลิกใช้แล้ว
      // (ของเดิมเขียนว่า Planning WH ซึ่งถอดออกไปแล้ว ชี้ให้คนไปหาผิดที่)
      await scanErrorAlert('เครื่องนี้ยังไม่มีรายการให้จ่าย', {
        hint: 'ตรวจว่าอัปโหลดข้อมูลของเครื่องนี้ครบแล้วหรือยัง'
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

      let item = planItemOf(machine, partNo);
      if (!item) {
        // มีหลายรายการที่รอสแกนและไม่มี P/N กำหนดไว้ ต้องให้คนบอกว่ายิงของชิ้นไหน
        // ส่ง component ว่างไปไม่ได้ backend จะหารายการไม่เจอแล้วตีเป็นข้อมูลผิด
        const open = openItemsOf(machine);
        if (open.length > 1) {
          const pick = await scanSelect({
            title: 'นี่คือของชิ้นไหน',
            html: mcHint,
            options: open.map(it => ({
              value: it.component,
              label: it.label
            }))
          });
          if (!pick) return;
          item = open.find(it => it.component === pick) || null;
        }
      }
      if (!item) {
        // ข้อความเดียวกับกรณีอื่นที่สแกนไม่ผ่าน ให้หน้างานเห็นแบบเดียวกันหมด
        // ไม่ต้องบอกว่า P/N ไหนหรือให้ไปตรวจอะไร
        await scanErrorAlert('ข้อมูลไม่ถูกต้อง');
        return;
      }
      // CW / CV / SM / MP / Pump HYD — master_data มีแต่ Part# จึงเก็บ S/N ว่างไว้
      // (MFG เป็นคนสแกน S/N# ตอนประกอบ) ไม่เอา Part# ไปใส่ซ้ำในช่อง S/N
      let serialNo = '';
      // ถาม S/N จากธง whNeedsSerial ที่ backend ส่งมา ไม่ใช่จากการที่แผนกำหนด S/N ไว้
      //
      // ของเดิมถามเฉพาะตอนมี expectedSerialNo ซึ่งมาจากไฟล์ Planning WH
      // พอเลิกใช้ไฟล์นั้นแล้ว IT Controller จะไม่มี expectedSerialNo อีกต่อไป
      // หน้าเว็บเลยข้ามการถาม S/N ส่งค่าว่างไป แล้ว backend ปฏิเสธทุกครั้ง
      //
      // whNeedsSerial บอกว่า "ของชิ้นนี้ต้องสแกน S/N" ตามชนิดของ ไม่ขึ้นกับว่า
      // แผนกำหนดค่าไว้หรือไม่ ซึ่งตรงกับสิ่งที่ backend ตรวจจริง
      if (item?.whNeedsSerial || item?.expectedSerialNo) {
        serialNo = serialToken(await scanStep({
          title: `${item.label} — สแกน S/N`,
          html: `${mcHint}<div class="scan-popup-hint">P/N: <b>${escapeHtml(partNo)}</b></div>`,
          confirmText: 'บันทึก',
          // เตือนเฉพาะตอนที่ค่าที่ได้ออกมาเท่ากับ P/N จริง ๆ
          // ยิงบาร์โค้ดรวมซ้ำจะได้วรรคที่สองซึ่งเป็น S/N จึงไม่เข้าเงื่อนไขนี้
          validate: v => serialToken(v) === partNo ? 'ค่า S/N ซ้ำกับ P/N' : undefined
        }));
        if (!serialNo) return;
      }

      // ให้ดูค่าที่ยิงมาก่อนบันทึก
      //
      // เครื่องสแกนอ่านพลาดได้ เช่น P/N ออกมาเป็น "ll" หรือตัวอักษรขาดไปบางตัว
      // ของเดิมยิงเสร็จแล้วบันทึกทันที กว่าจะรู้ว่าผิดก็ต้องไปลบในฐานข้อมูล
      //
      // ขั้นนี้ไม่ได้ตรวจว่าถูกหรือผิด แค่เอาค่ามาวางให้อ่านก่อนกดยืนยัน
      // ใช้ฟอนต์ monospace เพราะตัวอักษรที่สับสนง่าย (I l 1 · O 0) จะแยกออก
      const confirmHtml = `${mcHint}
        <div class="scan-confirm-rows">
          <div class="scan-confirm-row">
            <span class="scan-confirm-key">P/N</span>
            <span class="scan-confirm-val">${escapeHtml(partNo)}</span>
          </div>
          ${serialNo ? `<div class="scan-confirm-row">
            <span class="scan-confirm-key">S/N</span>
            <span class="scan-confirm-val">${escapeHtml(serialNo)}</span>
          </div>` : ''}
        </div>`;
      const ok = await scanConfirmAlert({
        title: `${item.label} — ตรวจก่อนบันทึก`,
        html: confirmHtml,
        confirmText: 'ยืนยัน',
        denyText: null,
        cancelText: 'ยกเลิก'
      });
      if (ok !== 'yes') return;

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
        // ข้อความหลักสั้นเหมือนเดิม แต่ถ้าผิดเรื่องประเทศ ให้โชว์สองค่าที่ชนกัน
        //
        // ไม่โชว์ res.detail ทั้งก้อน เพราะยาวและมีศัพท์เทคนิคปนเยอะ
        // เลือกมาเฉพาะเรื่องประเทศ ซึ่งเป็นกรณีที่คนหน้างานแก้เองไม่ได้
        // ต้องไปแก้ที่ไฟล์ ถ้าไม่บอกจะเสียเวลาหยิบกล่องใหม่มายิงซ้ำเรื่อย ๆ
        //
        // ผิดเรื่อง P/N หรือ S/N ยังขึ้นสั้น ๆ เหมือนเดิม เพราะคนตรวจเองได้จากกล่อง
        await scanErrorAlert(res?.message || 'ข้อมูลไม่ถูกต้อง', {
          rows: mismatchRows(res)
        });
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

  // ตัวเลือกเป็น "MC#, ประเทศ" เพราะเลขเครื่องอย่างเดียวแยกไม่ออกว่าคันไหนไปไหน
  // ประเทศคือสิ่งที่ระบบจะเอาไปเทียบกับใบอนุญาตนำออก คนหน้างานจึงควรเห็นตั้งแต่ตอนเลือก
  //
  // จ่ายครบทุกรายการแล้ว → ขึ้นไอคอนติ๊กถูกท้ายเลขเครื่อง
  const machineOptions = useMemo(() => machines.map(m => {
    const total = m.total ?? 0;
    const issued = m.issued ?? 0;
    const done = total > 0 && issued >= total;

    // ท้ายตัวเลือกบอกสถานะการจ่ายของเครื่องนั้น
    //
    // ของเดิมขึ้นไอคอนเฉพาะตอนจ่าย "ครบทุกชิ้น" เท่านั้น
    // แต่เครื่องหนึ่งมีของได้ถึง 7 ชนิด (ITC · Engine · CW · CV · SM · MP · PH)
    // ไอคอนจึงแทบไม่เคยขึ้น และระหว่างทางก็ไม่รู้เลยว่าเครื่องไหนเริ่มจ่ายไปแล้วบ้าง
    // พอกลับมาทำต่อตอนบ่ายต้องกดเข้าไปดูทีละคัน
    //
    // ตอนนี้บอกสามสถานะ
    //   ยังไม่เริ่ม   ไม่มีอะไรท้ายตัวเลือก
    //   จ่ายไปบ้าง   ตัวเลขความคืบหน้า เช่น 3/7
    //   ครบแล้ว      ไอคอนติ๊กถูก
    //
    // ตัวเลขสำคัญกว่าไอคอนในกรณีที่ยังไม่ครบ เพราะบอกได้ว่าเหลืออีกกี่ชิ้น
    let suffix;
    if (done) {
      suffix = <CheckCircleIcon className="size-4 wh-mc-complete" />;
    } else if (issued > 0) {
      suffix = <span className="wh-mc-partial">{issued}/{total}</span>;
    }
    return {
      value: m.machineNo,
      label: m.customer ? `${m.machineNo}, ${m.customer}` : m.machineNo,
      suffix
    };
  }), [machines]);
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
  // .wmx / .wmx-wh = ขอบเขตของ CSS มือถือเฉพาะหน้า WH (ดู WhMfgMobile.css)
  return <div className="wmx wmx-wh">
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
        <SelectField id="wh-machine-select" searchable searchPlaceholder="พิมพ์เลขเครื่องเพื่อค้นหา..." value={selected?.machineNo || ''} disabled={loading || detailLoading || machineOptions.length === 0} placeholder={machineOptions.length === 0 ? 'ไม่มีเครื่องต้องจ่ายใน 7 วันข้างหน้า' : '— เลือก MC# —'} options={machineOptions} onChange={v => v && openMachine(v, true)} />
      </div>

      {/* สองชีตในไฟล์แผนบอกไม่ตรงกัน — ไม่บล็อกการจ่ายของ แต่ต้องให้เห็น
          เพราะแปลว่าคนแก้แผนไม่ครบ ซึ่งจบลงที่จ่ายของผิดวันหรือผิดประเทศ */}
      {selected?.planConflict && <div className="wh-plan-conflict">
          {selected.planConflict}
        </div>}

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
                {/* ถอดช่องเลขเครื่องฝั่งประกอบออกแล้ว
                    เป็นเลขตัวเดียวกับช่อง S/N ด้านบนเวลายืนยันผ่าน จึงซ้ำซ้อน
                    และถ้าสองค่าไม่ตรงกัน ฝั่งประกอบจะยืนยันไม่ผ่านตั้งแต่แรก
                    บรรทัดสถานะด้านบนจึงบอกได้ครบแล้วว่าตรงหรือไม่ */}
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
    </div>;
}