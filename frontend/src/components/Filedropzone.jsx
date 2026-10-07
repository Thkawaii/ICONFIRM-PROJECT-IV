import { useRef } from 'react';
import { ArrowUpTrayIcon, XMarkIcon } from './icons.jsx';
import useFileDrop from '../lib/useFileDrop.js';
export default function FileDropZone({
  file,
  onSelect,
  accept = '',
  label = 'เลือกไฟล์',
  hint = '',
  disabled = false
}) {
  const inputRef = useRef(null);
  const extensions = accept.split(',').map(a => a.trim().replace('.', '').toUpperCase()).filter(Boolean);
  function accepts(candidate) {
    if (!extensions.length) return true;
    const ext = candidate.name.split('.').pop()?.toUpperCase();
    return extensions.includes(ext);
  }
  function pick(candidate) {
    if (!candidate || disabled) return;
    if (!accepts(candidate)) return;
    onSelect(candidate);
  }
  const {
    dragging,
    stateClass,
    dropProps
  } = useFileDrop({
    accept,
    disabled,
    onFile: pick
  });
  function openPicker() {
    if (!disabled) inputRef.current?.click();
  }
  const className = ['fdz', file ? 'fdz-filled' : '', disabled ? 'fdz-disabled' : '', stateClass].filter(Boolean).join(' ');
  return <div className={className} onClick={openPicker} {...dropProps} onKeyDown={e => {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      openPicker();
    }
  }} role="button" tabIndex={disabled ? -1 : 0} aria-label={label}>
      <span className={file ? 'fdz-badge ' + badgeTone(file.name) : 'fdz-icon'}>
        {file ? extOf(file.name) : <ArrowUpTrayIcon className="size-[22px]" />}
      </span>

      <span className="fdz-body">
        {file ? <>
            <span className="fdz-name">{file.name}</span>
            <span className="fdz-meta">{formatSize(file.size)}</span>
          </> : <>
            <span className="fdz-label">{dragging ? <span className="dz-drop-text"><span className="dz-arrow">↓</span> ปล่อยไฟล์ได้เลย</span> : label}</span>
            <span className="fdz-meta">
              {hint || extensions.join(' / ')}
            </span>
          </>}
      </span>

      {!file && !disabled && <span className="fdz-cta">เลือกไฟล์</span>}

      {file && !disabled && <button type="button" className="fdz-clear" aria-label="เอาไฟล์ออก" onClick={e => {
      e.stopPropagation();
      onSelect(null);
      if (inputRef.current) inputRef.current.value = '';
    }}>
          <XMarkIcon className="size-3.5" />
        </button>}

      <input ref={inputRef} type="file" accept={accept} className="fdz-input" disabled={disabled} onChange={e => {
      pick(e.target.files?.[0]);
      e.target.value = '';
    }} />
    </div>;
}
function formatSize(bytes) {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}
function extOf(name) {
  return (name.split('.').pop() || '').toUpperCase().slice(0, 4);
}
function badgeTone(name) {
  return extOf(name) === 'PDF' ? 'fdz-badge-pdf' : 'fdz-badge-xls';
}
