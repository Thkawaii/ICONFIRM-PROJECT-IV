import { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { CALENDAR_MAX_YMD, CALENDAR_MIN_YMD, THAI_MONTHS, THAI_MONTHS_SHORT, formatShortThaiDate, formatTypedDate, parseTypedDate, parseYMD, ymdOf } from '../lib/typedDate.js';
import './DatePickerField.css';

const THAI_WEEKDAYS = ['อา', 'จ', 'อ', 'พ', 'พฤ', 'ศ', 'ส'];
const pad2 = n => String(n).padStart(2, '0');
const toYMD = d => `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`;
const monthKey = (y, m) => y * 12 + m;
function CalendarIcon() {
  return <svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <rect x="3" y="4.5" width="18" height="16" rx="2.5" stroke="currentColor" strokeWidth="1.6" />
      <path d="M3 9h18M8 3v3M16 3v3" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
    </svg>;
}
function Chevron({
  dir
}) {
  const d = dir === 'left' ? 'M15 6l-6 6 6 6' : dir === 'right' ? 'M9 6l6 6-6 6' : 'M6 9l6 6 6-6';
  return <svg width={dir === 'down' ? 14 : 18} height={dir === 'down' ? 14 : 18} viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path d={d} stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
    </svg>;
}
export default function DatePickerField({
  value,
  onChange,
  min: minProp,
  max: maxProp,
  placeholder = 'เลือกวันที่',
  renderTrigger
}) {
  const min = minProp && minProp > CALENDAR_MIN_YMD ? minProp : CALENDAR_MIN_YMD;
  const max = maxProp && maxProp < CALENDAR_MAX_YMD ? maxProp : CALENDAR_MAX_YMD;
  const [open, setOpen] = useState(false);
  const [panel, setPanel] = useState('days');
  const [editing, setEditing] = useState(false);
  const [text, setText] = useState('');
  const [error, setError] = useState('');
  const boxRef = useRef(null);
  const inputRef = useRef(null);
  const yearListRef = useRef(null);
  const popRef = useRef(null);
  const [placement, setPlacement] = useState(null);
  const [errorPos, setErrorPos] = useState(null);
  const textRef = useRef('');
  const pointerTypeRef = useRef('mouse');
  const errorTimerRef = useRef(null);
  const selected = parseYMD(value);
  const lastValueRef = useRef(value);
  lastValueRef.current = value;
  const todayYMD = toYMD(new Date());
  function clampYMD(ymd) {
    if (min && ymd < min) return min;
    if (max && ymd > max) return max;
    return ymd;
  }
  const [view, setView] = useState(() => {
    const base = selected || parseYMD(clampYMD(todayYMD));
    return {
      y: base.y,
      m: base.m
    };
  });
  useEffect(() => {
    if (!open) return;
    setPanel('days');
    if (selected) setView({
      y: selected.y,
      m: selected.m
    });
  }, [open]);
  useEffect(() => {
    if (!open) return;
    function onOutside(e) {
      if (boxRef.current?.contains(e.target) || popRef.current?.contains(e.target)) return;
      setOpen(false);
    }
    function onKey(e) {
      if (e.key === 'Escape') setOpen(false);
    }
    document.addEventListener('mousedown', onOutside);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onOutside);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);
  useEffect(() => () => clearTimeout(errorTimerRef.current), []);
  useLayoutEffect(() => {
    if (!open) {
      setPlacement(null);
      return undefined;
    }
    const MARGIN = 8;
    const GAP = 6;
    let frame = 0;
    function place() {
      const pop = popRef.current;
      const box = boxRef.current;
      if (!pop || !box) return;
      const vv = window.visualViewport;
      const vTop = vv ? vv.offsetTop : 0;
      const vLeft = vv ? vv.offsetLeft : 0;
      const vH = vv ? vv.height : window.innerHeight;
      const vW = vv ? vv.width : window.innerWidth;
      const r = box.getBoundingClientRect();
      if (r.bottom < vTop || r.top > vTop + vH) {
        setOpen(false);
        return;
      }
      pop.style.removeProperty('--dpf-day-h');
      const h = pop.scrollHeight + 2;
      const w = pop.offsetWidth;
      const below = vTop + vH - r.bottom - GAP - MARGIN;
      const above = r.top - vTop - GAP - MARGIN;
      const usable = vH - MARGIN * 2;
      const WEEK_ROWS = 6;
      const coarse = window.matchMedia?.('(pointer: coarse)').matches;
      const minDay = coarse ? 32 : 28;
      const baseDay = parseFloat(getComputedStyle(pop).getPropertyValue('--dpf-day-base')) || 38;
      function shrinkFor(space) {
        if (panel !== 'days' || space <= 0) return null;
        const perRow = Math.ceil((h - space) / WEEK_ROWS);
        const day = baseDay - perRow;
        return day >= minDay ? day : null;
      }
      let top;
      let maxH = null;
      let dayH = null;
      let sideLeft = null;
      if (h <= below) {
        top = r.bottom + GAP;
      } else if (h <= above) {
        top = r.top - GAP - h;
      } else if ((dayH = shrinkFor(Math.max(below, above))) !== null) {
        const shrunk = h - (baseDay - dayH) * WEEK_ROWS;
        top = below >= above ? r.bottom + GAP : r.top - GAP - shrunk;
      } else if (h <= usable && (r.right + GAP + w <= vLeft + vW - MARGIN || r.left - GAP - w >= vLeft + MARGIN)) {
        top = Math.min(Math.max(r.top, vTop + MARGIN), vTop + vH - MARGIN - h);
        sideLeft = r.right + GAP + w <= vLeft + vW - MARGIN ? r.right + GAP : r.left - GAP - w;
      } else if (h <= usable) {
        top = below >= above ? vTop + vH - MARGIN - h : vTop + MARGIN;
      } else {
        top = vTop + MARGIN;
        maxH = usable;
      }
      if (dayH !== null) pop.style.setProperty('--dpf-day-h', `${dayH}px`);
      let left = sideLeft !== null ? sideLeft : r.left;
      left = Math.min(left, vLeft + vW - MARGIN - w);
      left = Math.max(left, vLeft + MARGIN);
      setPlacement(prev => prev && prev.top === Math.round(top) && prev.left === Math.round(left) && prev.maxH === maxH ? prev : {
        top: Math.round(top),
        left: Math.round(left),
        maxH
      });
    }
    function schedule(e) {
      if (e && e.type === 'scroll' && popRef.current && e.target instanceof Node && popRef.current.contains(e.target)) return;
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(place);
    }
    place();
    window.addEventListener('resize', schedule);
    window.addEventListener('scroll', schedule, true);
    window.visualViewport?.addEventListener('resize', schedule);
    window.visualViewport?.addEventListener('scroll', schedule);
    return () => {
      cancelAnimationFrame(frame);
      window.removeEventListener('resize', schedule);
      window.removeEventListener('scroll', schedule, true);
      window.visualViewport?.removeEventListener('resize', schedule);
      window.visualViewport?.removeEventListener('scroll', schedule);
    };
  }, [open, panel]);

  useEffect(() => {
    if (!error || open) return undefined;
    const hide = () => setError('');
    window.addEventListener('scroll', hide, true);
    window.addEventListener('resize', hide);
    return () => {
      window.removeEventListener('scroll', hide, true);
      window.removeEventListener('resize', hide);
    };
  }, [error, open]);

  useEffect(() => {
    if (panel !== 'years' || !yearListRef.current) return;
    const list = yearListRef.current;
    const active = list.querySelector('.dpf-cell-active');
    if (active) list.scrollTop = active.offsetTop - list.clientHeight / 2 + active.offsetHeight / 2;
  }, [panel]);
  const weeks = useMemo(() => {
    const first = new Date(view.y, view.m, 1);
    const start = new Date(view.y, view.m, 1 - first.getDay());
    const rows = [];
    for (let w = 0; w < 6; w++) {
      const row = [];
      for (let i = 0; i < 7; i++) row.push(new Date(start.getFullYear(), start.getMonth(), start.getDate() + w * 7 + i));
      rows.push(row);
    }
    return rows;
  }, [view]);
  const minP = parseYMD(min);
  const maxP = parseYMD(max);
  const minKey = minP ? monthKey(minP.y, minP.m) : null;
  const maxKey = maxP ? monthKey(maxP.y, maxP.m) : null;
  const viewKey = monthKey(view.y, view.m);
  const prevDisabled = minKey !== null && viewKey <= minKey;
  const nextDisabled = maxKey !== null && viewKey >= maxKey;
  const firstYear = minP.y;
  const lastYear = maxP.y;
  const years = useMemo(() => {
    const list = [];
    for (let y = firstYear; y <= lastYear; y++) list.push(y);
    return list;
  }, [firstYear, lastYear]);
  const monthDisabled = (y, m) => minKey !== null && monthKey(y, m) < minKey || maxKey !== null && monthKey(y, m) > maxKey;
  function goMonth(delta) {
    setView(v => {
      const d = new Date(v.y, v.m + delta, 1);
      return {
        y: d.getFullYear(),
        m: d.getMonth()
      };
    });
  }
  function goYear(delta) {
    setView(v => ({
      y: Math.min(Math.max(v.y + delta, firstYear), lastYear),
      m: v.m
    }));
  }
  function chooseYear(y) {
    let m = view.m;
    if (minKey !== null && monthKey(y, m) < minKey) m = minP.m;
    if (maxKey !== null && monthKey(y, m) > maxKey) m = maxP.m;
    setView({
      y,
      m
    });
    setPanel('months');
  }
  function chooseMonth(m) {
    setView(v => ({
      y: v.y,
      m
    }));
    setPanel('days');
  }
  function flashError(message) {
    const r = boxRef.current?.getBoundingClientRect();
    const vh = window.visualViewport?.height || window.innerHeight;
    const vw = window.visualViewport?.width || window.innerWidth;
    if (r) {
      const up = vh - r.bottom < 90 && r.top > 90;
      setErrorPos({
        top: up ? undefined : Math.round(r.bottom + 6),
        bottom: up ? Math.round(vh - r.top + 6) : undefined,
        left: Math.round(Math.max(8, Math.min(r.left, vw - 8 - Math.min(320, vw - 32))))
      });
    }
    setError(message);
    clearTimeout(errorTimerRef.current);
    errorTimerRef.current = setTimeout(() => setError(''), 4000);
  }
  function rangeMessage() {
    return `เลือกได้ระหว่าง ${formatShortThaiDate(min)} – ${formatShortThaiDate(max)}`;
  }

  function commitText(raw) {
    const trimmed = String(raw ?? '').trim();
    if (!trimmed) return true;
    const parsed = parseTypedDate(trimmed, {
      min,
      max
    });
    if (!parsed.ymd) {
      flashError('วันที่ไม่ถูกต้อง — พิมพ์แบบ วว/ดด/ปปปป เช่น 15/07/2569');
      return false;
    }
    if (min && parsed.ymd < min || max && parsed.ymd > max) {
      flashError(rangeMessage());
      return false;
    }
    setError('');
    if (parsed.ymd !== lastValueRef.current) {
      lastValueRef.current = parsed.ymd;
      onChange(parsed.ymd);
    }
    return true;
  }
  function setTextBoth(next) {
    textRef.current = next;
    setText(next);
  }
  function handleFocus() {
    setEditing(true);
    setTextBoth(formatTypedDate(value));
    requestAnimationFrame(() => inputRef.current?.select());
    if (pointerTypeRef.current !== 'touch') setOpen(true);
  }
  function handleTextChange(e) {
    const next = e.target.value;
    setTextBoth(next);
    if (error) setError('');
    const parsed = parseTypedDate(next, {
      min,
      max
    });
    const p = parseYMD(parsed.ymd);
    if (p) {
      setView({
        y: p.y,
        m: p.m
      });
      setPanel('days');
    }
  }
  function handleBlur() {
    commitText(textRef.current);
    setEditing(false);
  }
  function handleKeyDown(e) {
    if (e.key === 'Enter') {
      e.preventDefault();
      if (commitText(textRef.current)) {
        setOpen(false);
        inputRef.current?.blur();
      }
    } else if (e.key === 'ArrowDown' && !open) {
      e.preventDefault();
      setOpen(true);
    }
  }
  function pick(ymd) {
    setError('');
    setTextBoth(formatTypedDate(ymd));
    if (ymd !== lastValueRef.current) {
      lastValueRef.current = ymd;
      onChange(ymd);
    }
    setOpen(false);
    if (document.activeElement === inputRef.current) inputRef.current.blur();
  }
  const triggerLabel = selected ? `${selected.d} ${THAI_MONTHS[selected.m]} ${selected.y + 543}` : placeholder;
  const todayDisabled = min && todayYMD < min || max && todayYMD > max;
  return <div className="dpf" ref={boxRef} data-scan-ignore="">
      {renderTrigger ? renderTrigger({
      open,
      toggle: () => setOpen(o => !o),
      label: triggerLabel,
      hasValue: !!selected
    }) : <div className={'dpf-trigger' + (open || editing ? ' dpf-trigger-open' : '') + (error ? ' dpf-trigger-error' : '')} onPointerDown={e => {
      pointerTypeRef.current = e.pointerType || 'mouse';
    }}>
          <input ref={inputRef} className={'dpf-input' + (selected || editing ? '' : ' dpf-input-empty')} type="text" inputMode="numeric" autoComplete="off" spellCheck={false} value={editing ? text : selected ? triggerLabel : ''} placeholder={editing ? 'วว/ดด/ปปปป' : placeholder} onFocus={handleFocus} onChange={handleTextChange} onBlur={handleBlur} onKeyDown={handleKeyDown} aria-label="วันที่ (พิมพ์ได้ เช่น 15/07/2569)" aria-invalid={!!error} />
          <button type="button" className="dpf-cal-btn" onMouseDown={e => e.preventDefault()} onClick={() => setOpen(o => !o)} aria-haspopup="dialog" aria-expanded={open} aria-label="เปิดปฏิทิน" title="เลือกจากปฏิทิน">
            <CalendarIcon />
          </button>
        </div>}

      {error && !open && errorPos && createPortal(<div className="dpf-error" role="alert" data-scan-ignore="" style={errorPos}>{error}</div>, document.body)}

      {open && createPortal(<div ref={popRef} className={'dpf-pop' + (placement?.maxH ? ' dpf-pop-scroll' : '')} style={{
      top: placement ? placement.top : 0,
      left: placement ? placement.left : 0,
      maxHeight: placement?.maxH || undefined,
      visibility: placement ? 'visible' : 'hidden'
    }} role="dialog" data-scan-ignore=""
    onMouseDown={e => e.preventDefault()}>
          {error && <div className="dpf-error dpf-error-inline" role="alert">{error}</div>}

          {panel === 'days' && <>
              <div className="dpf-head">
                <button type="button" className="dpf-nav" onClick={() => goMonth(-1)} disabled={prevDisabled} aria-label="เดือนก่อนหน้า">
                  <Chevron dir="left" />
                </button>
                <div className="dpf-title-group">
                  <button type="button" className="dpf-title-btn" onClick={() => setPanel('months')} aria-label="เลือกเดือน">
                    {THAI_MONTHS[view.m]}
                    <Chevron dir="down" />
                  </button>
                  <button type="button" className="dpf-title-btn" onClick={() => setPanel('years')} aria-label="เลือกปี">
                    {view.y + 543}
                    <Chevron dir="down" />
                  </button>
                </div>
                <button type="button" className="dpf-nav" onClick={() => goMonth(1)} disabled={nextDisabled} aria-label="เดือนถัดไป">
                  <Chevron dir="right" />
                </button>
              </div>

              <div className="dpf-weekdays">
                {THAI_WEEKDAYS.map((w, i) => <div key={i} className="dpf-weekday">{w}</div>)}
              </div>

              <div className="dpf-grid">
                {weeks.map((row, ri) => row.map((d, ci) => {
              const ymd = toYMD(d);
              const inMonth = d.getMonth() === view.m;
              const isSelected = value === ymd;
              const isToday = todayYMD === ymd;
              const disabled = min && ymd < min || max && ymd > max;
              return <button key={`${ri}-${ci}`} type="button" className={'dpf-day' + (inMonth ? '' : ' dpf-day-out') + (isSelected ? ' dpf-day-selected' : '') + (isToday && !isSelected ? ' dpf-day-today' : '')} disabled={disabled} onClick={() => pick(ymd)}>
                        {d.getDate()}
                      </button>;
            }))}
              </div>
            </>}

          {panel === 'months' && <>
              <div className="dpf-head">
                <button type="button" className="dpf-nav" onClick={() => goYear(-1)} disabled={view.y <= firstYear} aria-label="ปีก่อนหน้า">
                  <Chevron dir="left" />
                </button>
                <button type="button" className="dpf-title-btn" onClick={() => setPanel('years')} aria-label="เลือกปี">
                  ปี {view.y + 543}
                  <Chevron dir="down" />
                </button>
                <button type="button" className="dpf-nav" onClick={() => goYear(1)} disabled={view.y >= lastYear} aria-label="ปีถัดไป">
                  <Chevron dir="right" />
                </button>
              </div>
              <div className="dpf-cells dpf-cells-months">
                {THAI_MONTHS_SHORT.map((label, m) => {
              const isActive = m === view.m;
              const isSelected = selected && selected.y === view.y && selected.m === m;
              return <button key={label} type="button" className={'dpf-cell' + (isActive ? ' dpf-cell-active' : '') + (isSelected ? ' dpf-cell-selected' : '')} disabled={monthDisabled(view.y, m)} onClick={() => chooseMonth(m)} title={THAI_MONTHS[m]}>
                      {label}
                    </button>;
            })}
              </div>
            </>}

          {panel === 'years' && <>
              <div className="dpf-head dpf-head-center">
                <button type="button" className="dpf-title-btn" onClick={() => setPanel('days')} aria-label="กลับไปหน้าปฏิทิน">
                  เลือกปี พ.ศ. {firstYear + 543} – {lastYear + 543}
                </button>
              </div>
              <div className="dpf-cells dpf-cells-years" ref={yearListRef}>
                {years.map(y => {
              const isActive = y === view.y;
              const isSelected = selected && selected.y === y;
              return <button key={y} type="button" className={'dpf-cell' + (isActive ? ' dpf-cell-active' : '') + (isSelected ? ' dpf-cell-selected' : '')} onClick={() => chooseYear(y)} title={`ค.ศ. ${y}`}>
                      {y + 543}
                    </button>;
            })}
              </div>
            </>}

          <div className="dpf-foot">
            <span className="dpf-hint" />
            <button type="button" className="dpf-today" onClick={() => pick(todayYMD)} disabled={!!todayDisabled}>
              วันนี้
            </button>
          </div>
        </div>, document.body)}
    </div>;
}
