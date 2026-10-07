import { apiFetch } from './client.js';
export function getMachineProfile(machineNo) {
  return apiFetch(`/machine-profile/${encodeURIComponent(machineNo)}`);
}
