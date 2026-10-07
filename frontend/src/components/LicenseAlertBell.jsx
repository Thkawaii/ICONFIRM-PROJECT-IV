import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { getImportLicenseAlerts } from '../api/importLicense.js';
import { useAppNavigate } from '../lib/nav.jsx';
import { formatThaiDate, daysLeftLabel } from '../lib/licenseExpiry.js';
import { addDismissed, clearDismissed, dismissKey, pruneDismissed, readDismissed, removeDismissed } from '../lib/licenseDismiss.js';
import { BellAlertIcon, XMarkIcon, ClockIcon, EyeSlashIcon, ArrowPathIcon, ArrowDownTrayIcon } from './icons.jsx';
const POLL_MS = 60_000;
const ACK_KEY = 'iconfirm_license_alert_ack';
export default function LicenseAlertBell() {
  const navigate = useAppNavigate();
  const [open, setOpen] = useState(false);
  const [items, setItems] = useState([]);
  const [counts, setCounts] = useState({
    alert: 0,
    expired: 0,
    expiring: 0,
    noDate: 0
  });
  const [loaded, setLoaded] = useState(false);
  const [hasNew, setHasNew] = useState(false);
  const [dismissed, setDismissed] = useState(() => readDismissed());
  const [showHidden, setShowHidden] = useState(false);
  const rootRef = useRef(null);
  const load = useCallback(async () => {
    try {
      const data = await getImportLicenseAlerts({
        onlyAlert: true
      });
      const c = data?.counts || {};
      const list = data?.items || [];
      setItems(list);
      setCounts({
        alert: c.alert || 0,
        expired: c.expired || 0,
        expiring: c.expiring || 0,
        noDate: c.noDate || 0
      });
      setDismissed(pruneDismissed(list));
      setLoaded(true);
    } catch {
      setLoaded(true);
    }
  }, []);
  useEffect(() => {
    load();
    const id = setInterval(load, POLL_MS);
    const onFocus = () => load();
    window.addEventListener('focus', onFocus);
    return () => {
      clearInterval(id);
      window.removeEventListener('focus', onFocus);
    };
  }, [load]);
  const {
    hidden,
    vExpired,
    vExpiring,
    visibleAlert
  } = useMemo(() => {
    const isHidden = it => Object.prototype.hasOwnProperty.call(dismissed, dismissKey(it));
    const vis = items.filter(it => !isHidden(it));
    const hid = items.filter(isHidden);
    return {
      hidden: hid,
      vExpired: vis.filter(it => it.Status === 'EXPIRED'),
      vExpiring: vis.filter(it => it.Status === 'EXPIRING'),
      visibleAlert: vis.length
    };
  }, [items, dismissed]);
  useEffect(() => {
    if (!loaded) return;
    const ack = Number(localStorage.getItem(ACK_KEY) || 0);
    setHasNew(visibleAlert > ack);
  }, [loaded, visibleAlert]);
  useEffect(() => {
    if (!open) return;
    function onDown(e) {
      if (rootRef.current && !rootRef.current.contains(e.target)) setOpen(false);
    }
    function onKey(e) {
      if (e.key === 'Escape') setOpen(false);
    }
    document.addEventListener('mousedown', onDown);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onDown);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);
  function toggle() {
    const next = !open;
    setOpen(next);
    if (next) {
      localStorage.setItem(ACK_KEY, String(visibleAlert));
      setHasNew(false);
    } else {
      setShowHidden(false);
    }
  }
  function goToLicense() {
    setOpen(false);
    navigate('/warehouse');
  }
  function handleDismiss(item) {
    const next = addDismissed(item);
    setDismissed({
      ...next
    });
    localStorage.setItem(ACK_KEY, String(Math.max(0, visibleAlert - 1)));
  }
  function handleRestore(item) {
    const next = removeDismissed(item);
    setDismissed({
      ...next
    });
  }
  function handleRestoreAll() {
    setDismissed(clearDismissed());
    setShowHidden(false);
  }
  const badge = visibleAlert;
  const hiddenCount = hidden.length;
  const allDismissed = loaded && items.length > 0 && visibleAlert === 0 && hiddenCount > 0;
  return <div className="lab-root" ref={rootRef}>
      <button type="button" className={'lab-bell' + (badge > 0 ? ' lab-bell-active' : '') + (hasNew ? ' lab-bell-pulse' : '')} onClick={toggle} aria-label={`แจ้งเตือนอายุใบอนุญาตนำเข้า${badge > 0 ? ` (${badge})` : ''}`} title="แจ้งเตือนอายุใบอนุญาตนำเข้า">
        <span className="lab-bell-icon">
          <BellAlertIcon className="size-5" />
        </span>
        <span className="lab-kind lab-kind-import" aria-hidden="true">
          <ArrowDownTrayIcon className="size-3" />
        </span>
        {badge > 0 && <span className="lab-badge">{badge > 99 ? '99+' : badge}</span>}
      </button>

      {open && <div className="lab-panel" role="dialog" aria-label="แจ้งเตือนอายุใบอนุญาต">
          <div className="lab-panel-head">
            <div>
              <h3 className="lab-panel-title">อายุใบอนุญาตนำเข้า</h3>
            </div>
            <button className="lab-panel-close" onClick={() => setOpen(false)} aria-label="ปิด">
              <XMarkIcon className="size-4" />
            </button>
          </div>

          <div className="lab-summary">
            <div className="lab-sum-chip lab-sum-expired">
              <span className="lab-sum-num">{vExpired.length}</span>
              <span className="lab-sum-lbl">หมดอายุแล้ว</span>
            </div>
            <div className="lab-sum-chip lab-sum-expiring">
              <span className="lab-sum-num">{vExpiring.length}</span>
              <span className="lab-sum-lbl">ใกล้หมดอายุ</span>
            </div>
          </div>

          <div className="lab-list">
            {!loaded && <div className="lab-empty">กำลังโหลด...</div>}

            {loaded && badge === 0 && !allDismissed && <div className="lab-empty lab-empty-ok">
                <span className="lab-empty-dot" />
                ทุกใบอนุญาตยังอยู่ในอายุ ไม่มีรายการต้องจัดการ
              </div>}

            {loaded && allDismissed && <div className="lab-empty lab-empty-ok">
                <span className="lab-empty-dot" />
                ซ่อนรายการแจ้งเตือนไว้ทั้งหมดแล้ว
              </div>}

            {vExpired.length > 0 && <>
                <div className="lab-group-label lab-group-expired">หมดอายุแล้ว</div>
                {vExpired.map(it => <AlertItem key={dismissKey(it)} item={it} onOpen={goToLicense} onDismiss={handleDismiss} />)}
              </>}

            {vExpiring.length > 0 && <>
                <div className="lab-group-label lab-group-expiring">ใกล้หมดอายุ (ภายใน 30 วัน)</div>
                {vExpiring.map(it => <AlertItem key={dismissKey(it)} item={it} onOpen={goToLicense} onDismiss={handleDismiss} />)}
              </>}

            {showHidden && hidden.length > 0 && <>
                <div className="lab-group-label lab-group-hidden">ซ่อนไว้</div>
                {hidden.map(it => <AlertItem key={dismissKey(it)} item={it} hidden onOpen={goToLicense} onRestore={handleRestore} />)}
              </>}
          </div>

          {counts.noDate > 0 && <div className="lab-foot-note">
              <ClockIcon className="size-4" />
              ยังไม่ได้ระบุวันที่ออกใบอนุญาต: {counts.noDate}
            </div>}

          {hiddenCount > 0 && <div className="lab-hidden-bar">
              <button className="lab-hidden-toggle" onClick={() => setShowHidden(v => !v)}>
                <EyeSlashIcon className="size-4" />
                {showHidden ? 'ซ่อนรายการที่ซ่อนไว้' : `ดูที่ซ่อนไว้ ${hiddenCount} รายการ`}
              </button>
              <button className="lab-hidden-restore" onClick={handleRestoreAll} title="คืนค่าการแจ้งเตือนทั้งหมด">
                <ArrowPathIcon className="size-4" />
                คืนค่าทั้งหมด
              </button>
            </div>}
        </div>}
    </div>;
}
function AlertItem({
  item,
  hidden = false,
  onOpen,
  onDismiss,
  onRestore
}) {
  const isExpired = item.Status === 'EXPIRED';
  return <div className={'lab-item' + (hidden ? ' lab-item-hidden' : '') + (isExpired ? ' lab-item-expired' : ' lab-item-expiring')}>
      <button className="lab-item-main" onClick={() => onOpen?.(item)}>
        <span className={'lab-item-bar ' + (isExpired ? 'lab-bar-expired' : 'lab-bar-expiring')} />
        <span className="lab-item-body">
          <span className="lab-item-top">
            <span className="lab-item-license">{item.LicenseNo || '—'}</span>
            <span className={'lab-item-days ' + (isExpired ? 'lab-days-expired' : 'lab-days-expiring')}>
              {daysLeftLabel(item.DaysLeft)}
            </span>
          </span>
          <span className="lab-item-meta">
            Invoice {item.InvoiceNo || '—'}
            {item.Model ? ` · ${item.Model}` : ''} · {item.Total} เครื่อง
          </span>
          <span className="lab-item-expiry">หมดอายุ {formatThaiDate(item.ExpiryDate)}</span>
        </span>
      </button>

      {hidden ? <button type="button" className="lab-item-action lab-item-restore" onClick={() => onRestore?.(item)} aria-label="คืนค่าการแจ้งเตือนนี้" title="คืนค่าการแจ้งเตือน">
          <ArrowPathIcon className="size-4" />
        </button> : <button type="button" className="lab-item-action lab-item-dismiss" onClick={() => onDismiss?.(item)} aria-label="ซ่อนการแจ้งเตือนนี้" title="ซ่อนการแจ้งเตือน (ไม่ลบใบอนุญาต)">
          <XMarkIcon className="size-4" />
        </button>}
    </div>;
}
