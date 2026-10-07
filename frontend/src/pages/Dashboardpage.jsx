import { useAppNavigate } from '../lib/nav.jsx';
import kobelcoLogo from '../assets/brand/kobelco-logo-white.png';
export default function DashboardPage() {
  const navigate = useAppNavigate();
  function handleLogout() {
    localStorage.removeItem('iconfirm_token');
    localStorage.removeItem('iconfirm_role');
    navigate('/login');
  }
  return <div className="wh-page">
      <header className="wh-topbar">
        <div className="brand-row">
          <img className="brand-logo-topbar" src={kobelcoLogo} alt="KOBELCO" draggable="false" />
          <span className="brand-divider-topbar" aria-hidden="true" />
          <h1 className="brand-title-sm">I-CONFIRMATION</h1>
        </div>
        <button className="wh-logout-btn" onClick={handleLogout}>
          Logout
        </button>
      </header>

      <main className="wh-main">
        <h2 className="wh-title">Dashboard</h2>
        <p className="wh-subtitle">
          หน้านี้เป็น placeholder — ยังไม่ได้ทำสำหรับ role นี้ (จะทำต่อในสเต็ปถัดไป เช่น QA / TSF Operator)
        </p>
      </main>
    </div>;
}
