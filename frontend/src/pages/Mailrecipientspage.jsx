import { useEffect, useMemo, useState } from 'react';
import AppShell from '../components/AppShell.jsx';
import SelectField from '../components/Selectfield.jsx';
import { confirmDelete, toastSuccess, toastError } from '../lib/toast.js';
import { ADMIN_NAV_ITEMS } from './AdminDashboardpage.jsx';
import { getMailRecipients, createMailRecipient, updateMailRecipient, deleteMailRecipient } from '../api/admin.js';
import { EnvelopeIcon, PaperAirplaneIcon, XMarkIcon } from '../components/icons.jsx';

const KIND_OPTIONS = [{
  value: 'TO',
  label: 'TO — ผู้รับหลัก'
}, {
  value: 'CC',
  label: 'CC — สำเนา'
}, {
  value: 'BCC',
  label: 'BCC — สำเนาลับ'
}];

const KIND_BADGE = {
  TO: {
    bg: '#dbeafe',
    color: '#1e40af'
  },
  CC: {
    bg: '#fef3c7',
    color: '#92400e'
  },
  BCC: {
    bg: '#ede9fe',
    color: '#5b21b6'
  }
};

function KindBadge({
  kind
}) {
  const m = KIND_BADGE[kind] || {
    bg: '#f1f5f9',
    color: '#475569'
  };
  return <span style={{
    background: m.bg,
    color: m.color,
    borderRadius: 999,
    padding: '2px 10px',
    fontSize: 12,
    fontWeight: 700
  }}>
      {kind}
    </span>;
}

export default function MailRecipientsPage() {
  const [rows, setRows] = useState([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('ALL');
  const [showAdd, setShowAdd] = useState(false);

  async function load() {
    setLoading(true);
    try {
      const data = await getMailRecipients();
      setRows(Array.isArray(data) ? data : []);
    } catch (err) {
      toastError(err.message || 'โหลดรายชื่อผู้รับอีเมลไม่สำเร็จ');
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => {
    load();
  }, []);

  const counts = useMemo(() => {
    const c = {
      total: rows.length,
      TO: 0,
      CC: 0,
      BCC: 0
    };
    for (const r of rows) {
      if (c[r.kind] !== undefined) c[r.kind]++;
    }
    return c;
  }, [rows]);

  const filtered = useMemo(() => filter === 'ALL' ? rows : rows.filter(r => r.kind === filter), [rows, filter]);

  async function toggleActive(r) {
    try {
      await updateMailRecipient(r.id, {
        active: !r.active
      });
      await load();
    } catch (err) {
      toastError(err.message || 'แก้ไขไม่สำเร็จ');
    }
  }

  async function handleDelete(r) {
    const ok = await confirmDelete({
      text: `ลบ "${r.email}" ออกจากรายชื่อผู้รับ ${r.kind}?`,
      confirmText: 'ลบ'
    });
    if (!ok) return;
    try {
      await deleteMailRecipient(r.id);
      toastSuccess('ลบรายชื่อแล้ว');
      await load();
    } catch (err) {
      toastError(err.message || 'ลบไม่สำเร็จ');
    }
  }

  const stat = (label, value, accent) => <div style={{
    background: '#fff',
    border: '1px solid #e5e9f0',
    borderRadius: 16,
    padding: '18px 20px',
    display: 'flex',
    alignItems: 'center',
    gap: 14,
    boxShadow: '0 1px 2px rgba(15,23,42,0.04)'
  }}>
      <span style={{
      width: 46,
      height: 46,
      borderRadius: 12,
      background: accent.bg,
      color: accent.color,
      display: 'inline-flex',
      alignItems: 'center',
      justifyContent: 'center',
      flexShrink: 0
    }}>
        <EnvelopeIcon className="size-6" />
      </span>
      <div>
        <div style={{
        fontSize: 28,
        fontWeight: 100,
        lineHeight: 1,
        color: '#0f172a'
      }}>{value}</div>
        <div style={{
        fontSize: 13,
        color: '#64748b',
        marginTop: 4
      }}>{label}</div>
      </div>
    </div>;

  return <AppShell navItems={ADMIN_NAV_ITEMS} roleLabel="Admin">
      <div className="wh-heading-row" style={{
      marginBottom: 4
    }}>
        <div>
          <h1 className="wh-title" style={{
          fontSize: 24
        }}>ผู้รับอีเมลแจ้งเตือน</h1>
          <p style={{
          fontSize: 13,
          color: '#64748b',
          margin: '4px 0 0'
        }}>

            <br />
          </p>
        </div>
        <button className="wh-issue-btn" onClick={() => setShowAdd(true)}>
          + เพิ่มผู้รับ
        </button>
      </div>

      <div style={{
      display: 'grid',
      gridTemplateColumns: 'repeat(auto-fit, minmax(190px, 1fr))',
      gap: 14,
      margin: '10px 0 22px'
    }}>
        {stat('ผู้รับทั้งหมด', counts.total, {
        bg: '#eef2ff',
        color: '#4338ca'
      })}
        {stat('TO — ผู้รับหลัก', counts.TO, {
        bg: '#dbeafe',
        color: '#1e40af'
      })}
        {stat('CC — สำเนา', counts.CC, {
        bg: '#fef3c7',
        color: '#92400e'
      })}
        {stat('BCC — สำเนาลับ', counts.BCC, {
        bg: '#ede9fe',
        color: '#5b21b6'
      })}
      </div>

      <div style={{
      display: 'flex',
      background: '#f1f5f9',
      borderRadius: 10,
      padding: 3,
      width: 'fit-content',
      marginBottom: 12
    }}>
        {[{
        v: 'ALL',
        label: 'ทั้งหมด'
      }, {
        v: 'TO',
        label: 'TO'
      }, {
        v: 'CC',
        label: 'CC'
      }, {
        v: 'BCC',
        label: 'BCC'
      }].map(t => <button key={t.v} onClick={() => setFilter(t.v)} style={{
        border: 'none',
        borderRadius: 8,
        padding: '7px 16px',
        fontSize: 13,
        fontWeight: 600,
        cursor: 'pointer',
        background: filter === t.v ? '#ffffff' : 'transparent',
        color: filter === t.v ? '#0f172a' : '#64748b',
        boxShadow: filter === t.v ? '0 1px 2px rgba(15,23,42,0.08)' : 'none'
      }}>
            {t.label}
          </button>)}
      </div>

      <div className="wh-table-card">
        <div style={{
        overflowX: 'auto'
      }}>
          <table className="wh-table" style={{
          width: '100%'
        }}>
            <thead>
              <tr>
                <th style={{
                width: 54
              }}>#</th>
                <th>อีเมล</th>
                <th>ชื่อนามสกุล</th>
                <th>ประเภท</th>
                <th>หมายเหตุ</th>
                <th>สถานะ</th>
                <th style={{
                textAlign: 'right'
              }}></th>
              </tr>
            </thead>
            <tbody>
              {loading && <tr>
                  <td colSpan={7} className="wh-empty-cell">กำลังโหลด...</td>
                </tr>}
              {!loading && filtered.length === 0 && <tr>
                  <td colSpan={7} className="wh-empty-cell">ยังไม่มีรายชื่อผู้รับ — กด "เพิ่มผู้รับ" เพื่อเริ่มต้น</td>
                </tr>}
              {!loading && filtered.map((r, i) => <tr key={r.id}>
                    <td className="wh-cell-head" data-label="#">
                      <strong>{i + 1}</strong>
                    </td>
                    <td data-label="อีเมล" style={{
                fontFamily: 'ui-monospace, Menlo, monospace',
                fontWeight: 600
              }}>{r.email}</td>
                    <td data-label="ชื่อนามสกุล">{r.name || '—'}</td>
                    <td data-label="ประเภท"><KindBadge kind={r.kind} /></td>
                    <td data-label="หมายเหตุ" style={{
                color: '#64748b'
              }}>{r.note || '—'}</td>
                    <td data-label="สถานะ">
                      <button onClick={() => toggleActive(r)} style={{
                  border: 'none',
                  background: 'none',
                  cursor: 'pointer',
                  fontSize: 12,
                  fontWeight: 600,
                  color: r.active ? '#166534' : '#b91c1c',
                  padding: 0
                }}>
                        {r.active ? '● ใช้งาน' : '● ปิดใช้งาน'}
                      </button>
                    </td>
                    <td className="wh-cell-action">
                      <div style={{
                  display: 'flex',
                  gap: 6,
                  justifyContent: 'flex-end'
                }}>
                        <button className="qa-fail-btn" onClick={() => handleDelete(r)}>ลบ</button>
                      </div>
                    </td>
                  </tr>)}
            </tbody>
          </table>
        </div>
      </div>

      {showAdd && <AddRecipientModal onClose={() => setShowAdd(false)} onSaved={async () => {
      setShowAdd(false);
      await load();
    }} />}
    </AppShell>;
}

function AddRecipientModal({
  onClose,
  onSaved
}) {
  const [form, setForm] = useState({
    email: '',
    kind: 'TO',
    name: '',
    note: ''
  });
  const [saving, setSaving] = useState(false);
  const set = k => e => setForm(f => ({
    ...f,
    [k]: e.target.value
  }));

  async function handleSave() {
    const email = form.email.trim();
    if (!email || !email.includes('@')) {
      toastError('กรุณากรอกอีเมลให้ถูกต้อง');
      return;
    }
    setSaving(true);
    try {
      const res = await createMailRecipient({
        email,
        kind: form.kind,
        name: form.name.trim(),
        note: form.note.trim()
      });
      if (res?.welcome_mail === 'queued') {
        toastSuccess(`เพิ่มผู้รับแล้ว — กำลังส่งอีเมลแจ้งเตือนฉบับล่าสุดให้ ${email}`);
      } else {
        toastSuccess('เพิ่มผู้รับแล้ว');
      }
      onSaved();
    } catch (err) {
      toastError(err.message || 'บันทึกไม่สำเร็จ');
    } finally {
      setSaving(false);
    }
  }

  const labelStyle = {
    fontSize: 12,
    color: '#64748b',
    marginBottom: 4,
    display: 'block'
  };
  const inputStyle = {
    border: '1px solid #cbd5e1',
    borderRadius: 8,
    padding: '9px 11px',
    fontSize: 14,
    width: '100%'
  };

  return <div className="wh-modal-overlay" onClick={onClose}>
      <div className="wh-modal" style={{
      maxWidth: 440
    }} onClick={e => e.stopPropagation()}>
        <div style={{
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center'
      }}>
          <h3 className="wh-modal-title">
            <PaperAirplaneIcon className="size-4" style={{
            marginRight: 6
          }} />
            เพิ่มผู้รับอีเมลแจ้งเตือน
          </h3>
          <button className="lab-panel-close" onClick={onClose} aria-label="ปิด">
            <XMarkIcon className="size-4" />
          </button>
        </div>

        <div style={{
        display: 'flex',
        flexDirection: 'column',
        gap: 12,
        marginTop: 10
      }}>
          <div>
            <label style={labelStyle}>อีเมล</label>
            <input style={inputStyle} value={form.email} onChange={set('email')} placeholder="name@kobelco.com" autoFocus />
          </div>
          <div>
            <label style={labelStyle}>ประเภทผู้รับ</label>
            <SelectField value={form.kind} onChange={v => setForm(f => ({
            ...f,
            kind: v
          }))} options={KIND_OPTIONS} />
          </div>
          {form.kind === 'TO' && <div>
            <label style={labelStyle}>ชื่อนามสกุล (ไม่บังคับ)</label>
            <input style={inputStyle} value={form.name} onChange={set('name')} placeholder="เช่น Sarai Promden" />
            <p style={{
            fontSize: 12,
            color: '#94a3b8',
            margin: '4px 0 0'
          }}>
            </p>
          </div>}
          <div>
            <label style={labelStyle}>หมายเหตุ (ไม่บังคับ)</label>
            <input style={inputStyle} value={form.note} onChange={set('note')} placeholder="เช่น ชื่อ หรือ แผนก" />
          </div>
        </div>

        <div className="wh-modal-actions">
          <button className="wh-modal-cancel" onClick={onClose} disabled={saving}>ยกเลิก</button>
          <button className="wh-issue-btn" onClick={handleSave} disabled={saving}>
            {saving ? 'กำลังบันทึก...' : 'เพิ่มผู้รับ'}
          </button>
        </div>
      </div>
    </div>;
}
