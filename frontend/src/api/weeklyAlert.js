import { apiFetch } from './client.js';

export function getWeeklyAlertStatus() {
  return apiFetch('/weekly-alert/status');
}

export function getWeeklyAlertPreview() {
  return apiFetch('/weekly-alert/preview');
}

export function sendWeeklyAlertNow(to = '') {
  return apiFetch('/weekly-alert/send', {
    method: 'POST',
    body: JSON.stringify({ to })
  });
}
