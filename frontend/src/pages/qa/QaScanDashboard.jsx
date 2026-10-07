import { useEffect, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { getQAPartScanSummary } from '../../api/qaPartScan.js';
import { resolvePeriodRange, periodRangeLabel, shiftPeriodAnchor } from '../../lib/dateRange.js';
import { ArrowPathIcon, ChevronLeftIcon, ChevronRightIcon, QrCodeIcon, WrenchScrewdriverIcon, XMarkIcon } from '../../components/icons.jsx';

const COMPONENT_LABELS = {
  ITC: 'IT Controller',
  CV: 'Control Valve',
  SM: 'Swing Motor',
  MP: 'Motor Propel',
  PH: 'Pump Assy HYD',
  EN: 'Engine',
  CW: 'Counter Weight'
};
const MODE_OPTIONS = [{
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
const pad2 = n => String(n).padStart(2, '0');
const todayYMD = () => {
  const d = new Date();
  return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`;
};
const dash = v => v && String(v).trim() !== '' ? v : '—';
const pct = (a, b) => b > 0 ? Math.round(a / b * 100) : 0;
function inRange(value, range) {
  if (!range) return true;
  if (!value) return false;
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return false;
  return d >= range.start && d < range.end;
}
const fmtTime = v => v ? new Date(v).toLocaleString('th-TH', {
  day: 'numeric',
  month: 'short',
  year: '2-digit',
  hour: '2-digit',
  minute: '2-digit'
}) : '';

function PeriodBar({
  mode,
  onMode,
  anchor,
  onAnchor,
  rangeLabel,
  onRefresh,
  refreshing
}) {
  return <div className="flex flex-wrap items-center gap-2">
      <div className="inline-flex rounded-xl border border-solid border-slate-200 bg-white p-1 shadow-[0_1px_2px_rgb(16_24_40/0.04)]">
        {MODE_OPTIONS.map(o => <button key={o.key} type="button" onClick={() => onMode(o.key)} className={'cursor-pointer appearance-none rounded-lg border-0 px-3 py-1.5 font-sans text-[13px] font-semibold transition ' + (mode === o.key ? 'bg-brand-500 text-brand-ink' : 'bg-transparent text-slate-500 hover:bg-slate-100 hover:text-slate-800')}>
            {o.label}
          </button>)}
      </div>

      {mode !== 'all' && <div className="inline-flex items-center gap-1 rounded-xl border border-solid border-slate-200 bg-white p-1 shadow-[0_1px_2px_rgb(16_24_40/0.04)]">
          <button type="button" title="ช่วงก่อนหน้า" onClick={() => onAnchor(shiftPeriodAnchor(mode, anchor || todayYMD(), -1))} className="flex size-7 cursor-pointer appearance-none items-center justify-center rounded-lg border-0 bg-transparent text-slate-500 transition hover:bg-slate-100 hover:text-slate-900">
            <ChevronLeftIcon className="size-4" />
          </button>
          <span className="min-w-[120px] px-1 text-center text-[13px] font-semibold text-slate-700">
            {rangeLabel}
          </span>
          <button type="button" title="ช่วงถัดไป" onClick={() => onAnchor(shiftPeriodAnchor(mode, anchor || todayYMD(), 1))} className="flex size-7 cursor-pointer appearance-none items-center justify-center rounded-lg border-0 bg-transparent text-slate-500 transition hover:bg-slate-100 hover:text-slate-900">
            <ChevronRightIcon className="size-4" />
          </button>
          <input type="date" value={anchor || todayYMD()} onChange={e => onAnchor(e.target.value)} className="ml-1 cursor-pointer rounded-lg border border-solid border-slate-200 bg-white px-2 py-1 font-sans text-[12px] text-slate-600 outline-none focus:border-brand-500" />
        </div>}

      <button type="button" onClick={onRefresh} disabled={refreshing} className="ml-auto inline-flex cursor-pointer appearance-none items-center gap-1.5 rounded-xl border border-solid border-slate-200 bg-white px-3 py-2 font-sans text-[13px] font-semibold text-slate-600 transition hover:border-slate-300 hover:text-slate-900 disabled:cursor-not-allowed disabled:opacity-50">
        <ArrowPathIcon className={'size-4' + (refreshing ? ' animate-spin' : '')} />
        รีเฟรช
      </button>
    </div>;
}

function StageStat({
  title,
  subtitle,
  Icon,
  doneLabel,
  doneValue,
  pendingLabel,
  pendingValue,
  onOpenDone,
  onOpenPending
}) {
  const total = doneValue + pendingValue;
  const p = pct(doneValue, total);
  return <div className="group relative overflow-hidden rounded-2xl border border-solid border-slate-200 bg-linear-to-br from-white to-brand-50/40 p-4 shadow-[0_1px_2px_rgb(16_24_40/0.04)] transition duration-200 hover:border-brand-200 hover:shadow-[0_1px_2px_rgb(16_24_40/0.06),0_16px_32px_-24px_rgb(16_24_40/0.5)]">
      <span className="absolute top-0 left-0 h-[3px] w-12 rounded-r-full bg-linear-to-r from-brand-500 to-brand-600 transition-[width] duration-500 ease-out group-hover:w-full" />

      <div className="flex items-center gap-2.5">
        <span className="flex size-9 shrink-0 items-center justify-center rounded-xl bg-brand-50 text-brand-700 ring-1 ring-brand-100">
          <Icon className="size-4.5" />
        </span>
        <div className="min-w-0">
          <div className="truncate text-[14px] font-normal text-slate-800">{title}</div>
          <div className="truncate text-[11px] text-slate-400">{subtitle}</div>
        </div>
        <span className="ml-auto rounded-full bg-brand-50 px-2 py-0.5 text-[13px] font-bold text-brand-700 tabular-nums ring-1 ring-brand-100">
          {p}%
        </span>
      </div>

      <div className="mt-3 h-2 w-full overflow-hidden rounded-full bg-slate-100">
        <span className="block h-full rounded-full bg-linear-to-r from-brand-500 to-brand-600 transition-[width] duration-700 ease-out" style={{
        width: `${p}%`
      }} />
      </div>

      <div className="mt-3 grid grid-cols-2 gap-2">
        <button type="button" onClick={onOpenDone} className="flex cursor-pointer appearance-none items-center gap-2 rounded-xl border-0 bg-brand-50 px-3 py-2.5 text-left transition hover:bg-brand-100 focus-visible:ring-4 focus-visible:ring-brand-500/25 focus-visible:outline-none">
          <span className="size-2 shrink-0 rounded-full bg-brand-500" />
          <span className="truncate text-[12px] font-semibold text-brand-800">{doneLabel}</span>
          <span className="ml-auto text-[16px] leading-none font-bold text-brand-700 tabular-nums">
            {doneValue}
          </span>
        </button>
        <button type="button" onClick={onOpenPending} className="flex cursor-pointer appearance-none items-center gap-2 rounded-xl border-0 bg-slate-100 px-3 py-2.5 text-left transition hover:bg-slate-200/80 focus-visible:ring-4 focus-visible:ring-slate-400/25 focus-visible:outline-none">
          <span className="size-2 shrink-0 rounded-full bg-slate-400" />
          <span className="truncate text-[12px] font-semibold text-slate-600">{pendingLabel}</span>
          <span className="ml-auto text-[16px] leading-none font-bold text-brand-700 tabular-nums">
            {pendingValue}
          </span>
        </button>
      </div>

      <p className="mt-2 text-[11px] text-slate-400">รวม {total} ชิ้น</p>
    </div>;
}

function DetailModal({
  selection,
  units,
  onClose,
  onSelectionChange
}) {
  const [search, setSearch] = useState('');
  const [limit, setLimit] = useState(50);
  useEffect(() => {
    setSearch('');
    setLimit(50);
  }, [selection]);
  const onCloseRef = useRef(onClose);
  onCloseRef.current = onClose;
  useEffect(() => {
    const root = document.documentElement;
    const prevOverflow = document.body.style.overflow;
    const prevRootOverflow = root.style.overflow;
    document.body.style.overflow = 'hidden';
    root.style.overflow = 'hidden';
    const onKey = e => {
      if (e.key === 'Escape') onCloseRef.current();
    };
    window.addEventListener('keydown', onKey);
    return () => {
      document.body.style.overflow = prevOverflow;
      root.style.overflow = prevRootOverflow;
      window.removeEventListener('keydown', onKey);
    };
  }, []);
  const isWH = selection.kind === 'wh';
  const rows = useMemo(() => {
    const q = search.trim().toLowerCase();
    return units.filter(u => {
      const done = selection.range ? inRange(isWH ? u.scannedAt : u.assembledAt, selection.range) : isWH ? u.scannedInPeriod : u.assembledInPeriod;
      if (selection.status === 'done' && !done) return false;
      if (selection.status === 'pending' && done) return false;
      if (selection.component && u.component !== selection.component) return false;
      if (q) {
        const hay = [u.machineNo, u.model, u.componentLabel, u.plannedNo, u.scannedNo, u.scannedBy, u.assembledBy, ...(u.plannedNoFormer || []), ...(u.machineNoFormer || [])].join(' ').toLowerCase();
        if (!hay.includes(q)) return false;
      }
      return true;
    });
  }, [units, selection, search, isWH]);
  const components = useMemo(() => Array.from(new Set(units.map(u => u.component))), [units]);
  const stageName = isWH ? 'WH' : 'MFG';
  const statusName = selection.status === 'pending' ? isWH ? 'ยังไม่สแกน' : 'ยังไม่ประกอบ' : isWH ? 'สแกนแล้ว' : 'ประกอบแล้ว';
  return createPortal(<div className="fixed inset-0 z-[120] flex items-end justify-center overscroll-contain bg-slate-900/45 backdrop-blur-[2px] sm:items-center sm:p-3 lg:p-6" onClick={onClose}>
      <div className="flex max-h-[92vh] w-full max-w-4xl flex-col overflow-hidden rounded-t-2xl border border-solid border-slate-200 bg-white shadow-[0_24px_64px_-20px_rgb(16_24_40/0.5)] supports-[height:100dvh]:max-h-[92dvh] sm:max-h-[calc(100vh-2rem)] sm:rounded-2xl sm:supports-[height:100dvh]:max-h-[calc(100dvh-2rem)] lg:max-h-[calc(100vh-3rem)] lg:max-w-5xl lg:supports-[height:100dvh]:max-h-[calc(100dvh-3rem)]" onClick={e => e.stopPropagation()}>
        <div className="flex items-start gap-3 border-b border-solid border-slate-200 bg-slate-50/70 px-5 py-4">
          <div className="min-w-0 flex-1">
            <h3 className="truncate text-[16px] font-normal text-slate-900">
              {stageName} ({statusName})
            </h3>
            <p className="mt-0.5 text-[12px] text-slate-500">{rows.length} รายการ</p>
          </div>
          <button type="button" onClick={onClose} aria-label="ปิด" className="flex size-8 shrink-0 cursor-pointer appearance-none items-center justify-center rounded-lg border-0 bg-white text-slate-500 ring-1 ring-slate-200 transition hover:bg-slate-100 hover:text-slate-900">
            <XMarkIcon className="size-4" />
          </button>
        </div>

        <div className="flex flex-wrap items-center gap-1.5 border-b border-solid border-slate-200 px-5 py-3">
          <button type="button" onClick={() => onSelectionChange({
          ...selection,
          component: null
        })} className={'cursor-pointer appearance-none rounded-lg border border-solid px-2.5 py-1 font-sans text-[12px] font-semibold transition ' + (selection.component ? 'border-slate-200 bg-white text-slate-500 hover:border-slate-300' : 'border-transparent bg-slate-800 text-white')}>
            ทุกชนิด
          </button>
          {components.map(code => <button key={code} type="button" onClick={() => onSelectionChange({
          ...selection,
          component: code
        })} className={'cursor-pointer appearance-none rounded-lg border border-solid px-2.5 py-1 font-sans text-[12px] font-semibold transition ' + (selection.component === code ? 'border-transparent bg-slate-800 text-white' : 'border-slate-200 bg-white text-slate-500 hover:border-slate-300')}>
              {COMPONENT_LABELS[code] || code}
            </button>)}
          <input value={search} onChange={e => setSearch(e.target.value)} placeholder="ค้นหา..." className="ml-auto w-full min-w-[160px] rounded-lg border border-solid border-slate-200 bg-white px-3 py-1.5 font-sans text-[13px] text-slate-800 outline-none transition placeholder:text-slate-400 focus:border-brand-500 sm:w-48 pointer-coarse:text-[16px]" />
        </div>

        <div className="min-h-0 flex-1 overflow-x-hidden overflow-y-auto overscroll-contain [scrollbar-width:thin]">
          <table className="w-full border-collapse text-[13px]">
            <thead className="sticky top-0 z-10">
              <tr>
                {['Machine No', 'Model', 'ชนิดพาร์ท', 'เลขพาร์ท', 'WH สแกน', 'MFG ประกอบ'].map(h => <th key={h} className="border-b border-solid border-slate-200 bg-slate-50 px-2 py-2.5 text-left text-[11px] md:px-3 lg:px-4 font-semibold tracking-wider text-slate-500 uppercase">
                    {h}
                  </th>)}
              </tr>
            </thead>
            <tbody>
              {rows.slice(0, limit).map((u, i) => <tr key={`${u.machineNo}-${u.component}-${i}`} className="hover:bg-slate-50/70">
                  <td className="border-b border-solid border-slate-100 px-2 py-2.5 md:px-3 lg:px-4 font-mono font-semibold text-slate-800 [overflow-wrap:anywhere] sm:whitespace-nowrap">
                    {dash(u.machineNo)}
                  </td>
                  <td className="border-b border-solid border-slate-100 px-2 py-2.5 md:px-3 lg:px-4 text-slate-600 [overflow-wrap:anywhere] sm:whitespace-nowrap">
                    {dash(u.model)}
                  </td>
                  <td className="border-b border-solid border-slate-100 px-2 py-2.5 text-slate-600 md:px-3 lg:px-4">
                    {dash(u.componentLabel)}
                  </td>
                  <td className="border-b border-solid border-slate-100 px-2 py-2.5 md:px-3 lg:px-4 font-mono text-slate-700 [overflow-wrap:anywhere] sm:whitespace-nowrap">
                    {dash(u.plannedNo)}
                  </td>
                  <td className="border-b border-solid border-slate-100 px-2 py-2.5 md:px-3 lg:px-4">
                    {u.scanned ? <span className="text-slate-600">
                        <span className="lg:whitespace-nowrap">{fmtTime(u.scannedAt)}</span>{' '}
                        {u.scannedBy ? <span className="whitespace-nowrap text-slate-400">· {u.scannedBy}</span> : null}
                      </span> : u.scanAttempted ? <span className="font-semibold whitespace-nowrap text-amber-600" title={u.matchMessage || ''}>
                        สแกนไม่ผ่าน
                      </span> : <span className="whitespace-nowrap text-slate-400">ยังไม่สแกน</span>}
                  </td>
                  <td className="border-b border-solid border-slate-100 px-2 py-2.5 md:px-3 lg:px-4">
                    {u.assembled ? <span className="text-slate-600">
                        <span className="lg:whitespace-nowrap">{fmtTime(u.assembledAt)}</span>{' '}
                        {u.assembledBy ? <span className="whitespace-nowrap text-slate-400">· {u.assembledBy}</span> : null}
                      </span> : <span className="whitespace-nowrap text-slate-400">ยังไม่ประกอบ</span>}
                  </td>
                </tr>)}
              {rows.length === 0 && <tr>
                  <td colSpan={6} className="px-4 py-12 text-center text-slate-400">
                    ไม่พบรายการ
                  </td>
                </tr>}
            </tbody>
          </table>

          {rows.length > limit && <div className="p-4 text-center">
              <button type="button" onClick={() => setLimit(l => l + 100)} className="cursor-pointer appearance-none rounded-xl border border-solid border-slate-200 bg-white px-4 py-2 font-sans text-[13px] font-semibold text-slate-600 transition hover:border-slate-300 hover:text-slate-900">
                แสดงเพิ่มอีก 100 รายการ (เหลือ {rows.length - limit})
              </button>
            </div>}
        </div>
      </div>
    </div>, document.body);
}

export default function QAScanDashboard() {
  const [units, setUnits] = useState([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState('');
  const [mode, setMode] = useState('all');
  const [anchor, setAnchor] = useState(todayYMD());
  const [selection, setSelection] = useState(null);
  async function load(isRefresh) {
    if (isRefresh) setRefreshing(true);else setLoading(true);
    setError('');
    try {
      const res = await getQAPartScanSummary();
      setUnits(res?.units || []);
    } catch (err) {
      setError(err.message || 'โหลดข้อมูลแดชบอร์ดไม่สำเร็จ');
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }
  useEffect(() => {
    load(false);
    const onFocus = () => load(true);
    window.addEventListener('focus', onFocus);
    return () => window.removeEventListener('focus', onFocus);
  }, []);
  const range = useMemo(() => resolvePeriodRange(mode, anchor), [mode, anchor]);
  const rangeLabel = mode === 'all' ? 'ทั้งหมด' : periodRangeLabel(mode, anchor);
  const rows = useMemo(() => units.map(u => ({
    ...u,
    scannedInPeriod: Boolean(u.scanned) && (!range || inRange(u.scannedAt, range)),
    assembledInPeriod: Boolean(u.assembled) && (!range || inRange(u.assembledAt, range))
  })), [units, range]);
  const stats = useMemo(() => {
    const total = rows.length;
    const wh = rows.filter(u => u.scannedInPeriod).length;
    const mfg = rows.filter(u => u.assembledInPeriod).length;
    return {
      total,
      wh,
      whPending: total - wh,
      mfg,
      mfgPending: total - mfg
    };
  }, [rows]);
  function openDetail(sel) {
    setSelection({
      kind: 'wh',
      status: 'done',
      component: null,
      range: null,
      label: '',
      ...sel
    });
  }
  if (loading) {
    return <div className="mb-6 grid gap-3 lg:grid-cols-2">
        {[0, 1].map(i => <div key={i} className="h-[168px] animate-pulse rounded-2xl border border-solid border-slate-200 bg-white" />)}
      </div>;
  }
  return <section className="mb-6 flex flex-col gap-3">
      <PeriodBar mode={mode} onMode={setMode} anchor={anchor} onAnchor={setAnchor} rangeLabel={rangeLabel} onRefresh={() => load(true)} refreshing={refreshing} />

      {error && <div className="rounded-xl border border-solid border-l-4 border-red-100 border-l-bad-500 bg-bad-50 px-4 py-3 text-sm text-red-900">
          {error}
        </div>}

      <div className="grid gap-3 lg:grid-cols-2">
        <StageStat title="WH" subtitle={`ช่วง ${rangeLabel}`} Icon={QrCodeIcon} doneLabel="สแกนแล้ว" doneValue={stats.wh} pendingLabel="ยังไม่สแกน" pendingValue={stats.whPending} onOpenDone={() => openDetail({
        kind: 'wh',
        status: 'done',
        label: rangeLabel
      })} onOpenPending={() => openDetail({
        kind: 'wh',
        status: 'pending',
        label: rangeLabel
      })} />

        <StageStat title="MFG" subtitle={`ช่วง ${rangeLabel}`} Icon={WrenchScrewdriverIcon} doneLabel="ประกอบแล้ว" doneValue={stats.mfg} pendingLabel="ยังไม่ประกอบ" pendingValue={stats.mfgPending} onOpenDone={() => openDetail({
        kind: 'mfg',
        status: 'done',
        label: rangeLabel
      })} onOpenPending={() => openDetail({
        kind: 'mfg',
        status: 'pending',
        label: rangeLabel
      })} />
      </div>

      {selection && <DetailModal selection={selection} units={rows} onClose={() => setSelection(null)} onSelectionChange={setSelection} />}
    </section>;
}
