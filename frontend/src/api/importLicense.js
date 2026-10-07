import { apiFetch, API_BASE_URL, getToken } from './client.js';
export function getImportLicenseItems({
  licenseNo,
  invoiceNo,
  status,
  code
} = {}) {
  const params = new URLSearchParams();
  if (licenseNo) params.set('license_no', licenseNo);
  if (invoiceNo) params.set('invoice_no', invoiceNo);
  if (status) params.set('status', status);
  if (code) params.set('code', code);
  const qs = params.toString();
  return apiFetch(`/import-license${qs ? `?${qs}` : ''}`);
}
export function getImportLicenseSummary() {
  return apiFetch('/import-license/summary');
}
export function verifyImportLicenseCode({
  code,
  invoiceNo = '',
  productionNo = ''
}) {
  return apiFetch('/import-license/verify', {
    method: 'POST',
    body: JSON.stringify({
      code,
      invoiceNo,
      productionNo
    })
  });
}
export function updateImportLicenseItem(id, patch) {
  return apiFetch(`/import-license/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(patch)
  });
}
export function deleteImportLicenseItem(id) {
  return apiFetch(`/import-license/${id}`, {
    method: 'DELETE'
  });
}
export function renewImportLicense(licenseNo = '', invoiceNo = '', days = 0) {
  return apiFetch('/import-license/renew', {
    method: 'POST',
    body: JSON.stringify({
      licenseNo: licenseNo ?? '',
      invoiceNo: invoiceNo ?? '',
      days
    })
  });
}
export function setImportLicenseComplete({
  ids = null,
  licenseNo = null,
  invoiceNo = null,
  completed = true
} = {}) {
  const body = {
    completed
  };
  if (Array.isArray(ids) && ids.length > 0) {
    body.ids = ids.map(Number);
  } else {
    body.licenseNo = licenseNo ?? '';
    body.invoiceNo = invoiceNo ?? '';
  }
  return apiFetch('/import-license/complete', {
    method: 'POST',
    body: JSON.stringify(body)
  });
}
export function clearImportLicense(licenseNo = '', invoiceNo = '', all = false) {
  const params = new URLSearchParams();
  if (all) {
    params.set('all', 'true');
  } else {
    params.set('license_no', licenseNo ?? '');
    params.set('invoice_no', invoiceNo ?? '');
  }
  return apiFetch(`/import-license?${params.toString()}`, {
    method: 'DELETE'
  });
}
export async function previewImportLicense(file) {
  const token = getToken();
  const formData = new FormData();
  formData.append('file', file);
  const res = await fetch(`${API_BASE_URL}/import-license/preview`, {
    method: 'POST',
    headers: token ? {
      Authorization: `Bearer ${token}`
    } : {},
    body: formData
  });
  const data = await res.json().catch(() => null);
  if (!res.ok) {
    throw new Error(data?.message || `Preview failed (${res.status})`);
  }
  return data;
}
export async function uploadImportLicense(file) {
  const token = getToken();
  const formData = new FormData();
  formData.append('file', file);
  const res = await fetch(`${API_BASE_URL}/import-license/upload`, {
    method: 'POST',
    headers: token ? {
      Authorization: `Bearer ${token}`
    } : {},
    body: formData
  });
  const data = await res.json().catch(() => null);
  if (!res.ok) {
    throw new Error(data?.message || `Upload failed (${res.status})`);
  }
  return data;
}
// ลบหลายรายการในครั้งเดียว — แถวที่สแกนผ่านแล้วจะถูกข้าม (ดู skipped)
export function bulkDeleteImportLicenseItems(ids = []) {
  return apiFetch('/import-license/bulk-delete', {
    method: 'POST',
    body: JSON.stringify({
      ids: ids.map(Number)
    })
  });
}
