import { useEffect, useMemo, useState } from 'react';
import AppShell from '../components/AppShell.jsx';
import SelectField from '../components/Selectfield.jsx';
import { confirmDelete, toastSuccess, toastError } from '../lib/toast.js';
import { getAdminUsers, createAdminUser, updateAdminUser, deleteAdminUser } from '../api/admin.js';
import { Squares2X2Icon, RectangleStackIcon, CpuChipIcon, ShieldCheckIcon, CubeIcon, PencilSquareIcon, XMarkIcon, FunnelIcon, EnvelopeIcon } from '../components/icons.jsx';
export const ADMIN_NAV_ITEMS = [{
  to: '/admin',
  label: 'User Management',
  icon: <Squares2X2Icon className="size-4" />
}, {
  to: '/admin/mail-recipients',
  label: 'ผู้รับอีเมลแจ้งเตือน',
  icon: <EnvelopeIcon className="size-4" />
}, {
  to: '/master-data',
  label: 'อัพโหลดข้อมูล',
  icon: <RectangleStackIcon className="size-4" />
}, {
  to: '/format-settings',
  label: 'Setting',
  icon: <Squares2X2Icon className="size-4" />
}];
const ROLE_OPTIONS = [{
  value: 'WH',
  label: 'WH — คลัง (Part Confirmation)'
}, {
  value: 'MFG',
  label: 'MFG — ฝ่ายผลิต/ประกอบ'
}, {
  value: 'LOG',
  label: 'LOG — Logistic (ใบอนุญาต)'
}, {
  value: 'QA',
  label: 'QA — ตรวจสอบคุณภาพ'
}, {
  value: 'UPLOAD',
  label: 'UPLOAD — อัพโหลดข้อมูล'
}, {
  value: 'ADMIN',
  label: 'ADMIN — ผู้ดูแลระบบ'
}];
const ROLE_BADGE = {
  WH: {
    label: 'WH',
    bg: '#dbeafe',
    color: '#1e40af'
  },
  MFG: {
    label: 'MFG',
    bg: '#dcfce7',
    color: '#166534'
  },
  LOG: {
    label: 'LOG',
    bg: '#fef3c7',
    color: '#92400e'
  },
  QA: {
    label: 'QA',
    bg: '#ede9fe',
    color: '#5b21b6'
  },
  TSF: {
    label: 'TSF',
    bg: '#e0f2fe',
    color: '#075985'
  },
  UPLOAD: {
    label: 'UPLOAD',
    bg: '#f1f5f9',
    color: '#475569'
  },
  ADMIN: {
    label: 'ADMIN',
    bg: '#fee2e2',
    color: '#991b1b'
  }
};
function RoleBadge({
  role
}) {
  const m = ROLE_BADGE[role] || {
    label: role || '—',
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
      {m.label}
    </span>;
}
export default function AdminDashboardPage() {
  const [users, setUsers] = useState([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('ALL');
  const [search, setSearch] = useState('');
  const [editUser, setEditUser] = useState(null);
  async function load() {
    setLoading(true);
    try {
      const data = await getAdminUsers();
      setUsers(Array.isArray(data) ? data : []);
    } catch (err) {
      toastError(err.message || 'โหลดรายชื่อผู้ใช้ไม่สำเร็จ');
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => {
    load();
  }, []);
  const counts = useMemo(() => {
    const c = {
      total: users.length,
      WH: 0,
      MFG: 0,
      other: 0
    };
    for (const u of users) {
      if (u.role_name === 'WH') c.WH++;else if (u.role_name === 'MFG') c.MFG++;else c.other++;
    }
    return c;
  }, [users]);
  const filtered = useMemo(() => {
    let list = users;
    if (filter !== 'ALL') list = list.filter(u => u.role_name === filter);
    const kw = search.trim().toLowerCase();
    if (kw) list = list.filter(u => (u.name || '').toLowerCase().includes(kw) || (u.username || '').toLowerCase().includes(kw));
    return list;
  }, [users, filter, search]);
  async function handleDelete(u) {
    const ok = await confirmDelete({
      text: `ลบผู้ใช้ "${u.name}" ออกจากระบบ?`,
      confirmText: 'ลบ'
    });
    if (!ok) return;
    try {
      const res = await deleteAdminUser(u.id);
      toastSuccess(res?.archived ? 'ลบผู้ใช้แล้ว — เก็บประวัติการใช้งานไว้' : 'ลบผู้ใช้แล้ว');
      await load();
    } catch (err) {
      toastError(err.message || 'ลบไม่สำเร็จ');
    }
  }
  const stat = (icon, label, value, accent) => <div style={{
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
        {icon}
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
        }}>User Management</h1>
        </div>
        <button className="wh-issue-btn" onClick={() => setEditUser({})}>
          + เพิ่มผู้ใช้
        </button>
      </div>

      <div style={{
      display: 'grid',
      gridTemplateColumns: 'repeat(auto-fit, minmax(190px, 1fr))',
      gap: 14,
      margin: '10px 0 22px'
    }}>
        {stat(<Squares2X2Icon className="size-6" />, 'ผู้ใช้ทั้งหมด', counts.total, {
        bg: '#eef2ff',
        color: '#4338ca'
      })}
        {stat(<RectangleStackIcon className="size-6" />, 'พนักงาน WH (คลัง)', counts.WH, {
        bg: '#dbeafe',
        color: '#1e40af'
      })}
        {stat(<CpuChipIcon className="size-6" />, 'พนักงาน MFG (ผลิต)', counts.MFG, {
        bg: '#dcfce7',
        color: '#166534'
      })}
        {stat(<ShieldCheckIcon className="size-6" />, 'บทบาทอื่น ๆ', counts.other, {
        bg: '#f1f5f9',
        color: '#475569'
      })}
      </div>

      <div style={{
      display: 'flex',
      gap: 10,
      flexWrap: 'wrap',
      alignItems: 'center',
      marginBottom: 12
    }}>
        <div style={{
        display: 'flex',
        background: '#f1f5f9',
        borderRadius: 10,
        padding: 3,
        minWidth: 0,
        maxWidth: '100%',
        overflowX: 'auto',
        WebkitOverflowScrolling: 'touch'
      }}>
          {[{
          v: 'ALL',
          label: 'ทั้งหมด'
        }, {
          v: 'ADMIN',
          label: 'Admin'
        }, {
          v: 'LOG',
          label: 'LOG'
        }, {
          v: 'WH',
          label: 'WH'
        }, {
          v: 'MFG',
          label: 'MFG'
        }, {
          v: 'QA',
          label: 'QA'
        }, {
          v: 'UPLOAD',
          label: 'Upload'
        }].map(t => <button key={t.v} onClick={() => setFilter(t.v)} style={{
          border: 'none',
          borderRadius: 8,
          padding: '7px 16px',
          fontSize: 13,
          fontWeight: 600,
          cursor: 'pointer',
          whiteSpace: 'nowrap',
          flexShrink: 0,
          background: filter === t.v ? '#ffffff' : 'transparent',
          color: filter === t.v ? '#0f172a' : '#64748b',
          boxShadow: filter === t.v ? '0 1px 2px rgba(15,23,42,0.08)' : 'none'
        }}>
              {t.label}
            </button>)}
        </div>
        <div style={{
        position: 'relative',
        flex: '1 1 240px',
        maxWidth: 360
      }}>
          <span style={{
          position: 'absolute',
          left: 10,
          top: '50%',
          transform: 'translateY(-50%)',
          color: '#94a3b8'
        }}>
            <FunnelIcon className="size-4" />
          </span>
          <input className="wh-search" style={{
          paddingLeft: 32,
          width: '100%'
        }} placeholder="ค้นหาชื่อพนักงาน / username" value={search} onChange={e => setSearch(e.target.value)} />
        </div>
        <span style={{
        fontSize: 13,
        color: '#94a3b8'
      }}>{filtered.length} คน</span>
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
                <th>ชื่อ - นามสกุล</th>
                <th>Username</th>
                <th>อีเมล</th>
                <th>แผนก / Role</th>
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
                  <td colSpan={7} className="wh-empty-cell">ไม่พบผู้ใช้ตามเงื่อนไข</td>
                </tr>}
              {!loading && filtered.map((u, i) => <tr key={u.id}>
                    <td className="wh-cell-head" data-label="#">
                      <strong>{i + 1}</strong>
                    </td>
                    <td data-label="ชื่อ - นามสกุล" style={{
                fontWeight: 600
              }}>{u.name}</td>
                    <td data-label="Username" style={{
                fontFamily: 'ui-monospace, Menlo, monospace',
                color: '#475569'
              }}>{u.username}</td>
                    <td data-label="อีเมล" style={{
                color: '#475569'
              }}>
                      {u.email ? <span>
                          {u.email}
                          {u.receives_alerts && <span title="รับรายงานใบอนุญาตทางอีเมล" style={{
                    marginLeft: 6,
                    fontSize: 11,
                    fontWeight: 600,
                    color: '#0f766e'
                  }}>● รับรายงาน</span>}
                        </span> : <span style={{
                  color: '#cbd5e1'
                }}>—</span>}
                    </td>
                    <td data-label="แผนก / Role"><RoleBadge role={u.role_name} /></td>
                    <td data-label="สถานะ">
                      <span style={{
                  fontSize: 12,
                  fontWeight: 600,
                  color: u.status === 'Active' ? '#166534' : '#b91c1c'
                }}>
                        {u.status === 'Active' ? '● ใช้งาน' : '● ปิดใช้งาน'}
                      </span>
                    </td>
                    <td className="wh-cell-action">
                      <div style={{
                  display: 'flex',
                  gap: 6,
                  justifyContent: 'flex-end'
                }}>
                        <button className="wh-issue-btn" onClick={() => setEditUser(u)}>
                          <PencilSquareIcon className="size-4" /> แก้ไข
                        </button>
                        <button className="qa-fail-btn" onClick={() => handleDelete(u)}>ลบ</button>
                      </div>
                    </td>
                  </tr>)}
            </tbody>
          </table>
        </div>
      </div>

      {editUser && <UserModal user={editUser} onClose={() => setEditUser(null)} onSaved={async () => {
      setEditUser(null);
      await load();
    }} />}
    </AppShell>;
}
function UserModal({
  user,
  onClose,
  onSaved
}) {
  const isEdit = !!user.id;
  const [form, setForm] = useState({
    name: user.name || '',
    username: user.username || '',
    password: '',
    role_name: user.role_name || 'WH',
    status: user.status || 'Active',
    email: user.email || '',
    receive_alerts: !!user.receives_alerts
  });
  const [saving, setSaving] = useState(false);
  const set = k => e => setForm(f => ({
    ...f,
    [k]: e.target.value
  }));
  async function handleSave() {
    if (!form.name.trim() || !form.username.trim() || !form.role_name) {
      toastError('กรอกชื่อ, username และแผนกให้ครบ');
      return;
    }
    if (!isEdit && !form.password.trim()) {
      toastError('กรุณาตั้งรหัสผ่านสำหรับผู้ใช้ใหม่');
      return;
    }
    const email = form.email.trim().toLowerCase();
    if (form.receive_alerts && !email) {
      toastError('ติ๊กรับรายงานทางอีเมลไว้ แต่ยังไม่ได้กรอกอีเมล');
      return;
    }
    if (email && !email.includes('@')) {
      toastError('รูปแบบอีเมลไม่ถูกต้อง');
      return;
    }
    setSaving(true);
    try {
      let saved;
      if (isEdit) {
        const patch = {
          name: form.name.trim(),
          role_name: form.role_name,
          status: form.status,
          email,
          receive_alerts: form.receive_alerts
        };
        if (form.password.trim()) patch.password = form.password.trim();
        saved = await updateAdminUser(user.id, patch);
        toastSuccess('บันทึกการแก้ไขแล้ว');
      } else {
        saved = await createAdminUser({
          name: form.name.trim(),
          username: form.username.trim(),
          password: form.password.trim(),
          role_name: form.role_name,
          status: form.status,
          email,
          receive_alerts: form.receive_alerts
        });
        toastSuccess('เพิ่มผู้ใช้แล้ว');
      }
      // บอกผู้ดูแลให้ชัดว่ารายงานฉบับล่าสุดถูกส่งออกไปแล้วหรือยัง
      // ไม่งั้นจะไม่มีทางรู้เลยว่าคนใหม่ได้รับอะไรไปบ้าง
      if (saved && saved.welcome_mail === 'queued') {
        toastSuccess(`ส่งรายงานใบอนุญาตฉบับล่าสุดไปที่ ${email} แล้ว`);
      } else if (saved && saved.welcome_mail === 'disabled') {
        toastSuccess('เพิ่มเข้ารายชื่อผู้รับแล้ว — ระบบจะส่งให้ในรอบแจ้งเตือนถัดไป');
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
      maxWidth: 460
    }} onClick={e => e.stopPropagation()}>
        <div style={{
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center'
      }}>
          <h3 className="wh-modal-title">{isEdit ? 'แก้ไขผู้ใช้' : 'เพิ่มผู้ใช้ใหม่'}</h3>
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
            <label style={labelStyle}>ชื่อ - นามสกุล</label>
            <input style={inputStyle} value={form.name} onChange={set('name')} placeholder="" />
          </div>
          <div>
            <label style={labelStyle}>
              Username {isEdit && <span style={{
              color: '#94a3b8'
            }}>(แก้ไม่ได้)</span>}
            </label>
            <input style={{
            ...inputStyle,
            ...(isEdit ? {
              background: '#f8fafc',
              color: '#94a3b8'
            } : {})
          }} value={form.username} onChange={set('username')} placeholder="" disabled={isEdit} />
          </div>
          <div>
            <label style={labelStyle}>รหัสผ่าน {isEdit && <span style={{
              color: '#94a3b8'
            }}>(เว้นว่าง = ไม่เปลี่ยน)</span>}</label>
            <input style={inputStyle} type="text" value={form.password} onChange={set('password')} placeholder={isEdit ? '••••••' : ''} />
          </div>
          <div>
            <label style={labelStyle}>แผนก / Role</label>
            <SelectField value={form.role_name} onChange={v => setForm(f => ({
            ...f,
            role_name: v
          }))} options={ROLE_OPTIONS} />
          </div>
          <div>
            <label style={labelStyle}>สถานะ</label>
            <SelectField value={form.status} onChange={v => setForm(f => ({
            ...f,
            status: v
          }))} options={[{
            value: 'Active',
            label: 'ใช้งาน'
          }, {
            value: 'Inactive',
            label: 'ปิดใช้งาน'
          }]} />
          </div>
          <div>
            <label style={labelStyle}>อีเมล <span style={{
              color: '#94a3b8'
            }}>(ไม่บังคับ)</span></label>
            <input style={inputStyle} type="email" value={form.email} onChange={set('email')} placeholder="name@kobelco.com" />
          </div>
          <label style={{
          display: 'flex',
          alignItems: 'flex-start',
          gap: 9,
          fontSize: 13,
          color: '#334155',
          cursor: 'pointer'
        }}>
            <input type="checkbox" style={{
            marginTop: 2
          }} checked={form.receive_alerts} onChange={e => setForm(f => ({
            ...f,
            receive_alerts: e.target.checked
          }))} />
            <span>
              รับรายงานสถานะใบอนุญาตทางอีเมล
              <span style={{
              display: 'block',
              color: '#94a3b8',
              fontSize: 12,
              marginTop: 2
            }}>
                เพิ่มเข้ารายชื่อผู้รับให้อัตโนมัติ และส่งรายงานฉบับล่าสุดให้ทันที
              </span>
            </span>
          </label>
        </div>

        <div className="wh-modal-actions">
          <button className="wh-modal-cancel" onClick={onClose} disabled={saving}>ยกเลิก</button>
          <button className="wh-issue-btn" onClick={handleSave} disabled={saving}>
            {saving ? 'กำลังบันทึก...' : isEdit ? 'บันทึก' : 'เพิ่มผู้ใช้'}
          </button>
        </div>
      </div>
    </div>;
}

