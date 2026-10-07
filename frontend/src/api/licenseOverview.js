import { apiFetch, API_BASE_URL, getToken } from './client.js';

// ---------------------------------------------------------------------------
// License Overview — Import + Export ในหน้าเดียว พร้อมประวัติการต่ออายุ
//
// ไฟล์ Import / Export ยังอัปโหลดผ่าน API เดิม (importLicense.js / exportLicense.js)
// ที่นี่เพิ่มเฉพาะไฟล์ Renewal และ API สำหรับอ่านภาพรวม
// ---------------------------------------------------------------------------

export const LICENSE_TYPE = {
  IMPORT: 'IMPORT',
  EXPORT: 'EXPORT'
};
export const LICENSE_TYPE_LABEL = {
  IMPORT: 'ใบนำเข้า',
  EXPORT: 'ใบนำออก'
};

// ค่าพิเศษของตัวกรองประเทศ
export const COUNTRY_ALL = '__all__';
export const COUNTRY_NONE = '__none__';
export const COUNTRY_NONE_LABEL = 'ไม่ระบุประเทศ';

// ใบที่ครอบหลายประเทศในใบเดียว เช่น "INDONESIA , MALAYSIA"
// ใบพวกนี้ยังขึ้นตอนกรองรายประเทศเหมือนเดิม แต่บางทีก็อยากดูเฉพาะใบที่ยังไม่แตกสาย
export const COUNTRY_MULTI = '__multi__';
export const COUNTRY_MULTI_LABEL = 'หลายประเทศ';

// isMultiCountry: ใบนี้ระบุประเทศไว้มากกว่าหนึ่งประเทศหรือไม่
export function isMultiCountry(value) {
  const keys = countryKeys(value);
  return keys.length > 1 && !keys.includes(COUNTRY_NONE);
}

// ชื่อประเทศในไฟล์เขียนไม่เหมือนกัน (ตัวพิมพ์ใหญ่เล็ก เว้นวรรคเกิน)
// จึงต้อง normalize ก่อนจัดกลุ่ม ไม่งั้น INDONESIA กับ Indonesia จะกลายเป็นคนละตัวเลือก
export function countryKey(value) {
  const key = String(value ?? '').trim().replace(/\s+/g, ' ').toUpperCase();
  return key || COUNTRY_NONE;
}

// ใบอนุญาตใบเดียวครอบคลุมได้หลายประเทศ ในไฟล์เขียนรวมกันไว้ เช่น "INDONESIA , MALAYSIA"
// (ในไฟล์จริงมีแบบนี้ถึง 501 แถว) จึงต้องแยกออกเป็นรายประเทศ
// ไม่งั้นตัวกรองจะมีตัวเลือกแปลก ๆ ว่า "Indonesia , Malaysia" และกรอง Malaysia เดี่ยว ๆ ไม่เจอใบเหล่านี้
export function countryKeys(value) {
  const raw = String(value ?? '').trim();
  if (!raw) return [COUNTRY_NONE];
  const parts = raw.split(/[,/;]|\band\b|\u0e41\u0e25\u0e30/i).map(v => countryKey(v)).filter(v => v && v !== COUNTRY_NONE);
  return parts.length ? Array.from(new Set(parts)) : [COUNTRY_NONE];
}

// ป้ายที่แสดงให้ผู้ใช้เห็น — จัดเว้นวรรคหน้าลูกน้ำให้เรียบร้อย
export function countryDisplay(value) {
  const keys = countryKeys(value);
  if (keys.length === 1 && keys[0] === COUNTRY_NONE) return COUNTRY_NONE_LABEL;
  return keys.map(countryLabel).join(', ');
}

export function countryLabel(key) {
  if (key === COUNTRY_NONE) return COUNTRY_NONE_LABEL;
  return key.toLowerCase().replace(/(^|[\s,\-/])(\S)/g, (_, sep, ch) => sep + ch.toUpperCase());
}

// ตัวกรองบนหน้า License Overview (ตรงกับค่าที่ backend รับ)
export const LICENSE_FILTER = {
  ALL: 'ALL',
  IMPORT: 'IMPORT',
  EXPORT: 'EXPORT',
  ACTIVE: 'ACTIVE',
  NEAR_EXPIRY: 'NEAR_EXPIRY',
  EXPIRED: 'EXPIRED',
  COMPLETED: 'COMPLETED'
};
export const LICENSE_FILTER_TABS = [{
  key: LICENSE_FILTER.ALL,
  label: 'ทั้งหมด'
}, {
  key: LICENSE_FILTER.IMPORT,
  label: 'ใบนำเข้า'
}, {
  key: LICENSE_FILTER.EXPORT,
  label: 'ใบนำออก'
}, {
  key: LICENSE_FILTER.ACTIVE,
  label: 'ปกติ'
}, {
  key: LICENSE_FILTER.NEAR_EXPIRY,
  label: 'ใกล้หมดอายุ'
}, {
  key: LICENSE_FILTER.EXPIRED,
  label: 'หมดอายุแล้ว'
}, {
  key: LICENSE_FILTER.COMPLETED,
  label: 'เสร็จสิ้นแล้ว'
}];

// สถานะที่ backend ส่งมา (ใช้ตรรกะเดิมของระบบ)
export const LICENSE_STATUS_LABEL = {
  VALID: 'ปกติ',
  EXPIRING: 'ใกล้หมดอายุ',
  EXPIRED: 'หมดอายุแล้ว',
  COMPLETED: 'เสร็จสิ้นแล้ว',
  RENEWED: 'ต่ออายุแล้ว',
  NO_DATE: 'ไม่มีวันหมดอายุ'
};
export const LICENSE_STATUS_CLASS = {
  VALID: 'il-badge il-badge-ok',
  EXPIRING: 'il-badge il-badge-warn',
  EXPIRED: 'il-badge il-badge-bad',
  COMPLETED: 'il-badge il-badge-ok',
  RENEWED: 'il-badge il-badge-muted',
  NO_DATE: 'il-badge il-badge-muted'
};

// ป้ายสถานะอายุใบอนุญาตแบบภาษาไทย พร้อมจำนวนวัน
// ใกล้หมดอายุ N วัน / เลยกำหนด N วัน / ปกติ N วัน
export function licenseDaysLabel(status, daysLeft) {
  const n = typeof daysLeft === 'number' ? Math.abs(daysLeft) : null;
  switch (status) {
    case 'EXPIRING':
      if (daysLeft === 0) return 'หมดอายุวันนี้';
      return n == null ? 'ใกล้หมดอายุ' : `ใกล้หมดอายุ ${n} วัน`;
    case 'EXPIRED':
      return n == null ? 'เลยกำหนด' : `เลยกำหนด ${n} วัน`;
    case 'VALID':
      return n == null ? 'ปกติ' : `ปกติ ${n} วัน`;
    case 'COMPLETED':
      return 'เสร็จสิ้นแล้ว';
    case 'RENEWED':
      return 'ต่ออายุแล้ว';
    default:
      return 'ยังไม่ระบุวันหมดอายุ';
  }
}

// คำอธิบายใต้ป้าย — บอกว่าจำนวนวันนั้นนับจากอะไร
export function licenseDaysHint(status, daysLeft) {
  if (typeof daysLeft !== 'number') return '';
  if (status === 'EXPIRED') return `หมดอายุไปแล้ว ${Math.abs(daysLeft)} วัน`;
  if (daysLeft === 0) return 'หมดอายุวันนี้';
  return `เหลืออีก ${daysLeft} วันจะหมดอายุ`;
}

// ลบใบอนุญาต 1 รายการ (ลบทั้งโซ่: รายการเครื่อง + ประวัติต่ออายุ + ทะเบียน)
export function deleteLicenseOverviewEntry({
  type,
  licenseNo
}) {
  const params = new URLSearchParams({
    type,
    licenseNo
  });
  return apiFetch(`/license-overview?${params.toString()}`, {
    method: 'DELETE'
  });
}

export function getLicenseOverview({
  filter = '',
  keyword = ''
} = {}) {
  const params = new URLSearchParams();
  if (filter && filter !== LICENSE_FILTER.ALL) params.set('filter', filter);
  if (keyword) params.set('keyword', keyword);
  const qs = params.toString();
  return apiFetch(`/license-overview${qs ? `?${qs}` : ''}`);
}

// รายละเอียดใบอนุญาต 1 ใบ + ประวัติการต่ออายุทั้งโซ่
export function getLicenseDetail({
  type,
  licenseNo
}) {
  const params = new URLSearchParams({
    type,
    licenseNo
  });
  return apiFetch(`/license-overview/detail?${params.toString()}`);
}

// ไฟล์ที่อัปโหลดล่าสุดของทั้ง 3 ประเภท + ประวัติการอัปโหลดย้อนหลัง
export function getLicenseUploadLog() {
  return apiFetch('/license-overview/upload-log');
}

export function getLicenseRenewalHistory({
  type = '',
  licenseNo = ''
} = {}) {
  const params = new URLSearchParams();
  if (type) params.set('type', type);
  if (licenseNo) params.set('licenseNo', licenseNo);
  const qs = params.toString();
  return apiFetch(`/license-renewal-history${qs ? `?${qs}` : ''}`);
}

export function clearLicenseRenewalHistory() {
  return apiFetch('/license-renewal-history', {
    method: 'DELETE'
  });
}

async function postRenewalFile(path, file) {
  const token = getToken();
  const formData = new FormData();
  formData.append('file', file);
  const res = await fetch(`${API_BASE_URL}${path}`, {
    method: 'POST',
    headers: token ? {
      Authorization: `Bearer ${token}`
    } : {},
    body: formData
  });
  const data = await res.json().catch(() => null);
  if (!res.ok) throw new Error(data?.message || `Upload failed (${res.status})`);
  return data;
}

// ตรวจไฟล์ Renewal ก่อนบันทึก — เห็น error ทุกข้อโดยยังไม่แตะฐานข้อมูล
export function previewLicenseRenewalHistory(file) {
  return postRenewalFile('/license-renewal-history/preview', file);
}

// บันทึกประวัติการต่ออายุ (เพิ่ม/ข้ามของซ้ำ ไม่ล้างของเดิม)
export function uploadLicenseRenewalHistory(file) {
  return postRenewalFile('/license-renewal-history/upload', file);
}

// ---------------------------------------------------------------------------
// ล้างข้อมูลทั้งหมด — เลือกได้ว่าเฉพาะฝั่งไหน
// ---------------------------------------------------------------------------

export const CLEAR_SCOPE = {
  IMPORT: 'import',
  EXPORT: 'export',
  RENEWAL: 'renewal',
  ALL: 'all'
};

// ตารางภาพรวมสร้างจาก 3 ไฟล์รวมกัน ใบเดียวอาจมาจากหลายไฟล์พร้อมกัน
// การล้างเฉพาะไฟล์เดียวจึงอาจไม่ทำให้แถวนั้นหายไป — ระบบจะบอกจำนวนที่เหลือหลังล้างแทน
export const CLEAR_SCOPE_OPTIONS = [{
  value: CLEAR_SCOPE.IMPORT,
  label: 'ข้อมูลใบนำเข้า (ไฟล์ Import)',
  countKeys: ['importItems']
}, {
  value: CLEAR_SCOPE.EXPORT,
  label: 'ข้อมูลใบนำออก (ไฟล์ Export)',
  countKeys: ['exportItems']
}, {
  value: CLEAR_SCOPE.RENEWAL,
  label: 'ข้อมูลต่ออายุ (ไฟล์ Renewal)',
  countKeys: ['renewalHistory', 'renewalLedger']
}, {
  value: CLEAR_SCOPE.ALL,
  label: 'ทั้งหมด',
  countKeys: ['importItems', 'exportItems', 'renewalHistory', 'renewalLedger']
}];

// นับแถวดิบที่จะถูกลบของตัวเลือกนั้น
export function clearScopeRowCount(option, sources) {
  if (!option || !sources) return null;
  return option.countKeys.reduce((sum, k) => sum + (Number(sources[k]) || 0), 0);
}

export function clearLicenseOverview(scope) {
  const params = new URLSearchParams({
    scope
  });
  return apiFetch(`/license-overview/all?${params.toString()}`, {
    method: 'DELETE'
  });
}

// กรอกวันที่ออกใบอนุญาตเอง — ใช้กับไฟล์ที่ไม่มีคอลัมน์วันที่
// ระบบจะคำนวณวันหมดอายุให้ทุกเครื่องในใบนั้น (นำเข้า + 6 เดือน)
export function setLicenseIssueDate({
  type,
  licenseNo,
  issueDate
}) {
  return apiFetch('/license-overview/issue-date', {
    method: 'PATCH',
    body: JSON.stringify({
      type,
      licenseNo,
      issueDate
    })
  });
}

// ---------------------------------------------------------------------------
// ตัวกรองแยกเป็น 2 ชั้น: ประเภทใบ และสถานะ
// เลือกร่วมกันได้ เช่น "ใบนำเข้า" + "ใกล้หมดอายุ"
// (ของเดิมเป็น dropdown เดียว เลือกได้อย่างใดอย่างหนึ่งเท่านั้น)
// ---------------------------------------------------------------------------

export const TYPE_FILTER_ALL = 'ALL';
export const STATUS_FILTER_ALL = 'ALL';

export const TYPE_FILTER_OPTIONS = [{
  value: TYPE_FILTER_ALL,
  label: 'ทั้งหมด'
}, {
  value: 'IMPORT',
  label: 'ใบนำเข้า'
}, {
  value: 'EXPORT',
  label: 'ใบนำออก'
}];

export const STATUS_FILTER_OPTIONS = [{
  value: STATUS_FILTER_ALL,
  label: 'ทั้งหมด'
}, {
  value: 'VALID',
  label: 'ปกติ'
}, {
  value: 'EXPIRING',
  label: 'ใกล้หมดอายุ'
}, {
  value: 'EXPIRED',
  label: 'หมดอายุแล้ว'
}, {
  value: 'COMPLETED',
  label: 'เสร็จสิ้นแล้ว'
}, {
  value: 'NO_DATE',
  label: 'ยังไม่ระบุวันหมดอายุ'
}];

export function matchTypeFilter(row, value) {
  return value === TYPE_FILTER_ALL || row.licenseType === value;
}

export function matchStatusFilter(row, value) {
  return value === STATUS_FILTER_ALL || row.status === value;
}
