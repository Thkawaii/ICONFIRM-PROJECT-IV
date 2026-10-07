import { useEffect, useRef, useState } from 'react';

export default function SelectField({
  value,
  onChange,
  options = [],
  placeholder = '— เลือก —',
  disabled = false,
  // searchable: มีช่องพิมพ์ค้นหาอยู่บนรายการ (ใช้ตอนตัวเลือกเยอะ เช่น เลือก MC#)
  searchable = false,
  searchPlaceholder = 'พิมพ์เพื่อค้นหา...',
  id
}) {
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);
  const [term, setTerm] = useState('');
  const boxRef = useRef(null);
  const listRef = useRef(null);
  const searchRef = useRef(null);
  const selected = options.find(o => o.value === value);
  const kw = term.trim().toLowerCase();
  const shown = searchable && kw ? options.filter(o => String(o.label ?? o.value).toLowerCase().includes(kw)) : options;
  useEffect(() => {
    if (!open) return;
    function onOutside(e) {
      if (boxRef.current && !boxRef.current.contains(e.target)) setOpen(false);
    }
    document.addEventListener('mousedown', onOutside);
    return () => document.removeEventListener('mousedown', onOutside);
  }, [open]);
  useEffect(() => {
    if (!open) {
      setTerm('');
      return;
    }
    setActive(Math.max(shown.findIndex(o => o.value === value), 0));
    if (searchable) {
      const t = setTimeout(() => searchRef.current?.focus(), 0);
      return () => clearTimeout(t);
    }
    return undefined;
  }, [open, value, options, searchable]);
  useEffect(() => {
    setActive(0);
  }, [term]);
  function choose(option) {
    onChange(option.value);
    setOpen(false);
  }
  function onKeyDown(e) {
    if (disabled) return;
    if (e.key === 'Escape') return setOpen(false);
    if (!open && (e.key === 'Enter' || e.key === ' ' || e.key === 'ArrowDown')) {
      e.preventDefault();
      return setOpen(true);
    }
    if (!open) return;
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      setActive(i => Math.min(i + 1, shown.length - 1));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setActive(i => Math.max(i - 1, 0));
    } else if (e.key === 'Enter') {
      e.preventDefault();
      if (shown[active]) choose(shown[active]);
    }
  }
  return <div className="sf" ref={boxRef}>
      <button type="button" id={id} className={'sf-trigger' + (open ? ' sf-trigger-open' : '')} onClick={() => !disabled && setOpen(!open)} onKeyDown={onKeyDown} disabled={disabled} aria-haspopup="listbox" aria-expanded={open}>
        <span className="sf-value-wrap">
          <span className={selected ? 'sf-value' : 'sf-value sf-value-empty'}>
            {selected ? selected.label : placeholder}
          </span>
          {selected?.suffix && <span className="sf-suffix">{selected.suffix}</span>}
        </span>
        <span className="sf-chevron" aria-hidden="true">
          <svg width="12" height="8" viewBox="0 0 12 8" fill="none">
            <path d="M1 1.5L6 6.5L11 1.5" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
        </span>
      </button>

      {open && <ul className="sf-list" role="listbox" ref={listRef}>
          {searchable && <li className="sf-search-row">
              <input ref={searchRef} className="sf-search" type="text" value={term} placeholder={searchPlaceholder} autoComplete="off" spellCheck="false" data-scan-ignore onChange={e => setTerm(e.target.value)} onKeyDown={onKeyDown} onClick={e => e.stopPropagation()} />
            </li>}

          {shown.map((option, i) => <li key={option.value}>
              <button type="button" role="option" aria-selected={option.value === value} className={'sf-option' + (option.value === value ? ' sf-option-selected' : '') + (i === active ? ' sf-option-active' : '')} onMouseEnter={() => setActive(i)} onClick={() => choose(option)}>
                {option.label}
                {option.suffix && <span className="sf-suffix sf-option-suffix">{option.suffix}</span>}
              </button>
            </li>)}

          {!shown.length && <li className="sf-empty">
              {options.length ? 'ไม่พบตัวเลือกที่ค้นหา' : 'ไม่มีตัวเลือก'}
            </li>}
        </ul>}
    </div>;
}
