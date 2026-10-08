import React from 'react';
import ReactDOM from 'react-dom/client';
import LoginPage from './pages/LoginPage.jsx';
import ImportLicensePage from './pages/Importlicensepage.jsx';
import LicenseOverviewPage from './pages/LicenseOverviewPage.jsx';
import WHPartConfirmationPage from './pages/Whpartconfirmationpage.jsx';
import DashboardPage from './pages/Dashboardpage.jsx';
import MasterDataPage from './pages/MasterDataPage.jsx';
import MFGAssemblyPage from './pages/Mfgassemblypage.jsx';
import FormatSettingsPage from './pages/Formatsettingspage.jsx';
import AdminDashboardPage from './pages/AdminDashboardpage.jsx';
import MailRecipientsPage from './pages/Mailrecipientspage.jsx';
import WeeklyAlertPage from './pages/WeeklyAlertPage.jsx';
import QAMachineList from './pages/qa/Qamachinelist.jsx';
import QAMachineDetail from './pages/qa/Qamachinedetail.jsx';
import UiKitPage from './pages/UiKitPage.jsx';
import { NavProvider, useAppNavigate, useAppView } from './lib/nav.jsx';
import { getToken } from './api/client.js';
import { homeRouteForRole } from './lib/roleRoutes.js';
import './styles.css';
import './Warehouse.css';
import './AppShell.css';
import './ImportLicense.css';
import './Filedropzone.css';
import './Dropzone.css';
import './components/Parttag.css';
import './Selectfield.css';
import './theme.css';
// ท้ายสุดเสมอ — ปรับ CSS มือถือเฉพาะหน้า WH / MFG (scope ด้วย .wmx)
import './WhMfgMobile.css';
const resolveHomeRoute = homeRouteForRole;
const ROUTE_CONFIG = {
  '/login': {
    component: LoginPage,
    public: true
  },
  '/warehouse': {
    component: ImportLicensePage,
    roles: ['LOG']
  },
  '/warehouse/license-overview': {
    component: LicenseOverviewPage,
    roles: ['LOG']
  },
  // หน้าทดสอบ/ตรวจสอบการส่งอีเมลแจ้งเตือน — ไม่ได้อยู่ในเมนูของ LOG แล้ว
  // เหลือไว้ให้ ADMIN เปิดจาก URL ตอนต้องตรวจว่าอีเมลส่งออกได้ไหม
  '/warehouse/weekly-alert': {
    component: WeeklyAlertPage,
    roles: ['ADMIN']
  },
  '/warehouse/confirm': {
    component: WHPartConfirmationPage,
    roles: ['WH']
  },
  '/mfg-assembly': {
    component: MFGAssemblyPage,
    roles: ['MFG']
  },
  '/admin': {
    component: AdminDashboardPage,
    roles: ['ADMIN']
  },
  '/master-data': {
    component: MasterDataPage,
    roles: ['UPLOAD', 'ADMIN']
  },
  '/format-settings': {
    component: FormatSettingsPage,
    roles: ['ADMIN']
  },
  '/admin/mail-recipients': {
    component: MailRecipientsPage,
    roles: ['ADMIN']
  },
  '/dashboard': {
    component: DashboardPage,
    roles: null
  },
  '/qa': {
    component: QAMachineList,
    roles: ['QA']
  },
  '/qa/machine': {
    component: QAMachineDetail,
    roles: ['QA']
  },
  '/ui-kit': {
    component: UiKitPage,
    roles: null
  }
};
function resolveEffectiveView(requestedView) {
  const token = getToken();
  const role = (localStorage.getItem('iconfirm_role') || '').toUpperCase();
  if (requestedView === '/login') {
    return token ? resolveHomeRoute(role) : '/login';
  }
  const entry = ROUTE_CONFIG[requestedView];
  if (!entry) {
    return token ? resolveHomeRoute(role) : '/login';
  }
  if (!token) return '/login';
  if (entry.roles && !entry.roles.includes(role)) return resolveHomeRoute(role);
  return requestedView;
}
function AppScreen() {
  const requestedView = useAppView();
  const navigate = useAppNavigate();
  const effectiveView = resolveEffectiveView(requestedView);
  React.useEffect(() => {
    if (effectiveView !== requestedView) {
      navigate(effectiveView);
    }
  }, [effectiveView, requestedView, navigate]);
  const entry = ROUTE_CONFIG[effectiveView] || ROUTE_CONFIG['/login'];
  const Component = entry.component;
  return <Component />;
}
['dragover', 'drop'].forEach(type => {
  window.addEventListener(type, e => {
    if (e.defaultPrevented) return;
    if (!Array.from(e.dataTransfer?.types || []).includes('Files')) return;
    e.preventDefault();
    if (e.type === 'dragover' && e.dataTransfer) e.dataTransfer.dropEffect = 'none';
  });
});
ReactDOM.createRoot(document.getElementById('root')).render(<React.StrictMode>
    <NavProvider initialView={getToken() ? resolveHomeRoute(localStorage.getItem('iconfirm_role')) : '/login'}>
      <AppScreen />
    </NavProvider>
  </React.StrictMode>);
