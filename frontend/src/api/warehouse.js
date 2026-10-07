import { apiFetch, API_BASE_URL, getToken } from './client.js';
export function getWarehouseStock() {
  return apiFetch('/warehouse');
}
export function issueWarehouseStock(id, {
  qty,
  scannedPartNo,
  scannedSerial,
  remark = ''
}) {
  return apiFetch(`/warehouse/${id}/issue`, {
    method: 'POST',
    body: JSON.stringify({
      qty,
      scanned_part_no: scannedPartNo,
      scanned_serial: scannedSerial,
      remark
    })
  });
}
export function getWhConfirms(status) {
  const qs = status ? `?status=${encodeURIComponent(status)}` : '';
  return apiFetch(`/wh-confirm${qs}`);
}
export function receiveWhTransfer(id) {
  return apiFetch(`/wh-confirm/${id}/receive`, {
    method: 'POST'
  });
}
export async function uploadWarehouseStock(file) {
  const token = getToken();
  const formData = new FormData();
  formData.append('file', file);
  const res = await fetch(`${API_BASE_URL}/warehouse/upload`, {
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
