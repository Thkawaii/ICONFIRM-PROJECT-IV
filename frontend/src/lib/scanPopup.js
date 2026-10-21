import Swal from 'sweetalert2';
import 'sweetalert2/dist/sweetalert2.min.css';
import { toastSuccess } from './toast.js';
export async function scanStep({
  title,
  html = '',
  placeholder = 'รอรับสัญญาณจากเครื่องสแกน...',
  confirmText = 'ต่อไป',
  cancelText = 'ยกเลิก',
  validate,
  initialValue = ''
}) {
  const res = await Swal.fire({
    title,
    html,
    input: 'text',
    inputValue: initialValue,
    inputPlaceholder: placeholder,
    inputAutoFocus: true,
    inputAttributes: {
      autocomplete: 'off',
      autocorrect: 'off',
      autocapitalize: 'off',
      spellcheck: 'false'
    },
    customClass: {
      popup: 'scan-popup',
      input: 'scan-popup-input'
    },
    confirmButtonText: confirmText,
    showCancelButton: true,
    cancelButtonText: cancelText,
    allowEnterKey: false,
    inputValidator: v => {
      const val = (v || '').trim();
      if (!val) return 'ยังไม่มีค่าที่สแกน';
      if (validate) return validate(val) || undefined;
      return undefined;
    },
    didOpen: () => {
      const input = Swal.getInput();
      if (!input) return;
      input.focus();
      // มีค่ามาให้แล้ว (เช่น ยิงจากเครื่องสแกนตอนอยู่หน้าหลัก) → เลือกไว้ทั้งหมด
      // ยิงซ้ำอีกครั้งจะได้ทับของเดิม ไม่ใช่ต่อท้ายจนข้อมูลเพี้ยน
      if (input.value) input.select();
      let confirmed = false;
      // auto = ระบบเดาเองว่าสแกนจบแล้ว · ถ้าค่ายังไม่ผ่าน validate ให้รอต่อ อย่าเพิ่งยิง API
      const doConfirm = (auto = false) => {
        if (confirmed) return;
        const val = input.value.trim();
        if (!val) return;
        if (auto && validate && validate(val)) return;
        confirmed = true;
        Swal.clickConfirm();
        setTimeout(() => {
          confirmed = false;
        }, 400);
      };
      input.addEventListener('keydown', e => {
        if (e.key === 'Enter') {
          e.preventDefault();
          doConfirm();
        }
      });
      input.addEventListener('paste', () => {
        setTimeout(() => doConfirm(true), 0);
      });
      // ตรวจว่าสแกนจบแล้วหรือยัง
      //
      // ของเดิม (และรอบที่แก้ไปแล้วรอบหนึ่ง) พยายามแยก "เครื่องสแกน" ออกจาก
      // "คนพิมพ์เอง" ด้วยการนับจังหวะเวลาระหว่างตัวอักษร (ห่างกันไม่เกิน 50ms ฯลฯ)
      // และเช็ค e.isComposing/e.inputType — ยังพลาดกับ CipherLab RS36/RS38 อยู่ดี
      // (จังหวะ/การรายงาน flag ของแต่ละเครื่อง/เฟิร์มแวร์ไม่นิ่งพอจะเดาได้แม่นด้วยวิธีนี้)
      //
      // เปลี่ยนมาใช้หลักการเดียวกับช่องรับสแกนที่พิสูจน์แล้วว่าใช้ได้จริงกับเครื่องนี้
      // (ตัวดักสแกนพื้นหลังในหน้า MFG Assembly): ไม่เดาว่าใครพิมพ์ ไม่สนจังหวะเวลา
      // ใด ๆ ทั้งสิ้น — แค่ "ค่าในช่องเปลี่ยน → รอให้เงียบจริง ๆ (ไม่มีอะไรเข้ามาเพิ่มอีก)
      // แล้วค่อยลองยืนยันให้" ใช้ได้กับทุกวิธีที่อุปกรณ์ส่งค่าเข้ามา ไม่ว่าจะทีละตัว
      // เป็นก้อน หรือผ่าน IME composition แบบไหนก็ตาม
      //
      // ช่องนี้ออกแบบไว้ใช้กับเครื่องสแกนเป็นหลัก (placeholder บอกชัดว่า "รอรับสัญญาณ
      // จากเครื่องสแกน") การพิมพ์มือจึงเป็นทางเลือกสำรอง ไม่ใช่ทางหลัก — ยอมแลกความ
      // เสี่ยงเล็กน้อยที่คนพิมพ์มือแล้วหยุดคิดนานเกิน 400ms จะโดนยืนยันให้ก่อนพิมพ์จบ
      // เพื่อให้เครื่องสแกนทำงานได้แน่นอนกว่าเดิม (validate() ที่ส่งมายังกันไว้อีกชั้น
      // ถ้าค่ายังไม่ผ่านจะไม่ยืนยันให้อัตโนมัติอยู่ดี)
      const QUIET_MS = 400; // เงียบนานเท่านี้ (ไม่มีอะไรเข้ามาเพิ่ม) ถือว่าสแกนจบ
      const MIN_LEN = 3;    // สั้นกว่านี้ยังไม่ลองยืนยันให้เอง กันกดโดนทีละตัวแล้วปิดหน้าทันที

      // กันกรณีคนพิมพ์มือแล้วโดนยืนยันให้กลางคัน
      //
      // กติกา "เงียบ 400ms แล้วยืนยัน" ใช้ได้ดีกับเครื่องสแกน แต่คนพิมพ์มือ
      // หยุดคิดเกิน 400ms เป็นเรื่องปกติ ระบบจึงส่งค่าที่ยังพิมพ์ไม่จบไปตรวจ
      //
      // ห้ามใช้ e.key แยกคนออกจากเครื่อง
      //
      //   บน Android คีย์บอร์ดบนจอส่ง keydown เป็น e.key = "Unidentified"
      //   (keyCode 229) เหมือนกับที่เครื่องสแกนในโหมด wedge ส่งมาเป๊ะ
      //   แยกจากตรงนี้ไม่ได้เลย รอบที่แล้วผมแก้ด้วยวิธีนี้จึงยังเด้งอยู่
      //
      // ดูที่ "ค่าในช่องโตขึ้นยังไง" แทน ซึ่งเป็นจริงทุกอุปกรณ์
      //
      //   คนพิมพ์   โตทีละ 1 ตัว และห่างกันเกิน 250ms
      //   เครื่อง    โตทีละหลายตัว หรือโตทีละตัวแต่รัวจนไม่ถึง 250ms
      //
      // เจอจังหวะแบบคนพิมพ์ติดกัน 2 ครั้ง = ปิดการยืนยันอัตโนมัติของหน้านี้
      // ให้กดปุ่มยืนยันหรือ Enter เอง ซึ่งคนพิมพ์มือทำอยู่แล้ว
      const MANUAL_GAP_MS = 250;  // ช้ากว่านี้และโตทีละตัว ถือว่าเป็นมือคน
      const MANUAL_HITS = 2;      // ต้องเจอติดกันเท่านี้ กันเครื่องสะดุดครั้งเดียว

      let idleTimer = null;
      let manual = false;
      let lastInputAt = 0;
      let lastLen = input.value.length;
      let slowHits = 0;

      const watchTyping = () => {
        const now = Date.now();
        const len = input.value.length;
        const grew = len - lastLen;
        const gap = lastInputAt ? now - lastInputAt : 0;
        lastInputAt = now;
        lastLen = len;

        // ลบตัวอักษรออก = คนกำลังแก้เองแน่นอน
        if (grew < 0) {
          manual = true;
          if (idleTimer) clearTimeout(idleTimer);
          return;
        }
        if (grew === 1 && gap > MANUAL_GAP_MS) {
          slowHits += 1;
          if (slowHits >= MANUAL_HITS) {
            manual = true;
            if (idleTimer) clearTimeout(idleTimer);
          }
        } else if (grew > 0) {
          // โตเป็นก้อน หรือโตทีละตัวแบบรัว = เครื่องสแกน
          slowHits = 0;
        }
      };

      const armIdle = () => {
        watchTyping();
        if (manual) return;
        if (idleTimer) clearTimeout(idleTimer);
        idleTimer = setTimeout(() => {
          if (input.value.trim().length >= MIN_LEN) doConfirm(true);
        }, QUIET_MS);
      };

      input.addEventListener('input', armIdle);
      // compositionend ยิงแน่นอนเสมอเมื่อ IME ยืนยันข้อความ — เผื่อ WebView บางตัว
      // ไม่ยิง "input" ซ้ำอีกครั้งหลัง commit
      input.addEventListener('compositionend', armIdle);

      // กันของค้าง: ถ้ารอบสแกนก่อนหน้าตรวจไม่ทันว่าจบ (ค่ายังไม่ถูกยืนยัน) แล้วเครื่อง
      // เริ่มยัดข้อความรอบใหม่เข้ามา (compositionstart) ให้เคลียร์ของเก่าทิ้งก่อนเสมอ
      // ไม่งั้นข้อมูลรอบใหม่จะไปต่อท้ายของเก่าจนเพี้ยน
      input.addEventListener('compositionstart', () => {
        if (!confirmed && input.value) {
          input.value = '';
        }
      });

    }
  });
  if (res.isConfirmed && res.value) return res.value.trim();
  return null;
}
export async function scanSelect({
  title,
  html = '',
  options
}) {
  const inputOptions = {};
  options.forEach(o => {
    inputOptions[o.value] = o.label;
  });
  const res = await Swal.fire({
    title,
    html,
    input: 'select',
    inputOptions,
    customClass: {
      popup: 'scan-popup'
    },
    inputPlaceholder: 'เลือกชนิดพาร์ท',
    confirmButtonText: 'ต่อไป',
    showCancelButton: true,
    cancelButtonText: 'ยกเลิก'
  });
  return res.isConfirmed ? res.value : null;
}
export function scanLoading(title = 'กำลังบันทึก...') {
  Swal.fire({
    title,
    allowOutsideClick: false,
    allowEscapeKey: false,
    didOpen: () => Swal.showLoading()
  });
}
export function scanClose() {
  Swal.close();
}
export function scanCloseWait() {
  return new Promise(resolve => {
    const popup = Swal.getPopup();
    if (!popup) {
      resolve();
      return;
    }
    let done = false;
    const finish = () => {
      if (done) return;
      done = true;
      resolve();
    };
    popup.addEventListener('animationend', finish, {
      once: true
    });
    popup.addEventListener('transitionend', finish, {
      once: true
    });
    Swal.close();
    setTimeout(finish, 320);
  });
}
export function scanSuccessToast(title) {
  return toastSuccess(title);
}
function escapeHtml(str) {
  return String(str ?? '').replace(/[&<>"']/g, ch => ({
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#39;'
  })[ch]);
}
export function scanErrorAlert(text, {
  hint,
  rows
} = {}) {
  // hint (เช่น "กรุณาติดต่อ ADMIN") ต่อท้ายข้อความเป็นบรรทัดเดียวกัน ไม่มีกล่องสี/ไอคอน
  const msg = String(text || '').trim();
  const full = hint && !msg.includes(hint) ? `${msg} ${hint}` : msg;

  // rows = คู่ค่าที่ไม่ตรงกัน แสดงเป็นป้ายใต้ข้อความหลัก
  //
  //   Daily_Plan(PCP)         Indonesia
  //   ใบอนุญาตส่งออก(LOG)      Malaysia
  //
  // บอกแค่ "ข้อมูลไม่ถูกต้อง" คนหน้างานจะเดาว่าหยิบผิดกล่อง แล้วไปหยิบใหม่มายิงซ้ำ
  // ซึ่งไม่ผ่านอีก เพราะปัญหาอยู่ที่ไฟล์ ไม่ได้อยู่ที่กล่อง
  // เห็นสองค่าวางคู่กันจะรู้ทันทีว่าต้องไปแก้ที่ไหน
  const list = Array.isArray(rows) ? rows.filter(r => r && r.label && r.value) : [];
  const detail = list.length ? `
    <div class="scan-error-rows">
      ${list.map(r => `
        <div class="scan-error-row">
          <span class="scan-error-key">${escapeHtml(r.label)}</span>
          <span class="scan-error-val">${escapeHtml(r.value)}</span>
        </div>`).join('')}
    </div>` : '';

  return Swal.fire({
    icon: 'error',
    title: 'เกิดข้อผิดพลาด',
    html: `<div class="scan-error-text">${escapeHtml(full)}</div>${detail}`,
    confirmButtonText: 'ตกลง'
  });
}
// scanSpecAlert: แสดงผลเทียบ QR กับ Specification sheet เป็นตาราง (ใช้ตอนไม่ตรง / ไม่พบ)
export function scanSpecAlert(result) {
  const r = result || {};
  const items = Array.isArray(r.items) ? r.items : [];
  const rows = items.map(it => `
      <tr class="${it.ok ? 'spec-ok' : 'spec-bad'}">
        <td>${escapeHtml(it.label)}</td>
        <td>${escapeHtml(it.qr || '—')}</td>
        <td>${escapeHtml(it.plan || '(ไม่มีในชีต)')}</td>
        <td class="spec-mark">${it.ok ? '✓' : '✗'}</td>
      </tr>`).join('');
  const table = rows ? `
    <div class="spec-table-wrap">
      <table class="spec-table">
        <thead><tr><th>รายการ</th><th>QR</th><th>Specification sheet</th><th></th></tr></thead>
        <tbody>${rows}</tbody>
      </table>
    </div>` : '';
  // ไม่มีตารางเทียบ (เช่น ไม่พบเครื่องใน Daily Plan) → แสดงแค่หัวข้อ ไม่ต้องมีรายละเอียด
  const head = rows && r.machineNo ? `<div class="scan-popup-hint">Machine No: <b>${escapeHtml(r.machineNo)}</b></div>` : '';
  const detail = '';
  return Swal.fire({
    icon: 'error',
    title: r.message || 'ข้อมูลไม่ตรงกับ Specification sheet',
    html: `${head}${detail}${table}`,
    width: rows ? 720 : undefined,
    confirmButtonText: 'ตกลง'
  });
}
// scanConfirmAlert: ให้ MFG ตัดสินว่าของที่ WH จ่ายมาถูกต้องไหม
// คืน 'yes' = ถูกต้อง, 'no' = ไม่ตรง, null = ปิดหน้าต่าง
// denyText / cancelText = null → ไม่ต้องแสดงปุ่มนั้น
// เช่น ITC / Engine ใช้แค่ "ถูกต้อง / ไม่ตรง" ส่วนพาร์ทที่ระบบเช็คให้แล้วใช้ "ถ่ายรูป / ปิด"
export async function scanConfirmAlert({
  title,
  html = '',
  confirmText = 'ถูกต้อง',
  denyText = 'ไม่ตรง',
  cancelText = 'ปิด'
}) {
  const res = await Swal.fire({
    title,
    html,
    width: 640,
    customClass: {
      popup: 'scan-popup'
    },
    confirmButtonText: confirmText,
    showDenyButton: denyText !== null,
    denyButtonText: denyText || '',
    showCancelButton: cancelText !== null,
    cancelButtonText: cancelText || '',
    allowOutsideClick: false,
    reverseButtons: false
  });
  if (res.isConfirmed) return 'yes';
  if (res.isDenied) return 'no';
  return null;
}
export async function scanPhotoCapture({
  title,
  html = ''
}) {
  let stream = null;
  const res = await Swal.fire({
    title,
    html: `
      ${html}
      <video id="scan-photo-video" autoplay playsinline muted class="scan-photo-video"></video>
      <canvas id="scan-photo-canvas" style="display:none;"></canvas>
    `,
    customClass: {
      popup: 'scan-popup'
    },
    confirmButtonText: 'ถ่ายรูป',
    showCancelButton: true,
    cancelButtonText: 'ปิด',
    allowOutsideClick: false,
    allowEscapeKey: true,
    didOpen: async () => {
      const video = document.getElementById('scan-photo-video');
      try {
        if (!navigator.mediaDevices || !navigator.mediaDevices.getUserMedia) {
          Swal.showValidationMessage('เปิดกล้องไม่ได้: ต้องเข้าเว็บผ่าน https (เช่น https://' + location.host + ') บนมือถือ กล้องถึงจะทำงาน');
          return;
        }
        stream = await navigator.mediaDevices.getUserMedia({
          video: {
            facingMode: 'environment'
          },
          audio: false
        });
        if (video) video.srcObject = stream;
      } catch (err) {
        Swal.showValidationMessage('เปิดกล้องไม่สำเร็จ: ' + (err.message || err));
        Swal.update({
          showCancelButton: true,
          cancelButtonText: 'ปิด'
        });
      }
    },
    preConfirm: () => {
      const video = document.getElementById('scan-photo-video');
      const canvas = document.getElementById('scan-photo-canvas');
      if (!video || !canvas || !video.videoWidth) {
        Swal.showValidationMessage('กล้องยังไม่พร้อม กรุณารอสักครู่แล้วลองใหม่');
        return false;
      }
      const vw = video.videoWidth;
      const vh = video.videoHeight;
      const side = Math.min(vw, vh);
      const sx = (vw - side) / 2;
      const sy = (vh - side) / 2;
      canvas.width = side;
      canvas.height = side;
      canvas.getContext('2d').drawImage(video, sx, sy, side, side, 0, 0, side, side);
      return new Promise(resolve => {
        canvas.toBlob(blob => resolve(blob), 'image/jpeg', 0.9);
      });
    },
    willClose: () => {
      if (stream) stream.getTracks().forEach(t => t.stop());
    }
  });
  if (res.isConfirmed && res.value) return res.value;
  return null;
}