import { apiFetch } from './client.js';
export function getQAPartScanSummary() {
  return apiFetch('/qa/part-scan-summary');
}
