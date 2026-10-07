import { apiFetch, API_BASE_URL, getToken } from './client.js';
export function getColumnAliases(scope) {
  const qs = scope ? `?scope=${encodeURIComponent(scope)}` : '';
  return apiFetch(`/format-config/column-alias${qs}`);
}
export function createColumnAlias({
  table,
  new: newValue,
  old,
  note,
  kind
}) {
  return apiFetch('/format-config/column-alias', {
    method: 'POST',
    body: JSON.stringify({
      table,
      new: newValue,
      old,
      note: note || '',
      kind: kind || 'rename'
    })
  });
}
export function deleteColumnAlias(id) {
  return apiFetch(`/format-config/column-alias/${id}`, {
    method: 'DELETE'
  });
}
export function getCodeAliases({
  componentType,
  kind
} = {}) {
  const params = new URLSearchParams();
  if (componentType) params.set('component_type', componentType);
  if (kind) params.set('kind', kind);
  const qs = params.toString();
  return apiFetch(`/format-config/code-alias${qs ? `?${qs}` : ''}`);
}
export function createCodeAlias(payload) {
  return apiFetch('/format-config/code-alias', {
    method: 'POST',
    body: JSON.stringify(payload)
  });
}
export function deleteCodeAlias(id) {
  return apiFetch(`/format-config/code-alias/${id}`, {
    method: 'DELETE'
  });
}
export async function uploadCodeAliases(file, componentType) {
  const token = getToken();
  const formData = new FormData();
  formData.append('file', file);
  if (componentType) formData.append('component_type', componentType);
  const res = await fetch(`${API_BASE_URL}/format-config/code-alias/upload`, {
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
