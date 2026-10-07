import { useEffect, useState } from 'react';
function dayKey(d = new Date()) {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  return `${y}-${m}-${day}`;
}
function msUntilNextMidnight() {
  const now = new Date();
  const next = new Date(now.getFullYear(), now.getMonth(), now.getDate() + 1, 0, 0, 0, 0);
  return next.getTime() - now.getTime() + 1000;
}
export function useDailyTick() {
  const [today, setToday] = useState(() => dayKey());
  useEffect(() => {
    let timer;
    const sync = () => setToday(prev => prev === dayKey() ? prev : dayKey());
    const schedule = () => {
      clearTimeout(timer);
      timer = setTimeout(() => {
        sync();
        schedule();
      }, msUntilNextMidnight());
    };
    schedule();
    const onWake = () => {
      if (document.visibilityState === 'visible') sync();
    };
    window.addEventListener('focus', onWake);
    document.addEventListener('visibilitychange', onWake);
    return () => {
      clearTimeout(timer);
      window.removeEventListener('focus', onWake);
      document.removeEventListener('visibilitychange', onWake);
    };
  }, []);
  return today;
}
