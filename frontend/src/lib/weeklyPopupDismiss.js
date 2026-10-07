const STORE_KEY = 'iconfirm_license_weekly_popup_shown';
export function isoWeekKey(input) {
  const src = input ? new Date(input) : new Date();
  const d = new Date(Date.UTC(src.getFullYear(), src.getMonth(), src.getDate()));
  const dayNum = d.getUTCDay() || 7;
  d.setUTCDate(d.getUTCDate() + 4 - dayNum);
  const yearStart = new Date(Date.UTC(d.getUTCFullYear(), 0, 1));
  const weekNo = Math.ceil(((d - yearStart) / 86400000 + 1) / 7);
  return `${d.getUTCFullYear()}-W${String(weekNo).padStart(2, '0')}`;
}
function readShownWeek() {
  try {
    return localStorage.getItem(STORE_KEY) || '';
  } catch {
    return '';
  }
}
export function wasShownThisWeek() {
  return readShownWeek() === isoWeekKey();
}
export function markShownThisWeek() {
  try {
    localStorage.setItem(STORE_KEY, isoWeekKey());
  } catch {}
}
export function resetWeeklyPopup() {
  try {
    localStorage.removeItem(STORE_KEY);
  } catch {}
}
