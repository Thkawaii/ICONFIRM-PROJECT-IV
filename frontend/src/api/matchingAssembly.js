import { apiFetch } from './client.js';
export function getMatchingAssemblies() {
  return apiFetch('/matching-assembly');
}
export function createMatchingAssembly(payload) {
  return apiFetch('/matching-assembly', {
    method: 'POST',
    body: JSON.stringify(payload)
  });
}
export function updateMatchingAssembly(id, payload) {
  return apiFetch(`/matching-assembly/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(payload)
  });
}
export function deleteMatchingAssembly(id) {
  return apiFetch(`/matching-assembly/${id}`, {
    method: 'DELETE'
  });
}
