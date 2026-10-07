import { EXPIRY_STATUS, formatThaiDate, noteMeansCompleted } from './licenseExpiry.js';

export const EXPORT_LICENSE_VALIDITY_MONTHS = 1;

// ---------------------------------------------------------------------------
// จำนวนที่ต่ออายุ (Renewal Round)
// ต่ออายุ Export License 1 ครั้ง = เลขใบอนุญาตใหม่ 1 ใบ = 1 แถวใหม่ในระบบ
// แถวของรอบเก่ายังอยู่ครบ (เป็นประวัติ)
// ---------------------------------------------------------------------------

export const RENEWAL_ROUND_ALL = 'all';
export const RENEWAL_ROUND_LATEST = 'latest';
export const RENEWAL_ROUND_LABEL = 'จำนวนที่ต่ออายุ';

export function renewalRoundOf(row) {
  const n = Number(row?.RenewalRound ?? row?.renewal_round ?? 1);
  return Number.isFinite(n) && n >= 1 ? Math.floor(n) : 1;
}

export function renewalRoundLabel(row) {
  const n = renewalRoundOf(row);
  return n <= 1 ? 'ใบแรก' : `ต่ออายุครั้งที่ ${n}`;
}

export function isLatestRound(row) {
  if (row?.IsLatestRound === false) return false;
  return true;
}

// ---------------------------------------------------------------------------
// สถานะ Checklist มาจากช่อง Note / หมายเหตุ ในไฟล์ Excel
// มีคำว่า "เสร็จแล้ว" → Complete (ผู้ใช้ไม่ต้องกดทีละรายการในหน้าเว็บ)
// ---------------------------------------------------------------------------

// ข้อความที่ใช้ตัดสินสถานะ Checklist — ใช้ช่อง Remark ในไฟล์เป็นหลัก
// (backend เก็บสำเนาไว้ที่ Note ด้วย จึงอ่านได้ทั้งสองทาง)
export function noteText(row) {
  const note = String(row?.Note ?? row?.note ?? '').trim();
  if (note) return note;
  return String(row?.Remark ?? row?.remark ?? '').trim();
}

export { noteMeansCompleted };

// isExportChecklistDone: ใช้ค่าที่ backend คำนวณไว้เป็นหลัก
// และเผื่อ fallback อ่านจากช่อง Note ตรง ๆ (กรณีข้อมูลเก่า/หน้าเว็บโหลดก่อน backend อัปเดต)
export function isExportChecklistDone(row) {
  if (!row) return false;
  if (row.NoteCompleted === true) return true;
  if (row.Completed === true || row.completed === true) return true;
  return noteMeansCompleted(noteText(row));
}

export const CHECKLIST_LABEL_DONE = 'เสร็จแล้ว';
export const CHECKLIST_LABEL_OPEN = 'ยังไม่เสร็จ';

export const EXPORT_LICENSE_LEAD_DAYS = 15;

export const EXPORT_LICENSE_LEAD_WARN_DAYS = 7;

export const LEAD_FILTER_DUE_SOON = 'LEAD_DUE_SOON';

export const LEAD_STATUS = {
  OVERDUE: 'LEAD_OVERDUE',
  DUE: 'LEAD_DUE',
  NO_DATE: 'LEAD_NO_DATE'
};

export const LEAD_STATUS_LABEL = {
  [LEAD_STATUS.OVERDUE]: 'เลยกำหนดยื่น',
  [LEAD_STATUS.DUE]: 'ถึงกำหนดยื่น',
  [LEAD_STATUS.NO_DATE]: 'ยังไม่ระบุวันที่'
};

export const LEAD_BADGE_CLASS = {
  [LEAD_STATUS.OVERDUE]: 'il-badge il-badge-bad',
  [LEAD_STATUS.DUE]: 'il-badge il-badge-ok',
  [LEAD_STATUS.NO_DATE]: 'il-badge il-badge-muted'
};

export function leadBadgeClass(info) {
  if (!info || !info.hasDate) return LEAD_BADGE_CLASS[LEAD_STATUS.NO_DATE];
  if (info.leadStatus === LEAD_STATUS.OVERDUE) return LEAD_BADGE_CLASS[LEAD_STATUS.OVERDUE];
  return LEAD_BADGE_CLASS[LEAD_STATUS.DUE];
}

function atMidnight(d) {
  return new Date(d.getFullYear(), d.getMonth(), d.getDate());
}

function toDate(raw) {
  if (!raw) return null;
  const d = raw instanceof Date ? new Date(raw.getTime()) : new Date(raw);
  return Number.isNaN(d.getTime()) ? null : d;
}

export function addMonthsClamped(date, months) {
  const base = toDate(date);
  if (!base) return null;
  const y = base.getFullYear();
  const m = base.getMonth();
  const d = base.getDate();
  const lastDayOfTarget = new Date(y, m + months + 1, 0).getDate();
  return new Date(y, m + months, Math.min(d, lastDayOfTarget));
}

export function addDays(date, days) {
  const base = toDate(date);
  if (!base) return null;
  return new Date(base.getFullYear(), base.getMonth(), base.getDate() + days);
}

export function exportExpiryDate(row) {
  if (!row) return null;
  const issue = toDate(row.IssueDate);
  if (issue) return addMonthsClamped(issue, EXPORT_LICENSE_VALIDITY_MONTHS);
  return toDate(row.ExpireDate);
}

export function exportLeadTimeDate(row) {
  const expiry = exportExpiryDate(row);
  if (!expiry) return null;
  return addDays(expiry, -EXPORT_LICENSE_LEAD_DAYS);
}

const EMPTY = {
  hasDate: false,
  issueDate: null,
  expiryDate: null,
  daysLeft: null,
  status: EXPIRY_STATUS.NO_DATE,
  leadDate: null,
  leadDaysLeft: null,
  leadStatus: LEAD_STATUS.NO_DATE,
  leadUrgent: false,
  leadAlert: false
};

export function computeExportLicenseDates(row, { withinDays = 7, leadWarnDays = EXPORT_LICENSE_LEAD_WARN_DAYS } = {}) {
  const expiry = exportExpiryDate(row);
  if (!expiry) return { ...EMPTY };

  const today = atMidnight(new Date());
  const expDay = atMidnight(expiry);
  const leadDay = addDays(expDay, -EXPORT_LICENSE_LEAD_DAYS);

  const daysLeft = Math.round((expDay - today) / 86400000);
  const leadDaysLeft = Math.round((leadDay - today) / 86400000);

  let status;
  if (daysLeft < 0) status = EXPIRY_STATUS.EXPIRED;
  else if (daysLeft <= withinDays) status = EXPIRY_STATUS.EXPIRING;
  else status = EXPIRY_STATUS.VALID;

  const leadStatus = leadDaysLeft < 0 ? LEAD_STATUS.OVERDUE : LEAD_STATUS.DUE;
  const leadUrgent = leadStatus === LEAD_STATUS.DUE && leadDaysLeft <= leadWarnDays;

  return {
    hasDate: true,
    issueDate: toDate(row?.IssueDate),
    expiryDate: expDay,
    daysLeft,
    status,
    leadDate: leadDay,
    leadDaysLeft,
    leadStatus,
    leadUrgent,
    leadAlert: leadStatus === LEAD_STATUS.OVERDUE || leadUrgent
  };
}

export function leadDaysLabel(leadDaysLeft) {
  if (leadDaysLeft == null) return 'ยังไม่ระบุวันที่';
  if (leadDaysLeft < 0) return `เลยกำหนดยื่นมา ${Math.abs(leadDaysLeft)} วัน`;
  if (leadDaysLeft === 0) return 'ต้องยื่นวันนี้';
  return `ต้องยื่นภายใน ${leadDaysLeft} วัน`;
}

export function leadTimeText(row) {
  const d = exportLeadTimeDate(row);
  return d ? formatThaiDate(d) : '—';
}

// remarkWithoutStatus: ตัดคำบอกสถานะที่อยู่หน้าสุดของ Remark ออก
// ป้ายสถานะแสดงคำว่า "เสร็จแล้ว" ให้อยู่แล้ว จึงไม่ต้องโชว์ซ้ำในข้อความ
// เหลือไว้เฉพาะรายละเอียดอื่นที่ผู้ใช้พิมพ์ต่อท้าย เช่น "เสร็จแล้ว | รอเอกสารตัวจริง" -> "รอเอกสารตัวจริง"
export function remarkWithoutStatus(remark) {
  let text = String(remark || '').trim();
  if (!text) return '';
  const lower = text.toLowerCase();
  const hit = NOTE_DONE_WORDS.find(w => lower.startsWith(w));
  if (!hit) return text;
  text = text.slice(hit.length).replace(/^[\s|·,:;.\-–—]+/, '').trim();
  return text;
}
