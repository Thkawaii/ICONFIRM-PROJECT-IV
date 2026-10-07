import { apiFetch, API_BASE_URL, getToken } from './client.js';
export { getMasterData } from './masterData.js';
export function getTsfScans() {
  return apiFetch('/tsf');
}
export function getTsfByMachine(machineNo) {
  return apiFetch(`/tsf/by-machine/${encodeURIComponent(machineNo)}`);
}
export function createTsfScan(payload) {
  return apiFetch('/tsf', {
    method: 'POST',
    body: JSON.stringify(payload)
  });
}
export function updateTsfScan(id, payload) {
  return apiFetch(`/tsf/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(payload)
  });
}
export function getUsers(role) {
  const qs = role ? `?role=${encodeURIComponent(role)}` : '';
  return apiFetch(`/users${qs}`);
}
export async function uploadPhoto(file) {
  const token = getToken();
  const formData = new FormData();
  formData.append('file', file);
  const res = await fetch(`${API_BASE_URL}/uploads`, {
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
export function photoUrl(path) {
  if (!path) return '';
  return path.startsWith('http') ? path : `${API_BASE_URL}${path}`;
}
