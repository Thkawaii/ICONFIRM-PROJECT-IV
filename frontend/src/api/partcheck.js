import { apiFetch } from './client.js';
export function getPartChecks({
  invoiceNo,
  partType
} = {}) {
  const params = new URLSearchParams();
  if (invoiceNo) params.set('invoice_no', invoiceNo);
  if (partType) params.set('part_type', partType);
  const qs = params.toString();
  return apiFetch(`/part-check${qs ? `?${qs}` : ''}`);
}
export function scanPartCheck({
  partType,
  pn,
  sn,
  productionNo = '',
  invoiceNo = ''
}) {
  return apiFetch('/part-check', {
    method: 'POST',
    body: JSON.stringify({
      partType,
      pn,
      sn,
      productionNo,
      invoiceNo
    })
  });
}
export function deletePartCheck(id) {
  return apiFetch(`/part-check/${id}`, {
    method: 'DELETE'
  });
}
