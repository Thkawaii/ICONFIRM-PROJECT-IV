import { apiFetch, API_BASE_URL, getToken } from './client.js';
import { uploadImportLicense } from './importLicense.js';

// ทะเบียนใบอนุญาต (ชีต "ต่ออายุ")
export function getLicenseRenewals({
  keyword = '',
  status = ''
} = {}) {
  const params = new URLSearchParams();
  if (keyword) params.set('keyword', keyword);
  if (status && status !== 'all') params.set('status', status);
  const qs = params.toString();
  return apiFetch(`/license-renewal${qs ? `?${qs}` : ''}`);
}
export function getLicenseRenewalAlerts() {
  return apiFetch('/license-renewal/alerts');
}

// แก้ / ลบแถวทะเบียนใบอนุญาตจากในระบบ
// เป็นการแก้ชั่วคราว มีผลจนกว่าจะอัปไฟล์รอบถัดไป เพราะไฟล์ Excel คือความจริงของตารางนี้
export function updateLicenseRenewal(id, payload) {
  return apiFetch(`/license-renewal/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(payload)
  });
}
export function deleteLicenseRenewal(id) {
  return apiFetch(`/license-renewal/${id}`, {
    method: 'DELETE'
  });
}
export async function uploadLicenseRenewal(file) {
  const token = getToken();
  const formData = new FormData();
  formData.append('file', file);
  const res = await fetch(`${API_BASE_URL}/license-renewal/upload`, {
    method: 'POST',
    headers: token ? {
      Authorization: `Bearer ${token}`
    } : {},
    body: formData
  });
  const data = await res.json().catch(() => null);
  if (!res.ok) throw new Error(data?.message || `Upload failed (${res.status})`);
  return data;
}

// อัปโหลดไฟล์เดียว → ระบบอ่านให้ทั้งชีตใบอนุญาตนำเข้าและชีตต่ออายุ
// ชีตไหนไม่มีในไฟล์ก็ข้ามไป ไม่ทำให้ทั้งชุดล้มเหลว
//
// ไม่มีส่วนใบอนุญาตนำออกแล้ว ไฟล์ Serial Allocation อัปแยกที่หน้าใบอนุญาตนำออก
const PARTS = [{
  key: 'renewal',
  label: 'ทะเบียนใบอนุญาต (ต่ออายุ)',
  run: uploadLicenseRenewal,
  count: d => d?.imported
}, {
  key: 'import',
  label: 'ใบอนุญาตนำเข้า (Import)',
  run: uploadImportLicense,
  count: d => d?.imported ?? d?.created ?? d?.inserted
}];
// อัปไฟล์รอบเดียว ฝั่งเซิร์ฟเวอร์ลงให้ครบทุกชีตในคำขอเดียว
// ไฟล์ใหญ่จะเร็วขึ้นชัดเจน เพราะเดิมไฟล์เดียวกันถูกส่งขึ้นไปหลายรอบ
export async function uploadLicenseWorkbook(file) {
  try {
    return await uploadWorkbookOnce(file);
  } catch (err) {
    // เซิร์ฟเวอร์รุ่นเก่ายังไม่มี endpoint นี้ — ถอยไปใช้วิธีเดิม ยิงทีละชีต
    if (err?.status === 404) return uploadWorkbookPerSheet(file);
    throw err;
  }
}

async function uploadWorkbookOnce(file) {
  const token = getToken();
  const formData = new FormData();
  formData.append('file', file);
  const res = await fetch(`${API_BASE_URL}/license-upload/workbook`, {
    method: 'POST',
    headers: token ? {
      Authorization: `Bearer ${token}`
    } : {},
    body: formData
  });
  const data = await res.json().catch(() => null);
  if (!res.ok && !Array.isArray(data?.parts)) {
    const err = new Error(data?.message || `Upload failed (${res.status})`);
    err.status = res.status;
    throw err;
  }
  return (data?.parts || []).map(p => ({
    key: p.key,
    label: p.label,
    ok: !!p.ok,
    count: p.count ?? null,
    message: p.message || ''
  }));
}

async function uploadWorkbookPerSheet(file) {
  const results = [];
  for (const part of PARTS) {
    try {
      const data = await part.run(file);
      results.push({
        key: part.key,
        label: part.label,
        ok: true,
        count: part.count(data) ?? null,
        message: data?.message || ''
      });
    } catch (err) {
      results.push({
        key: part.key,
        label: part.label,
        ok: false,
        count: null,
        message: err.message || 'อัปโหลดไม่สำเร็จ'
      });
    }
  }
  return results;
}
