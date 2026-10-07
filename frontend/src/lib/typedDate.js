export const THAI_MONTHS = ['มกราคม', 'กุมภาพันธ์', 'มีนาคม', 'เมษายน', 'พฤษภาคม', 'มิถุนายน', 'กรกฎาคม', 'สิงหาคม', 'กันยายน', 'ตุลาคม', 'พฤศจิกายน', 'ธันวาคม'];
export const THAI_MONTHS_SHORT = ['ม.ค.', 'ก.พ.', 'มี.ค.', 'เม.ย.', 'พ.ค.', 'มิ.ย.', 'ก.ค.', 'ส.ค.', 'ก.ย.', 'ต.ค.', 'พ.ย.', 'ธ.ค.'];

export const CALENDAR_MIN_YMD = '2000-01-01';
export const CALENDAR_MAX_YMD = '2037-12-31';

const pad2 = n => String(n).padStart(2, '0');
export const ymdOf = (y, m, d) => `${y}-${pad2(m + 1)}-${pad2(d)}`;

export function parseYMD(ymd) {
  if (!ymd) return null;
  const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(String(ymd));
  if (!m) return null;
  return {
    y: Number(m[1]),
    m: Number(m[2]) - 1,
    d: Number(m[3])
  };
}

export function formatTypedDate(ymd) {
  const p = parseYMD(ymd);
  if (!p) return '';
  return `${pad2(p.d)}/${pad2(p.m + 1)}/${p.y + 543}`;
}

export function formatShortThaiDate(ymd) {
  const p = parseYMD(ymd);
  if (!p) return '';
  return `${p.d} ${THAI_MONTHS_SHORT[p.m]} ${p.y + 543}`;
}

function isValidDay(y, m, d) {
  if (!Number.isInteger(y) || !Number.isInteger(m) || !Number.isInteger(d)) return false;
  if (m < 0 || m > 11 || d < 1) return false;
  return d <= new Date(y, m + 1, 0).getDate();
}

function normalizeText(text) {
  return String(text ?? '')
    .replace(/[๐-๙]/g, ch => String(ch.charCodeAt(0) - 0x0e50))
    .replace(/\s+/g, ' ')
    .trim();
}

function monthFromName(token) {
  const t = token.replace(/\s+/g, '');
  if (!t) return -1;
  let idx = THAI_MONTHS.findIndex(name => name === t);
  if (idx >= 0) return idx;
  idx = THAI_MONTHS_SHORT.findIndex(name => name === t || name.replace(/\./g, '') === t.replace(/\./g, ''));
  return idx;
}

function yearCandidates(raw) {
  const digits = String(raw);
  const n = Number(digits);
  if (digits.length === 4) return [n >= 2400 ? n - 543 : n];
  if (digits.length === 2) return [2500 + n - 543, 2000 + n];
  return [];
}

function inRange(ymd, min, max) {
  return !(min && ymd < min) && !(max && ymd > max);
}

export function parseTypedDate(text, {
  min,
  max
} = {}) {
  const s = normalizeText(text);
  if (!s) return {
    error: 'empty'
  };
  let d;
  let m;
  let yearRaw;
  let match;
  if (match = /^(\d{4})[-/.](\d{1,2})[-/.](\d{1,2})$/.exec(s)) {
    yearRaw = match[1];
    m = Number(match[2]) - 1;
    d = Number(match[3]);
  } else if (match = /^(\d{1,2})[-/. ](\d{1,2})[-/. ](\d{2}|\d{4})$/.exec(s)) {
    d = Number(match[1]);
    m = Number(match[2]) - 1;
    yearRaw = match[3];
  } else if (match = /^(\d{2})(\d{2})(\d{4})$/.exec(s)) {
    d = Number(match[1]);
    m = Number(match[2]) - 1;
    yearRaw = match[3];
  } else if (match = /^(\d{1,2})\s*([^\d\s]+(?:\s*[^\d\s]+)*)\s*(\d{2}|\d{4})$/.exec(s)) {
    d = Number(match[1]);
    m = monthFromName(match[2]);
    yearRaw = match[3];
  } else {
    return {
      error: 'format'
    };
  }
  const candidates = yearCandidates(yearRaw).filter(y => isValidDay(y, m, d));
  if (candidates.length === 0) return {
    error: 'invalid'
  };
  const ymds = candidates.map(y => ymdOf(y, m, d));
  const fitting = ymds.find(v => inRange(v, min, max));
  return {
    ymd: fitting || ymds[0]
  };
}
