import { apiFetch } from './client.js';
export function getAdminUsers({
  role = '',
  q = ''
} = {}) {
  const params = new URLSearchParams();
  if (role) params.set('role', role);
  if (q) params.set('q', q);
  const qs = params.toString();
  return apiFetch(`/admin/users${qs ? `?${qs}` : ''}`);
}
export function createAdminUser({
  name,
  username,
  password,
  role_name,
  status = 'Active',
  email = '',
  receive_alerts = false
}) {
  return apiFetch('/admin/users', {
    method: 'POST',
    body: JSON.stringify({
      name,
      username,
      password,
      role_name,
      status,
      email,
      receive_alerts
    })
  });
}
export function updateAdminUser(id, patch) {
  return apiFetch(`/admin/users/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(patch)
  });
}
export function deleteAdminUser(id) {
  return apiFetch(`/admin/users/${id}`, {
    method: 'DELETE'
  });
}
export function getMailRecipients({
  kind = ''
} = {}) {
  const params = new URLSearchParams();
  if (kind) params.set('kind', kind);
  const qs = params.toString();
  return apiFetch(`/admin/mail-recipients${qs ? `?${qs}` : ''}`);
}
export function createMailRecipient({
  email,
  kind = 'TO',
  note = '',
  name = '',
  active = true
}) {
  return apiFetch('/admin/mail-recipients', {
    method: 'POST',
    body: JSON.stringify({
      email,
      kind,
      note,
      name,
      active
    })
  });
}
export function updateMailRecipient(id, patch) {
  return apiFetch(`/admin/mail-recipients/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(patch)
  });
}
export function deleteMailRecipient(id) {
  return apiFetch(`/admin/mail-recipients/${id}`, {
    method: 'DELETE'
  });
}

