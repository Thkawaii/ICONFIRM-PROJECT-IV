import { apiFetch } from './client.js';

export function getMachinePlans() {
  return apiFetch('/machine-plans');
}

const norm = v => String(v || '').trim().toUpperCase();

export function indexMachinePlans(rows) {
  const byPair = {};
  const byMachine = {};
  const byITC = {};

  for (const r of rows || []) {
    const machine = norm(r.machineNo);
    const itc = norm(r.itControllerNo);
    if (machine && itc) byPair[`${machine}|${itc}`] = r;
    if (machine && !byMachine[machine]) byMachine[machine] = r;
    if (itc && !byITC[itc]) byITC[itc] = r;
  }

  return { byPair, byMachine, byITC };
}

export function lookupMachinePlan(index, machineNo, itControllerNo) {
  if (!index) return null;
  const machine = norm(machineNo);
  const itc = norm(itControllerNo);
  return (
    (machine && itc ? index.byPair[`${machine}|${itc}`] : null) ||
    (machine ? index.byMachine[machine] : null) ||
    (itc ? index.byITC[itc] : null) ||
    (machine ? index.byITC[machine] : null) ||
    null
  );
}
