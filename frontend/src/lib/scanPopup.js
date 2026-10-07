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
      let lastKeyAt = 0;
      let fastKeys = 0;
      let idleTimer = null;
      input.addEventListener('keydown', e => {
        if (e.key === 'Enter') {
          if (idleTimer) clearTimeout(idleTimer);
          return;
        }
        if (e.key.length !== 1) return;
        const now = Date.now();
        fastKeys = now - lastKeyAt <= 50 ? fastKeys + 1 : 0;
        lastKeyAt = now;
        if (idleTimer) clearTimeout(idleTimer);
        if (fastKeys >= 5) {
          // รอให้เงียบ 400 ms ก่อน — บาร์โค้ด Kanban ยาว ของเดิมรอ 200 ms แล้วตัดกลางคัน
          idleTimer = setTimeout(() => doConfirm(true), 400);
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
  hint
} = {}) {
  // hint (เช่น "กรุณาติดต่อ ADMIN") ต่อท้ายข้อความเป็นบรรทัดเดียวกัน ไม่มีกล่องสี/ไอคอน
  const msg = String(text || '').trim();
  const full = hint && !msg.includes(hint) ? `${msg} ${hint}` : msg;
  return Swal.fire({
    icon: 'error',
    title: 'เกิดข้อผิดพลาด',
    html: `<div class="scan-error-text">${escapeHtml(full)}</div>`,
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