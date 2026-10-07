import { apiFetch, API_BASE_URL, getToken } from './client.js';
export function getMFGAssemblies() {
  return apiFetch('/mfg-assembly');
}
export function scanMFGAssembly({
  machineNo,
  itControllerNo,
  serialNo,
  partNo,
  partType,
  qrCode
}) {
  return apiFetch('/mfg-assembly/scan', {
    method: 'POST',
    body: JSON.stringify({
      machineNo,
      serialNo: serialNo || itControllerNo,
      itControllerNo: itControllerNo || '',
      partNo: partNo || '',
      partType: partType || '',
      qrCode: qrCode || ''
    })
  });
}

// ตรวจ QR บน Specification sheet กับตาราง Planning (ยังไม่บันทึก)
export function checkMFGSpecQR(qrCode) {
  return apiFetch('/mfg-assembly/spec-check', {
    method: 'POST',
    body: JSON.stringify({
      qrCode
    })
  });
}

// QR บน Specification sheet: "Machine No,Spec code,ลูกค้า,..." (มีเครื่องหมาย , หลายตัว)
export function looksLikeSpecQR(value) {
  return (String(value || '').match(/,/g) || []).length >= 5;
}

export function specQRMachineNo(value) {
  return String(value || '').split(',')[0].trim().toUpperCase();
}
export function createMFGAssembly(payload) {
  return apiFetch('/mfg-assembly', {
    method: 'POST',
    body: JSON.stringify(payload)
  });
}
export function updateMFGAssembly(id, payload) {
  return apiFetch(`/mfg-assembly/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(payload)
  });
}
export function deleteMFGAssembly(id) {
  return apiFetch(`/mfg-assembly/${id}`, {
    method: 'DELETE'
  });
}
export async function uploadMFGAssemblyPhoto(id, fileOrBlob) {
  const token = getToken();
  const formData = new FormData();
  formData.append('file', fileOrBlob, 'mfg-photo.jpg');
  const res = await fetch(`${API_BASE_URL}/mfg-assembly/${id}/photo`, {
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
