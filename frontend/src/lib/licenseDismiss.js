const STORE_KEY = 'iconfirm_license_alert_dismissed';
function expiryDay(item) {
  const raw = item?.ExpiryDate;
  if (!raw) return 'no-exp';
  const d = raw instanceof Date ? raw : new Date(raw);
  if (Number.isNaN(d.getTime())) return 'no-exp';
  return d.toISOString().slice(0, 10);
}
export function dismissKey(item) {
  const license = item?.LicenseNo || '—';
  const invoice = item?.InvoiceNo || '—';
  const status = item?.Status || '—';
  return `${license}|${invoice}|${status}|${expiryDay(item)}`;
}
export function readDismissed() {
  try {
    const raw = localStorage.getItem(STORE_KEY);
    if (!raw) return {};
    const obj = JSON.parse(raw);
    return obj && typeof obj === 'object' ? obj : {};
  } catch {
    return {};
  }
}
function writeDismissed(map) {
  try {
    localStorage.setItem(STORE_KEY, JSON.stringify(map));
  } catch {}
}
export function addDismissed(item) {
  const map = readDismissed();
  map[dismissKey(item)] = new Date().toISOString();
  writeDismissed(map);
  return map;
}
export function removeDismissed(item) {
  const map = readDismissed();
  delete map[dismissKey(item)];
  writeDismissed(map);
  return map;
}
export function clearDismissed() {
  writeDismissed({});
  return {};
}
export function pruneDismissed(items = []) {
  const map = readDismissed();
  const live = new Set((items || []).map(dismissKey));
  let changed = false;
  const next = {};
  for (const [k, v] of Object.entries(map)) {
    if (live.has(k)) next[k] = v;else changed = true;
  }
  if (changed) writeDismissed(next);
  return next;
}
