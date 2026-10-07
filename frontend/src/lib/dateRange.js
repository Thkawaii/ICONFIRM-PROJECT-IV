function startOfDay(ref) {
  return new Date(ref.getFullYear(), ref.getMonth(), ref.getDate());
}
function startOfWeek(ref) {
  const d = startOfDay(ref);
  const dow = d.getDay();
  const backToMonday = (dow + 6) % 7;
  d.setDate(d.getDate() - backToMonday);
  return d;
}
export function inDateTab(value, tab, now = new Date()) {
  if (!tab || tab === 'all') return true;
  const d = value instanceof Date ? value : new Date(value);
  if (Number.isNaN(d.getTime())) return false;
  if (tab === 'day') {
    return d.getFullYear() === now.getFullYear() && d.getMonth() === now.getMonth() && d.getDate() === now.getDate();
  }
  if (tab === 'month') {
    return d.getFullYear() === now.getFullYear() && d.getMonth() === now.getMonth();
  }
  if (tab === 'week') {
    const start = startOfWeek(now);
    const end = new Date(start);
    end.setDate(start.getDate() + 7);
    return d >= start && d < end;
  }
  return true;
}
export const DATE_TAB_OPTIONS = [{
  key: 'all',
  label: 'ทั้งหมด'
}, {
  key: 'day',
  label: 'รายวัน'
}, {
  key: 'week',
  label: 'รายสัปดาห์'
}, {
  key: 'month',
  label: 'รายเดือน'
}];
const THAI_MONTHS_FULL = ['มกราคม', 'กุมภาพันธ์', 'มีนาคม', 'เมษายน', 'พฤษภาคม', 'มิถุนายน', 'กรกฎาคม', 'สิงหาคม', 'กันยายน', 'ตุลาคม', 'พฤศจิกายน', 'ธันวาคม'];
const THAI_MONTHS_SHORT = ['ม.ค.', 'ก.พ.', 'มี.ค.', 'เม.ย.', 'พ.ค.', 'มิ.ย.', 'ก.ค.', 'ส.ค.', 'ก.ย.', 'ต.ค.', 'พ.ย.', 'ธ.ค.'];
const pad2 = n => String(n).padStart(2, '0');
const beYear = d => d.getFullYear() + 543;
function startOfMonth(ref) {
  return new Date(ref.getFullYear(), ref.getMonth(), 1);
}
function startOfYear(ref) {
  return new Date(ref.getFullYear(), 0, 1);
}
function toLocalDate(v) {
  if (v instanceof Date) return v;
  if (typeof v === 'string') {
    const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(v.trim());
    if (m) return new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3]));
  }
  return new Date(v);
}
export function resolvePeriodRange(mode, anchor, now = new Date()) {
  if (!mode || mode === 'all') return null;
  const ref = anchor ? toLocalDate(anchor) : now;
  if (Number.isNaN(ref.getTime())) return null;
  if (mode === 'day') {
    const start = startOfDay(ref);
    const end = new Date(start);
    end.setDate(start.getDate() + 1);
    return {
      start,
      end
    };
  }
  if (mode === 'week') {
    const start = startOfWeek(ref);
    const end = new Date(start);
    end.setDate(start.getDate() + 7);
    return {
      start,
      end
    };
  }
  if (mode === 'month') {
    const start = startOfMonth(ref);
    const end = new Date(ref.getFullYear(), ref.getMonth() + 1, 1);
    return {
      start,
      end
    };
  }
  if (mode === 'year') {
    const start = startOfYear(ref);
    const end = new Date(ref.getFullYear() + 1, 0, 1);
    return {
      start,
      end
    };
  }
  return null;
}
export function inPeriod(value, mode, anchor, now = new Date()) {
  const range = resolvePeriodRange(mode, anchor, now);
  if (!range) return true;
  const d = value instanceof Date ? value : new Date(value);
  if (Number.isNaN(d.getTime())) return false;
  return d >= range.start && d < range.end;
}
export function periodRangeLabel(mode, anchor, now = new Date()) {
  const range = resolvePeriodRange(mode, anchor, now);
  if (!range) return 'ทั้งหมด';
  const {
    start,
    end
  } = range;
  const last = new Date(end);
  last.setDate(last.getDate() - 1);
  if (mode === 'day') {
    return `${start.getDate()} ${THAI_MONTHS_FULL[start.getMonth()]} ${beYear(start)}`;
  }
  if (mode === 'month') {
    return `${THAI_MONTHS_FULL[start.getMonth()]} ${beYear(start)}`;
  }
  if (mode === 'year') {
    return `ปี ${beYear(start)}`;
  }
  const sameMonth = start.getMonth() === last.getMonth() && start.getFullYear() === last.getFullYear();
  const sameYear = start.getFullYear() === last.getFullYear();
  if (sameMonth) {
    return `${start.getDate()}–${last.getDate()} ${THAI_MONTHS_FULL[start.getMonth()]} ${beYear(start)}`;
  }
  if (sameYear) {
    return `${start.getDate()} ${THAI_MONTHS_SHORT[start.getMonth()]} – ${last.getDate()} ${THAI_MONTHS_SHORT[last.getMonth()]} ${beYear(start)}`;
  }
  return `${start.getDate()} ${THAI_MONTHS_SHORT[start.getMonth()]} ${beYear(start)} – ${last.getDate()} ${THAI_MONTHS_SHORT[last.getMonth()]} ${beYear(last)}`;
}
export function periodFileTag(mode, anchor, now = new Date()) {
  const range = resolvePeriodRange(mode, anchor, now);
  if (!range) return 'ทั้งหมด';
  const s = range.start;
  const ymd = `${s.getFullYear()}-${pad2(s.getMonth() + 1)}-${pad2(s.getDate())}`;
  if (mode === 'day') return `รายวัน-${ymd}`;
  if (mode === 'week') return `รายสัปดาห์-${ymd}`;
  if (mode === 'month') return `รายเดือน-${s.getFullYear()}-${pad2(s.getMonth() + 1)}`;
  if (mode === 'year') return `รายปี-${s.getFullYear()}`;
  return 'ทั้งหมด';
}
export const PERIOD_MODE_OPTIONS = [{
  key: 'all',
  label: 'ทั้งหมด'
}, {
  key: 'day',
  label: 'รายวัน'
}, {
  key: 'week',
  label: 'รายสัปดาห์'
}, {
  key: 'month',
  label: 'รายเดือน'
}, {
  key: 'year',
  label: 'รายปี'
}];
export const PERIOD_ANCHOR_HINT = {
  day: 'เลือกวันที่ต้องการ',
  week: 'เลือกวันใดก็ได้ในสัปดาห์ที่ต้องการ',
  month: 'เลือกวันใดก็ได้ในเดือนที่ต้องการ',
  year: 'เลือกวันใดก็ได้ในปีที่ต้องการ'
};
const fmtYMD = d => `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`;
export function shiftPeriodAnchor(mode, anchor, delta, now = new Date()) {
  const range = resolvePeriodRange(mode, anchor, now);
  const ref = range ? range.start : anchor ? toLocalDate(anchor) : now;
  let d;
  if (mode === 'day') d = new Date(ref.getFullYear(), ref.getMonth(), ref.getDate() + delta);else if (mode === 'week') d = new Date(ref.getFullYear(), ref.getMonth(), ref.getDate() + delta * 7);else if (mode === 'month') d = new Date(ref.getFullYear(), ref.getMonth() + delta, 1);else if (mode === 'year') d = new Date(ref.getFullYear() + delta, 0, 1);else return anchor;
  return fmtYMD(d);
}
export const PERIOD_STEP_LABEL = {
  day: {
    prev: 'วันก่อนหน้า',
    next: 'วันถัดไป'
  },
  week: {
    prev: 'สัปดาห์ก่อนหน้า',
    next: 'สัปดาห์ถัดไป'
  },
  month: {
    prev: 'เดือนก่อนหน้า',
    next: 'เดือนถัดไป'
  },
  year: {
    prev: 'ปีก่อนหน้า',
    next: 'ปีถัดไป'
  }
};
