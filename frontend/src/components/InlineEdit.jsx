import { useEffect, useLayoutEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { CheckCircleIcon, CheckIcon, LockClosedIcon, PencilSquareIcon } from './icons.jsx';
import { toastError } from '../lib/toast.js';
import './InlineEdit.css';

// ---------------------------------------------------------------------------
// แก้ข้อมูลในตาราง: กดไอคอนดินสอ → พิมพ์ → Enter (หรือคลิกออก) = บันทึก, Esc = ยกเลิก
// คลิกที่ตัวข้อความเฉย ๆ ไม่เข้าโหมดแก้ไข กันมือไปโดนโดยไม่ตั้งใจ
// แถวที่สแกนผ่านแล้ว (locked) จะแก้ไม่ได้ และแสดงเหตุผลเมื่อชี้เมาส์
// ---------------------------------------------------------------------------

function toDateInput(value) {
  if (!value) return '';
  const s = String(value);
  if (/^\d{4}-\d{2}-\d{2}/.test(s)) return s.slice(0, 10);
  const d = new Date(s);
  if (Number.isNaN(d.getTime())) return '';
  const pad = n => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

function normalize(type, value) {
  if (value == null) return '';
  if (type === 'date') return toDateInput(value);
  return String(value);
}

export function EditableCell({
  value,
  onSave,
  locked = false,
  lockReason = '',
  type = 'text',
  options = [],
  display,
  mono = false,
  label = '',
  placeholder = '—',
  readOnly = false
}) {
  const current = normalize(type, value);
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(current);
  const [saving, setSaving] = useState(false);
  const [flash, setFlash] = useState('');
  const inputRef = useRef(null);
  const doneRef = useRef(false);

  useEffect(() => {
    if (!editing) setDraft(current);
  }, [current, editing]);

  useEffect(() => {
    if (editing && inputRef.current) {
      inputRef.current.focus();
      if (type === 'text' && inputRef.current.select) inputRef.current.select();
    }
  }, [editing, type]);

  useEffect(() => {
    if (!flash) return undefined;
    const t = setTimeout(() => setFlash(''), 1400);
    return () => clearTimeout(t);
  }, [flash]);

  const shown = display !== undefined ? display : current.trim() ? current : <span className="ie-empty">{placeholder}</span>;

  if (readOnly && !locked) {
    return <span className={['ie-cell', mono ? 'ie-mono' : ''].join(' ')}>{shown}</span>;
  }

  if (locked) {
    return <span className={['ie-cell', 'ie-locked', mono ? 'ie-mono' : ''].join(' ')} title={lockReason ? `แก้ไม่ได้ — ${lockReason}` : 'แก้ไม่ได้ — สแกนผ่านแล้ว'}>
        {shown}
      </span>;
  }

  function start() {
    if (saving) return;
    doneRef.current = false;
    setDraft(current);
    setEditing(true);
  }

  function cancel() {
    doneRef.current = true;
    setDraft(current);
    setEditing(false);
  }

  async function commit(next = draft) {
    if (doneRef.current) return;
    doneRef.current = true;
    const cleaned = type === 'select' || type === 'date' ? next : String(next ?? '').trim();
    if (cleaned === current.trim()) {
      setEditing(false);
      return;
    }
    setSaving(true);
    try {
      await onSave(cleaned);
      setFlash('ok');
    } catch (err) {
      toastError(err?.message || 'บันทึกไม่สำเร็จ');
      setDraft(current);
      setFlash('err');
    } finally {
      setSaving(false);
      setEditing(false);
    }
  }

  function onKeyDown(e) {
    if (e.key === 'Enter') {
      e.preventDefault();
      commit();
    } else if (e.key === 'Escape') {
      e.preventDefault();
      cancel();
    }
  }

  if (editing) {
    const common = {
      ref: inputRef,
      className: ['ie-input', mono ? 'ie-mono' : ''].join(' '),
      value: draft,
      disabled: saving,
      'aria-label': label ? `แก้ไข ${label}` : 'แก้ไข',
      onKeyDown,
      onBlur: () => commit()
    };
    if (type === 'select') {
      return <span className="ie-cell ie-editing">
          <SelectEditor value={draft} options={options} label={label} saving={saving} onChoose={v => {
          setDraft(v);
          commit(v);
        }} onCancel={cancel} />
        </span>;
    }
    return <span className="ie-cell ie-editing">
        <input {...common} type={type === 'date' ? 'date' : 'text'} inputMode={type === 'number' ? 'numeric' : undefined} onChange={e => setDraft(e.target.value)} />
      </span>;
  }

  return <span className={['ie-cell', 'ie-editable', mono ? 'ie-mono' : '', saving ? 'ie-saving' : '', flash === 'ok' ? 'ie-flash-ok' : '', flash === 'err' ? 'ie-flash-err' : ''].filter(Boolean).join(' ')}>
      <span className="ie-text">{shown}</span>
      <button type="button" className="ie-edit-btn" disabled={saving} title={label ? `แก้ไข ${label}` : 'แก้ไข'} aria-label={label ? `แก้ไข ${label}` : 'แก้ไข'} onClick={start}>
        <PencilSquareIcon className="size-4" aria-hidden="true" />
      </button>
    </span>;
}

// SelectEditor: รายการตัวเลือกแบบเดียวกับ SelectField ของระบบ
// เปิดรายการทันทีเมื่อกดดินสอ, วางรายการไว้บน body (ไม่โดนตารางตัดขอบ), รองรับ ↑ ↓ Enter Esc
function SelectEditor({
  value,
  options,
  label,
  saving,
  onChoose,
  onCancel
}) {
  const triggerRef = useRef(null);
  const listRef = useRef(null);
  const [open, setOpen] = useState(true);
  const [active, setActive] = useState(Math.max(options.findIndex(o => o.value === value), 0));
  const [pos, setPos] = useState(null);
  const selected = options.find(o => o.value === value);

  useLayoutEffect(() => {
    if (!open) return undefined;
    const place = () => {
      const el = triggerRef.current;
      if (!el) return;
      const r = el.getBoundingClientRect();
      const listH = Math.min(listRef.current?.offsetHeight || 220, 280);
      const below = window.innerHeight - r.bottom;
      const up = below < listH + 12 && r.top > below;
      setPos({
        left: Math.max(8, Math.min(r.left, window.innerWidth - Math.max(r.width, 240) - 8)),
        top: up ? r.top - listH - 6 : r.bottom + 6,
        minWidth: Math.max(r.width, 240)
      });
    };
    place();
    window.addEventListener('resize', place);
    window.addEventListener('scroll', place, true);
    return () => {
      window.removeEventListener('resize', place);
      window.removeEventListener('scroll', place, true);
    };
  }, [open]);

  useEffect(() => {
    triggerRef.current?.focus();
  }, []);

  useEffect(() => {
    function onOutside(e) {
      if (triggerRef.current?.contains(e.target) || listRef.current?.contains(e.target)) return;
      onCancel();
    }
    document.addEventListener('mousedown', onOutside);
    return () => document.removeEventListener('mousedown', onOutside);
  }, [onCancel]);

  useEffect(() => {
    if (open && listRef.current) {
      listRef.current.querySelector('[data-active="true"]')?.scrollIntoView({
        block: 'nearest'
      });
    }
  }, [active, open]);

  function onKeyDown(e) {
    if (saving) return;
    if (e.key === 'Escape') {
      e.preventDefault();
      onCancel();
    } else if (e.key === 'ArrowDown') {
      e.preventDefault();
      if (!open) return setOpen(true);
      setActive(i => Math.min(i + 1, options.length - 1));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setActive(i => Math.max(i - 1, 0));
    } else if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      if (!open) return setOpen(true);
      if (options[active]) onChoose(options[active].value);
    } else if (e.key === 'Tab') {
      onCancel();
    }
  }

  return <>
      <button type="button" ref={triggerRef} className={'ie-select-trigger' + (open ? ' ie-select-open' : '')} disabled={saving} aria-haspopup="listbox" aria-expanded={open} aria-label={label ? `เลือก ${label}` : 'เลือก'} onClick={() => setOpen(o => !o)} onKeyDown={onKeyDown}>
        <span className="ie-select-value">{selected ? selected.label : '— เลือก —'}</span>
        <svg className="ie-select-chevron" width="12" height="8" viewBox="0 0 12 8" fill="none" aria-hidden="true">
          <path d="M1 1.5L6 6.5L11 1.5" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      </button>
      {open && createPortal(<ul ref={listRef} className="ie-select-list" role="listbox" aria-label={label} style={pos ? {
      left: pos.left,
      top: pos.top,
      minWidth: pos.minWidth
    } : {
      visibility: 'hidden'
    }}>
            {options.map((o, i) => {
          const isSel = o.value === value;
          return <li key={o.value}>
                  <button type="button" role="option" aria-selected={isSel} data-active={i === active} className={'ie-select-option' + (isSel ? ' ie-select-option-selected' : '') + (i === active ? ' ie-select-option-active' : '')} onMouseEnter={() => setActive(i)} onMouseDown={e => e.preventDefault()} onClick={() => onChoose(o.value)}>
                    <span>
                      {o.label}
                      {o.hint && <span className="ie-select-hint">{o.hint}</span>}
                    </span>
                    {isSel && <CheckIcon className="size-4 ie-select-check" aria-hidden="true" />}
                  </button>
                </li>;
        })}
          </ul>, document.body)}
    </>;
}

export function LockBadge({
  locked,
  reason,
  variant = 'lock'
}) {
  if (!locked) return null;
  // variant="done": ป้ายสีเขียว + ไอคอนเครื่องหมายถูก (ใช้ในหน้า Import License)
  if (variant === 'done') {
    return <span className="ie-lock-badge ie-scan-done-badge" title={reason || 'สแกนผ่านแล้ว'}>
        <CheckCircleIcon className="size-3.5" aria-hidden="true" />
        สแกนแล้ว
      </span>;
  }
  return <span className="ie-lock-badge" title={reason || 'สแกนผ่านแล้ว'}>
      <LockClosedIcon className="size-3.5" aria-hidden="true" />
      สแกนแล้ว
    </span>;
}

// EditHint: บอกวิธีแก้ในตาราง + จำนวนแถวที่ล็อก (สแกนแล้ว)
// ตารางที่แก้ไม่ได้ (canEdit=false) จะไม่แสดงข้อความวิธีแก้ — แสดงเฉพาะจำนวนแถวที่สแกนแล้ว (ถ้ามี)
export function EditHint({
  lockedCount = 0,
  showLock = true,
  canEdit = true
}) {
  const lockText = showLock && lockedCount > 0 ? `${lockedCount} แถวสแกนแล้ว แก้ไข/ลบไม่ได้` : '';
  if (!canEdit && !lockText) return null;
  return <p className="ie-live-hint" style={{
    margin: '4px 0 0'
  }}>
      <span className="ie-live-dot" aria-hidden="true" />
      {canEdit ? `กดไอคอนดินสอเพื่อแก้ไข กด Enter เพื่อบันทึก ข้อมูลอัปเดตอัตโนมัติ${lockText ? `, ${lockText}` : ''}` : lockText}
    </p>;
}

// useLiveRefresh: ดึงข้อมูลใหม่อัตโนมัติ (ค่าเริ่มต้นทุก 10 วินาที) และทันทีที่กลับมาที่แท็บนี้
// หยุดชั่วคราวระหว่างที่กำลังพิมพ์แก้ช่องใดช่องหนึ่ง เพื่อไม่ให้ค่าที่กำลังพิมพ์หาย
export function useLiveRefresh(refresh, {
  intervalMs = 10000,
  enabled = true
} = {}) {
  const ref = useRef(refresh);
  ref.current = refresh;
  useEffect(() => {
    if (!enabled) return undefined;
    const tick = () => {
      if (document.visibilityState !== 'visible') return;
      if (document.querySelector('.ie-editing')) return;
      ref.current?.();
    };
    const id = setInterval(tick, intervalMs);
    const onVisible = () => {
      if (document.visibilityState === 'visible') tick();
    };
    document.addEventListener('visibilitychange', onVisible);
    window.addEventListener('focus', onVisible);
    return () => {
      clearInterval(id);
      document.removeEventListener('visibilitychange', onVisible);
      window.removeEventListener('focus', onVisible);
    };
  }, [intervalMs, enabled]);
}
