export const LICENSE_VALIDITY_MONTHS = 6;
export const EXPIRY_STATUS = {
  EXPIRED: 'EXPIRED',
  EXPIRING: 'EXPIRING',
  VALID: 'VALID',
  NO_DATE: 'NO_DATE'
};

export const COMPLETED_FILTER = 'COMPLETED';
export const COMPLETED_LABEL = 'เสร็จสิ้นแล้ว';

// ---------------------------------------------------------------------------
// คำในช่องหมายเหตุ / Remark / Note ที่ถือว่า "เสร็จสิ้นแล้ว"
//
// ตัวตัดสินอยู่ที่นี่ที่เดียว ใช้ร่วมกันทั้ง Import และ Export
// และตรงกับ NoteMeansCompleted ฝั่ง backend เพื่อให้หน้าเว็บกับฐานข้อมูลตีความเหมือนกัน
// ---------------------------------------------------------------------------

const NOTE_DONE_WORDS = ['เสร็จแล้ว', 'เสร็จสิ้น', 'เสร็จเรียบร้อย', 'completed', 'complete', 'done', 'finish'];

// คำปฏิเสธ ต้องอยู่ติดกับคำว่า "เสร็จแล้ว" จริง ๆ จึงจะพลิกความหมาย
// (ไม่งั้นคำว่า "ยังไม่" ที่อยู่คนละประโยคจะทำให้สถานะเพี้ยน)
const NEGATE_THAI = 'ไม่';
const NEGATE_PREFIX = ['not', 'non', 'un', 'in'];
const NEGATION_LOOK_BEHIND = 12;

function isNegated(window) {
  if (window.includes(NEGATE_THAI)) return true;
  const trimmed = window.replace(/[ \t\-_./]+$/, '');
  return NEGATE_PREFIX.some(p => trimmed.endsWith(p));
}

function hasUnnegatedWord(s, word) {
  let from = 0;
  for (;;) {
    const idx = s.indexOf(word, from);
    if (idx < 0) return false;
    const start = Math.max(0, idx - NEGATION_LOOK_BEHIND);
    if (!isNegated(s.slice(start, idx))) return true;
    from = idx + word.length;
  }
}

export function noteMeansCompleted(note) {
  const s = String(note || '').trim().toLowerCase();
  if (!s) return false;
  return NOTE_DONE_WORDS.some(w => hasUnnegatedWord(s, w));
}

// isLicenseCompleted: ใช้ค่าที่ backend คำนวณไว้เป็นหลัก
// และอ่านจากช่องหมายเหตุตรง ๆ เป็นทางสำรอง เพื่อให้ข้อมูลเก่าที่อัปโหลดไว้
// ก่อนระบบจะอ่านหมายเหตุ ขึ้นสถานะถูกทันทีโดยไม่ต้องอัปโหลดไฟล์ซ้ำ
export function isLicenseCompleted(row) {
  if (!row) return false;
  if (row.Completed === true || row.completed === true) return true;
  if (row.NoteCompleted === true) return true;
  const note = String(row.Note ?? row.note ?? '').trim() || String(row.Remark ?? row.remark ?? '').trim();
  return noteMeansCompleted(note);
}
function atMidnight(d) {
  return new Date(d.getFullYear(), d.getMonth(), d.getDate());
}
export function computeLicenseExpiry(issueDateRaw, withinDays = 30) {
  if (!issueDateRaw) {
    return {
      hasDate: false,
      issueDate: null,
      expiryDate: null,
      daysLeft: null,
      status: EXPIRY_STATUS.NO_DATE
    };
  }
  const issue = new Date(issueDateRaw);
  if (Number.isNaN(issue.getTime())) {
    return {
      hasDate: false,
      issueDate: null,
      expiryDate: null,
      daysLeft: null,
      status: EXPIRY_STATUS.NO_DATE
    };
  }
  const expiry = new Date(issue);
  expiry.setMonth(expiry.getMonth() + LICENSE_VALIDITY_MONTHS);
  const today = atMidnight(new Date());
  const expDay = atMidnight(expiry);
  const daysLeft = Math.round((expDay - today) / 86400000);
  let status;
  if (daysLeft < 0) status = EXPIRY_STATUS.EXPIRED;else if (daysLeft <= withinDays) status = EXPIRY_STATUS.EXPIRING;else status = EXPIRY_STATUS.VALID;
  return {
    hasDate: true,
    issueDate: issue,
    expiryDate: expDay,
    daysLeft,
    status
  };
}
const TH_MONTHS = ['ม.ค.', 'ก.พ.', 'มี.ค.', 'เม.ย.', 'พ.ค.', 'มิ.ย.', 'ก.ค.', 'ส.ค.', 'ก.ย.', 'ต.ค.', 'พ.ย.', 'ธ.ค.'];
export function formatThaiDate(d) {
  if (!d) return '—';
  const date = d instanceof Date ? d : new Date(d);
  if (Number.isNaN(date.getTime())) return '—';
  return `${date.getDate()} ${TH_MONTHS[date.getMonth()]} ${date.getFullYear()}`;
}
export function daysLeftLabel(daysLeft) {
  if (daysLeft == null) return 'ยังไม่ระบุวันที่';
  if (daysLeft < 0) return `เลยกำหนด ${Math.abs(daysLeft)} วัน`;
  if (daysLeft === 0) return 'หมดอายุวันนี้';
  return `เหลือ ${daysLeft} วัน`;
}
export const STATUS_LABEL = {
  [EXPIRY_STATUS.EXPIRED]: 'หมดอายุแล้ว',
  [EXPIRY_STATUS.EXPIRING]: 'ใกล้หมดอายุ',
  [EXPIRY_STATUS.VALID]: 'ปกติ',
  [EXPIRY_STATUS.NO_DATE]: 'ยังไม่ระบุวันที่',
  [COMPLETED_FILTER]: COMPLETED_LABEL
};
