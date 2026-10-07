import { apiFetch } from './client.js';
export function getQAConfirmedTable() {
  return apiFetch('/qa/confirmed');
}

// รายการสแกนของ WH (QA อ่านอย่างเดียว)
export function getQAWHRows() {
  return apiFetch('/qa/wh');
}

// รายการประกอบของ MFG (QA อ่านอย่างเดียว)
export function getQAMFGRows() {
  return apiFetch('/qa/mfg');
}
