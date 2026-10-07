import Swal from 'sweetalert2';
import 'sweetalert2/dist/sweetalert2.min.css';
export const Toast = Swal.mixin({
  toast: true,
  position: 'top-end',
  showConfirmButton: false,
  timer: 3000,
  timerProgressBar: true,
  didOpen: toast => {
    toast.onmouseenter = Swal.stopTimer;
    toast.onmouseleave = Swal.resumeTimer;
  }
});
export function toastSuccess(title) {
  return Toast.fire({
    icon: 'success',
    title
  });
}
export function toastError(title) {
  return Toast.fire({
    icon: 'error',
    title
  });
}
export async function confirmDelete({
  title = 'ยืนยันการลบ',
  text,
  confirmText = 'ลบ'
}) {
  const res = await Swal.fire({
    icon: 'warning',
    title,
    text,
    showCancelButton: true,
    focusCancel: true,
    confirmButtonText: confirmText,
    cancelButtonText: 'ยกเลิก',
    customClass: {
      confirmButton: 'swal2-confirm-danger'
    }
  });
  return res.isConfirmed;
}
export async function confirmComplete({
  title = 'ยืนยันการทำเครื่องหมายเสร็จสิ้น',
  html = '',
  confirmText = 'ตกลง',
  danger = false
} = {}) {
  const res = await Swal.fire({
    icon: danger ? 'warning' : 'question',
    title,
    html,
    showCancelButton: true,
    confirmButtonText: confirmText,
    cancelButtonText: 'ยกเลิก',
    customClass: {
      confirmButton: danger ? 'swal2-confirm-danger' : 'swal2-confirm-complete'
    }
  });
  return res.isConfirmed;
}
export async function promptRenewDays({
  title = 'ต่ออายุใบอนุญาต',
  html = '',
  defaultDays = 180
} = {}) {
  const res = await Swal.fire({
    title,
    html,
    input: 'number',
    inputValue: String(defaultDays),
    inputLabel: 'จำนวนวันที่ต่อ',
    inputPlaceholder: 'เช่น 180',
    inputAttributes: {
      min: '1',
      max: '3650',
      step: '1'
    },
    showCancelButton: true,
    confirmButtonText: 'ต่ออายุ',
    cancelButtonText: 'ยกเลิก',
    inputValidator: v => {
      const n = Number(v);
      if (!v || !Number.isFinite(n) || n <= 0) return 'กรุณากรอกจำนวนวันมากกว่า 0';
      if (n > 3650) return 'จำนวนวันมากเกินไป (สูงสุด 3650 วัน)';
      if (!Number.isInteger(n)) return 'กรุณากรอกจำนวนวันเป็นจำนวนเต็ม';
      return undefined;
    }
  });
  if (res.isConfirmed && res.value) return Number(res.value);
  return null;
}
export async function promptRenewExport({
  licenseOptions = [],
  defaultDays = 180
} = {}) {
  const opts = licenseOptions.map(v => `<option value="${String(v).replace(/"/g, '&quot;')}">${v}</option>`).join('');
  const res = await Swal.fire({
    title: 'ต่ออายุใบอนุญาตส่งออก',
    html: `
      <div class="scan-popup-hint">เลือกเลขใบอนุญาตส่งออกที่จะต่ออายุ</div>
      <select id="ren-lic" class="swal2-input">${opts}</select>
      <input id="ren-days" type="number" class="swal2-input" value="${defaultDays}"
             min="1" max="3650" step="1" placeholder="จำนวนวันที่ต่อ เช่น 180" />
    `,
    showCancelButton: true,
    confirmButtonText: 'ต่ออายุ',
    cancelButtonText: 'ยกเลิก',
    preConfirm: () => {
      const lic = document.getElementById('ren-lic')?.value || '';
      const days = Number(document.getElementById('ren-days')?.value);
      if (!lic) return Swal.showValidationMessage('กรุณาเลือกเลขใบอนุญาต');
      if (!days || !Number.isInteger(days) || days <= 0) return Swal.showValidationMessage('กรุณากรอกจำนวนวันเป็นจำนวนเต็มมากกว่า 0');
      if (days > 3650) return Swal.showValidationMessage('จำนวนวันมากเกินไป (สูงสุด 3650 วัน)');
      return {
        licenseNo: lic,
        days
      };
    }
  });
  if (res.isConfirmed && res.value) return res.value;
  return null;
}
