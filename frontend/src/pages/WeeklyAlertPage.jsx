import { useCallback, useEffect, useState } from 'react';
import AppShell from '../components/AppShell.jsx';
import { WH_NAV_ITEMS } from './Importlicensepage.jsx';
import { getWeeklyAlertStatus, getWeeklyAlertPreview, sendWeeklyAlertNow } from '../api/weeklyAlert.js';
import { toastError, toastSuccess } from '../lib/toast.js';
import { formatThaiDate } from '../lib/licenseExpiry.js';
import {
  ArrowPathIcon,
  ClockIcon,
  EnvelopeIcon,
  ExclamationTriangleIcon,
  PaperAirplaneIcon,
  XCircleIcon
} from '../components/icons.jsx';
import '../WeeklyAlert.css';

function thaiDateTime(raw) {
  if (!raw) return '—';
  const d = new Date(raw);
  if (Number.isNaN(d.getTime())) return '—';
  const hh = String(d.getHours()).padStart(2, '0');
  const mm = String(d.getMinutes()).padStart(2, '0');
  return `${formatThaiDate(d)} ${hh}:${mm} น.`;
}

const PROVIDER_LABEL = {
  smtp: 'SMTP relay ภายใน → Outlook',
  graph: 'Microsoft Graph API',
  flow: 'Power Automate (Outlook)',
  outlook: 'Outlook บนเครื่อง (Windows)',
  file: 'เขียนลงไฟล์ — ไม่ส่งจริง',
  log: 'โหมดทดสอบ — ไม่ส่งจริง'
};

function InfoRow({ label, value, hint }) {
  return (
    <div className="wa-info-row">
      <span className="wa-info-label">{label}</span>
      <span className="wa-info-value">
        {value}
        {hint && <span className="wa-info-hint">{hint}</span>}
      </span>
    </div>
  );
}

function StatCard({ value, label, tone }) {
  return (
    <div className={`wa-stat wa-stat-${tone}`}>
      <div className="wa-stat-value">{value}</div>
      <div className="wa-stat-label">{label}</div>
    </div>
  );
}

export default function WeeklyAlertPage() {
  const [status, setStatus] = useState(null);
  const [preview, setPreview] = useState(null);
  const [loading, setLoading] = useState(true);
  const [sending, setSending] = useState(false);
  const [testEmail, setTestEmail] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [st, pv] = await Promise.all([getWeeklyAlertStatus(), getWeeklyAlertPreview()]);
      setStatus(st);
      setPreview(pv);
    } catch (err) {
      toastError(err.message || 'โหลดข้อมูลการแจ้งเตือนไม่สำเร็จ');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  async function handleSend() {
    setSending(true);
    try {
      const res = await sendWeeklyAlertNow(testEmail.trim());
      toastSuccess(res?.message || 'ส่งอีเมลเรียบร้อยแล้ว');
      setTestEmail('');
      await load();
    } catch (err) {
      toastError(err.message || 'ส่งอีเมลไม่สำเร็จ');
      if (err.detail) {
        console.error('[weekly-alert]', err.detail);
      }
      await load();
    } finally {
      setSending(false);
    }
  }

  const cfg = status?.config || preview?.config || null;
  const summary = preview?.summary || null;
  const ready = Boolean(cfg?.ready);

  return (
    <AppShell navItems={WH_NAV_ITEMS} roleLabel="Warehouse">
      <div className="wh-heading-row">
        <div>
          <h2 className="wh-title">Weekly Alert</h2>
        </div>

        <div className="wa-heading-actions">
          <button type="button" className="wa-btn wa-btn-ghost" onClick={load} disabled={loading}>
            <ArrowPathIcon className={'size-4' + (loading ? ' wa-spin' : '')} />
            รีเฟรช
          </button>
        </div>
      </div>

      {loading && !cfg && <div className="wa-loading">กำลังโหลดข้อมูล…</div>}

      {cfg && (
        <div className="wa-grid">
          <div className="wa-col">
            <section className="wa-card">
              <header className="wa-card-head">
                <span className="wa-card-icon">
                  <ClockIcon className="size-4" />
                </span>
                <div>
                  <h3 className="wa-card-title">รอบการส่ง</h3>
                </div>
                <span className={cfg.enabled ? 'wa-pill wa-pill-ok' : 'wa-pill wa-pill-muted'}>
                  {cfg.enabled ? 'เปิดใช้งาน' : 'ปิดอยู่'}
                </span>
              </header>

              <div className="wa-card-body">
                <InfoRow label="กำหนดส่ง" value={cfg.schedule} />
                <InfoRow label="รอบถัดไป" value={thaiDateTime(cfg.nextRunAt)} />
                <InfoRow
                  label="สัปดาห์นี้"
                  value={status?.sentThisWeek ? 'ส่งไปแล้ว' : 'ยังไม่ถึงรอบส่ง'}
                  hint={status?.currentWeekKey ? `รอบ ${status.currentWeekKey}` : ''}
                />
              </div>
            </section>

            <section className="wa-card">
              <header className="wa-card-head">
                <span className="wa-card-icon">
                  <EnvelopeIcon className="size-4" />
                </span>
                <div>
                  <h3 className="wa-card-title">ปลายทางและช่องทางส่ง</h3>
                </div>
                <span className={ready ? 'wa-pill wa-pill-ok' : 'wa-pill wa-pill-bad'}>
                  {ready ? 'พร้อมส่ง' : 'ยังตั้งค่าไม่ครบ'}
                </span>
              </header>

              <div className="wa-card-body">
                <InfoRow label="ผู้รับ" value={(cfg.to || []).join(', ') || '—'} />
                {cfg.cc?.length > 0 && <InfoRow label="สำเนาถึง" value={cfg.cc.join(', ')} />}
                <InfoRow label="ผู้ส่ง" value={cfg.from || '—'} hint={cfg.fromName} />
                <InfoRow label="ช่องทาง" value={PROVIDER_LABEL[cfg.provider] || cfg.provider} />
              </div>

              {!ready && (
                <div className="wa-warning">
                  <ExclamationTriangleIcon className="size-4" />
                  <span>{cfg.problem}</span>
                </div>
              )}

              <div className="wa-send-row">
                <input
                  className="wa-input"
                  type="text"
                  value={testEmail}
                  onChange={e => setTestEmail(e.target.value)}
                  placeholder="อีเมล"
                />
                <button type="button" className="wa-btn wa-btn-primary" onClick={handleSend} disabled={sending}>
                  <PaperAirplaneIcon className="size-4" />
                  {sending ? 'กำลังส่ง…' : 'ส่งเดี๋ยวนี้'}
                </button>
              </div>
            </section>
          </div>

          <div className="wa-col">
            <section className="wa-card wa-card-preview">
              <header className="wa-card-head">
                <span className="wa-card-icon">
                  <EnvelopeIcon className="size-4" />
                </span>
                <div>
                  <h3 className="wa-card-title">ตัวอย่างอีเมล</h3>
                </div>
              </header>

              {summary && (
                <div className="wa-stats">
                  <StatCard value={summary.importCounts?.expired + summary.exportCounts?.expired || 0} label="หมดอายุแล้ว" tone="bad" />
                  <StatCard value={summary.importCounts?.expiring + summary.exportCounts?.expiring || 0} label="ใกล้หมดอายุ" tone="warn" />
                  <StatCard value={summary.exportCounts?.leadOverdue || 0} label="เลยกำหนดยื่น" tone="bad" />
                  <StatCard value={summary.exportCounts?.leadDueSoon || 0} label="ใกล้ครบกำหนดยื่น" tone="info" />
                </div>
              )}

              {preview?.subject && (
                <div className="wa-subject">
                  <span className="wa-subject-label">หัวข้อ</span>
                  <span className="wa-subject-text">{preview.subject}</span>
                </div>
              )}

              <div className="wa-preview-frame">
                {preview?.html ? (
                  <iframe
                    title="ตัวอย่างอีเมลแจ้งเตือนรายสัปดาห์"
                    className="wa-iframe"
                    sandbox=""
                    srcDoc={preview.html}
                  />
                ) : (
                  <div className="wa-empty">ยังไม่มีตัวอย่างอีเมล</div>
                )}
              </div>

              {summary && summary.total === 0 && (
                <div className="wa-note">
                  <XCircleIcon className="size-4" />
                  <span>
                    สัปดาห์นี้ไม่มีใบอนุญาตที่ต้องดำเนินการ
                    {cfg.sendWhenEmpty ? ' — ระบบจะยังส่งอีเมลสรุปตามรอบ' : ' — ระบบจะข้ามการส่งรอบนี้'}
                  </span>
                </div>
              )}
            </section>
          </div>
        </div>
      )}
    </AppShell>
  );
}
