export function homeRouteForRole(role) {
  const normalized = (role || '').toUpperCase();
  if (normalized === 'ADMIN') return '/admin';
  if (normalized === 'LOG') return '/warehouse/license-overview';
  if (normalized === 'WH') return '/warehouse/confirm';
  if (normalized === 'MFG') return '/mfg-assembly';
  if (normalized === 'QA') return '/qa';
  if (normalized === 'UPLOAD') return '/master-data';
  return '/dashboard';
}