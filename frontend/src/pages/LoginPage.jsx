import { useState } from 'react';
import { useAppNavigate } from '../lib/nav.jsx';
import { login } from '../api/auth.js';
import { homeRouteForRole } from '../lib/roleRoutes.js';
import { toastSuccess } from '../lib/toast.js';
import { EyeIcon, EyeSlashIcon, LockClosedIcon, ShieldCheckIcon, UserIcon } from '../components/icons.jsx';
import kobelcoLogo from '../assets/brand/kobelco-logo.png';
const resolveRoute = homeRouteForRole;
export default function LoginPage() {
  const navigate = useAppNavigate();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  async function handleSubmit(e) {
    e.preventDefault();
    setError('');
    if (!email && !password) {
      setError('กรุณากรอกชื่อผู้ใช้และรหัสผ่าน');
      return;
    }
    if (!email) {
      setError('กรุณากรอกชื่อผู้ใช้');
      return;
    }
    if (!password) {
      setError('กรุณากรอกรหัสผ่าน');
      return;
    }
    setLoading(true);
    try {
      const data = await login(email.trim(), password);
      toastSuccess('Signed in successfully');
      navigate(resolveRoute(data.role));
    } catch (err) {
      setError(err.message || 'เข้าสู่ระบบไม่สำเร็จ');
    } finally {
      setLoading(false);
    }
  }
  return <div className="auth-page">
      <div className="auth-bg" aria-hidden="true">
        <span className="auth-blob auth-blob-1" />
        <span className="auth-blob auth-blob-2" />
        <span className="auth-grid" />
      </div>

      <div className="auth-header">
        <div className="brand-lockup">
          <img className="brand-logo" src={kobelcoLogo} alt="KOBELCO" draggable="false" />
          <span className="brand-divider" aria-hidden="true" />
          <h1 className="brand-title">
            <ShieldCheckIcon className="shield-icon" />
            <span>I-CONFIRMATION</span>
          </h1>
        </div>
      </div>

      <div className="auth-card">
        <h2 className="card-title">Welcome back</h2>

        <form onSubmit={handleSubmit} noValidate>
          <div className="field">
            <label htmlFor="email">Username</label>
            <div className="input-wrap">
              <UserIcon className="input-icon size-[18px]" />
              <input id="email" name="username" type="text" autoComplete="username" placeholder="Enter your username" value={email} onChange={e => setEmail(e.target.value)} />
            </div>
          </div>

          <div className="field">
            <div className="field-row">
              <label htmlFor="password">Password</label>
            </div>
            <div className="input-wrap password-input">
              <LockClosedIcon className="input-icon size-[18px]" />
              <input id="password" name="password" type={showPassword ? 'text' : 'password'} autoComplete="current-password" placeholder="••••••••" value={password} onChange={e => setPassword(e.target.value)} />
              <button type="button" className="toggle-visibility" onClick={() => setShowPassword(v => !v)} aria-label={showPassword ? 'Hide password' : 'Show password'}>
                {showPassword ? <EyeSlashIcon className="size-[18px]" /> : <EyeIcon className="size-[18px]" />}
              </button>
            </div>
          </div>

          {error && <p className="form-error" role="alert">
              {error}
            </p>}

          <button type="submit" className="submit-btn" disabled={loading}>
            {loading ? <span className="spinner" aria-hidden="true" /> : <LockClosedIcon className="size-4" />}
            {loading ? 'Logging in…' : 'Log in'}
          </button>
        </form>
      </div>
    </div>;
}
