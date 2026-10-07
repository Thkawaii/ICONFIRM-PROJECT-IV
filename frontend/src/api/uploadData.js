import { apiFetch, API_BASE_URL, getToken } from './client.js';
// ชุดข้อมูลที่เปิดให้อัปโหลดในหน้า "อัพโหลดข้อมูล"
// (Planning / Daily Plan / WH1 / WH2 / Engine / พาร์ทรายชนิด เลิกอัปโหลดแล้ว)
export const DATASETS = [{
  key: 'engine_it_allocation',
  label: 'Planning WH'
}, {
  key: 'spec_sheet',
  label: 'Planning MFG'
}, {
  key: 'cw_cv_its',
  label: 'Master Data'
}];
export function getUploadData(dataset, keyword, page = 1, limit = 100) {
  const params = new URLSearchParams({
    dataset,
    page: String(page),
    limit: String(limit)
  });
  if (keyword) params.set('keyword', keyword);
  return apiFetch(`/upload-data?${params.toString()}`);
}
export async function uploadDataFile(dataset, file) {
  const token = getToken();
  const formData = new FormData();
  formData.append('file', file);
  const res = await fetch(`${API_BASE_URL}/upload-data/upload/${dataset}`, {
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
export async function previewUploadData(dataset, file) {
  const token = getToken();
  const formData = new FormData();
  formData.append('file', file);
  const res = await fetch(`${API_BASE_URL}/upload-data/preview/${dataset}`, {
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
export function deleteUploadDataRow(id) {
  return apiFetch(`/upload-data/${id}`, {
    method: 'DELETE'
  });
}
// ส่งมาเฉพาะช่องที่แก้ ({ "Machine": "..." }) ระบบรวมกับค่าเดิมให้
export function updateUploadDataRow(id, data) {
  return apiFetch(`/upload-data/${id}`, {
    method: 'PATCH',
    body: JSON.stringify({
      data
    })
  });
}
export function clearUploadData(dataset) {
  return apiFetch(`/upload-data?dataset=${encodeURIComponent(dataset)}`, {
    method: 'DELETE'
  });
}
export async function exportUploadData(dataset) {
  const token = getToken();
  const res = await fetch(`${API_BASE_URL}/upload-data/export?dataset=${encodeURIComponent(dataset)}`, {
    headers: token ? {
      Authorization: `Bearer ${token}`
    } : {}
  });
  if (!res.ok) {
    throw new Error(`Export failed (${res.status})`);
  }
  const blob = await res.blob();
  const url = window.URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `${dataset}-export-${Date.now()}.xlsx`;
  document.body.appendChild(a);
  a.click();
  a.remove();
  window.URL.revokeObjectURL(url);
}
