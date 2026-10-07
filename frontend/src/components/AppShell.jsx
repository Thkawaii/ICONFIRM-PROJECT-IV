import { useAppNavigate, useAppView } from '../lib/nav.jsx';
import { logout } from '../api/auth.js';
import { ArrowRightStartOnRectangleIcon } from './icons.jsx';
import kobelcoLogo from '../assets/brand/kobelco-logo-white.png';
const ROLE_LABELS = {
  ADMIN: 'Admin',
  LOG: 'LOG User',
  WH: 'WH User',
  MFG: 'MFG User',
  QA: 'QA User',
  TSF: 'TSF User',
  UPLOAD: 'Upload'
};
export default function AppShell({
  navItems,
  roleLabel,
  children
}) {
  const navigate = useAppNavigate();
  const currentView = useAppView();
  const role = (localStorage.getItem('iconfirm_role') || '').toUpperCase();
  const visibleNav = (navItems || []).filter(item => !item.roles || item.roles.includes(role));
  const shownLabel = ROLE_LABELS[role] || roleLabel || 'User';
  const personName = (localStorage.getItem('iconfirm_name') || '').trim();
  const displayName = personName || shownLabel;
  const initial = (displayName || 'U').trim().charAt(0).toUpperCase() || 'U';
  function handleLogout() {
    logout();
    navigate('/login');
  }
  return <div className="shell">
      <header className="shell-topbar">
        <div className="brand-row">
          <img className="brand-logo-topbar" src={kobelcoLogo} alt="KOBELCO" draggable="false" />
          <span className="brand-divider-topbar" aria-hidden="true" />
          <h1 className="brand-title-sm">I-CONFIRMATION</h1>
        </div>

        <div className="shell-topbar-right">
          <div className="shell-user" title={shownLabel}>
            <span className="shell-avatar">{initial}</span>
            <span className="shell-user-info">
              <span className="shell-user-name">{displayName}</span>
              <span className="shell-user-role">{shownLabel}</span>
            </span>
          </div>
          <button className="shell-logout-btn" onClick={handleLogout}>
            <ArrowRightStartOnRectangleIcon className="size-4" />
            <span>Log out</span>
          </button>
        </div>
      </header>

      {visibleNav.length > 1 && <nav className="shell-subnav" aria-label="เมนูภายในระบบ">
          {visibleNav.map(item => {
        const navLabel = item.labelByRole && item.labelByRole[role] || item.label;
        return <button key={item.to} type="button" onClick={() => navigate(item.to)} className={'shell-subnav-item' + (currentView === item.to ? ' shell-subnav-item-active' : '')}>
                <span className="shell-subnav-icon">{item.icon}</span>
                {navLabel}
              </button>;
      })}
        </nav>}

      <main className="shell-main">{children}</main>
    </div>;
}
