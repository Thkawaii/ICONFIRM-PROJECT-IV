import DatePickerField from './DatePickerField.jsx';
import { CalendarDaysIcon, XMarkIcon } from './icons.jsx';
import { PERIOD_MODE_OPTIONS, PERIOD_ANCHOR_HINT, periodRangeLabel } from '../lib/dateRange.js';
import './PeriodRangePicker.css';

export default function PeriodRangePicker({
  mode = 'all',
  onModeChange,
  anchor = '',
  onAnchorChange,
  min,
  max,
  label = 'ช่วงเวลา',
  countLabel,
  onClear,
  clearLabel = 'ล้างช่วง'
}) {
  const showAnchor = mode && mode !== 'all';
  const rangeLabel = periodRangeLabel(mode, anchor);
  return <div className="prp">
      <div className="prp-field prp-field-modes">
        <span className="prp-label">{label}</span>
        <div className="vr-tabs prp-modes" role="tablist">
          {PERIOD_MODE_OPTIONS.map(o => <button key={o.key} type="button" role="tab" aria-selected={mode === o.key} className={'vr-tab prp-mode' + (mode === o.key ? ' vr-tab-active' : '')} onClick={() => onModeChange?.(o.key)}>
              {o.label}
            </button>)}
        </div>
      </div>

      {showAnchor && <div className="prp-field prp-anchor">
          <span className="prp-label">{PERIOD_ANCHOR_HINT[mode] || 'เลือกวันอ้างอิง'}</span>
          <DatePickerField value={anchor} onChange={onAnchorChange} min={min} max={max} placeholder="— เลือกวัน —" />
        </div>}

      {showAnchor && <div className="prp-summary">
          <div className="prp-chip" title="ช่วงที่จะแสดง/ส่งออก">
            <CalendarDaysIcon className="size-4" />
            <span className="prp-chip-range">{rangeLabel}</span>
            {countLabel && <span className="prp-chip-count">{countLabel}</span>}
          </div>
          {onClear && <button type="button" className="prp-clear" onClick={onClear}>
              <XMarkIcon className="size-4" />
              {clearLabel}
            </button>}
        </div>}
    </div>;
}
