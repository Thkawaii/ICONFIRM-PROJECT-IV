const envBase = import.meta.env.VITE_API_BASE_URL;
export const API_BASE_URL = envBase === undefined || envBase === null ? 'http://localhost:8080' : envBase;
export function getToken() {
  return localStorage.getItem('iconfirm_token');
}
export async function apiFetch(path, options = {}) {
  const token = getToken();
  const headers = {
    'Content-Type': 'application/json',
    ...(options.headers || {})
  };
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }
  const res = await fetch(`${API_BASE_URL}${path}`, {
    ...options,
    headers
  });
  let data = null;
  try {
    data = await res.json();
  } catch {}
  if (!res.ok) {
    const base = data?.message || `Request failed (${res.status})`;
    const error = new Error(base);
    error.detail = data?.detail || '';
    error.status = res.status;
    error.data = data;
    throw error;
  }
  return data;
}
