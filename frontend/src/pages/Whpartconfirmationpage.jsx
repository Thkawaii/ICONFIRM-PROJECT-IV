import { useEffect, useMemo, useState } from 'react';
import { getImportLicenseItems } from '../api/importLicense.js';
import { CheckIcon, ChevronDoubleLeftIcon, ChevronDoubleRightIcon, ChevronLeftIcon, ChevronRightIcon, ClockIcon } from '../components/icons.jsx';
import AppShell from '../components/AppShell.jsx';
import SelectField from '../components/Selectfield.jsx';
import WHIssuePanel from '../components/WHIssuePanel.jsx';
import { WH_NAV_ITEMS } from './Importlicensepage.jsx';

// หน้านี้มี 2 มุมมอง
//   LOG : ตารางเทียบกับบัญชีใบอนุญาตนำเข้า (ของเดิม)
//   WH  : ขั้นตอนจ่ายของแบบใหม่ — เลือก MC# แล้วสแกนของทีละชิ้น (ดู WHIssuePanel)
export default function WHPartConfirmationPage() {
  const isManager = (localStorage.getItem('iconfirm_role') || '').toUpperCase() === 'LOG';
  const [loading, setLoading] = useState(isManager);
  const [loadError, setLoadError] = useState('');
  const [licenseItems, setLicenseItems] = useState([]);
  const [licenseTab, setLicenseTab] = useState('all');
  const [licenseModel, setLicenseModel] = useState('all');
  const [licenseNo, setLicenseNo] = useState('all');
  const [licensePageSize, setLicensePageSize] = useState(10);
  const [licensePage, setLicensePage] = useState(1);
  const [highlightId] = useState(null);
  useEffect(() => {
    if (!isManager) return;
    let cancelled = false;
    (async () => {
      setLoading(true);
      setLoadError('');
      try {
        const items = await getImportLicenseItems();
        if (!cancelled) setLicenseItems(items || []);
      } catch (err) {
        if (!cancelled) setLoadError(err.message || 'โหลดข้อมูลไม่สำเร็จ');
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [isManager]);
  useEffect(() => {
    setLicensePage(1);
  }, [licenseTab, licenseModel, licenseNo, licensePageSize]);
  const licenseRows = useMemo(() => {
    let list = licenseItems;
    if (licenseTab === 'pending') list = list.filter(r => r.ConfirmStatus !== 'CONFIRMED');
    if (licenseTab === 'confirmed') list = list.filter(r => r.ConfirmStatus === 'CONFIRMED');
    if (licenseModel !== 'all') list = list.filter(r => (r.Model || '') === licenseModel);
    if (licenseNo !== 'all') list = list.filter(r => (r.LicenseNo || '') === licenseNo);
    return list;
  }, [licenseItems, licenseTab, licenseModel, licenseNo]);
  const licenseTotalPages = Math.max(1, Math.ceil(licenseRows.length / licensePageSize));
  const licensePaged = licenseRows.slice((licensePage - 1) * licensePageSize, licensePage * licensePageSize);
  function goToLicensePage(p) {
    setLicensePage(Math.min(Math.max(1, p), licenseTotalPages));
  }
  const licenseModelOptions = useMemo(() => {
    const set = new Set();
    licenseItems.forEach(r => {
      if (r.Model) set.add(r.Model);
    });
    return Array.from(set).sort();
  }, [licenseItems]);
  const licenseNoOptions = useMemo(() => {
    const set = new Set();
    licenseItems.forEach(r => {
      if (r.LicenseNo) set.add(r.LicenseNo);
    });
    return Array.from(set).sort();
  }, [licenseItems]);
  const licenseCounts = useMemo(() => {
    return {
      total: licenseItems.length,
      confirmed: licenseItems.filter(r => r.ConfirmStatus === 'CONFIRMED').length,
      pending: licenseItems.filter(r => r.ConfirmStatus !== 'CONFIRMED').length
    };
  }, [licenseItems]);
  return <AppShell navItems={WH_NAV_ITEMS} roleLabel="Warehouse">
      <div className="wh-heading-row">
        <div>
          <h2 className="wh-title">{isManager ? 'Part Checklist' : 'Part Confirmation'}</h2>
        </div>
      </div>

      {loadError && <p className="form-error" role="alert">
          {loadError}
        </p>}

      {!isManager && <WHIssuePanel />}

      {isManager && <>
      <div className="wh-heading-row">
        <div>
          <h2 className="wh-title" style={{
            fontSize: 19
          }}>
            เทียบกับบัญชีใบอนุญาตนำเข้า ({licenseCounts.confirmed}/{licenseCounts.total})
          </h2>
        </div>
        <div className="vr-tabs">
          {[{
            key: 'all',
            label: `ทั้งหมด (${licenseCounts.total})`
          }, {
            key: 'pending',
            label: `รอสแกน (${licenseCounts.pending})`
          }, {
            key: 'confirmed',
            label: `ยืนยันแล้ว (${licenseCounts.confirmed})`
          }].map(tab => <button key={tab.key} className={'vr-tab' + (licenseTab === tab.key ? ' vr-tab-active' : '')} onClick={() => setLicenseTab(tab.key)}>
              {tab.label}
            </button>)}
        </div>
      </div>

      <div className="tsf-history-toolbar">
        <div className="wh-history-filters">
          <div className="tsf-history-pagesize">
            <div className="wh-pagesize-select">
              <SelectField value={licensePageSize} onChange={setLicensePageSize} options={[{
                value: 10,
                label: '10'
              }, {
                value: 25,
                label: '25'
              }, {
                value: 50,
                label: '50'
              }, {
                value: 100,
                label: '100'
              }]} />
            </div>
            entries per page
          </div>
          <div className="wh-filter-field">
            <span className="wh-filter-label">แบบ/รุ่น</span>
            <SelectField value={licenseModel} onChange={setLicenseModel} options={[{
              value: 'all',
              label: 'ทั้งหมด'
            }, ...licenseModelOptions.map(m => ({
              value: m,
              label: m
            }))]} />
          </div>
          <div className="wh-filter-field">
            <span className="wh-filter-label">ใบอนุญาตนำเข้า</span>
            <SelectField value={licenseNo} onChange={setLicenseNo} options={[{
              value: 'all',
              label: 'ทั้งหมด'
            }, ...licenseNoOptions.map(n => ({
              value: n,
              label: n
            }))]} />
          </div>
        </div>
      </div>

      <div className="wh-table-card">
        <table className="wh-table">
          <thead>
            <tr>
              <th>ลำดับ</th>
              <th>แบบ/รุ่น</th>
              <th>ใบอนุญาตนำเข้า</th>
              <th>อินวอยซ์</th>
              <th>หมายเลขเครื่อง</th>
              <th>หมายเลขการผลิต</th>
              <th>หมายเหตุ</th>
              <th>ส่งออกไปประเทศ</th>
              <th>สถานะ</th>
              <th>ยืนยันเมื่อ</th>
            </tr>
          </thead>
          <tbody>
            {loading && <tr>
                <td colSpan={10} className="wh-empty-cell">
                  กำลังโหลดข้อมูล...
                </td>
              </tr>}
            {!loading && licensePaged.map((r, idx) => <tr key={r.ID} className={highlightId === r.ID ? 'il-row-hit' : ''}>
                  <td className="wh-cell-head" data-label="ลำดับ">
                    {(licensePage - 1) * licensePageSize + idx + 1}
                  </td>
                  <td data-label="แบบ/รุ่น">{r.Model || '—'}</td>
                  <td data-label="ใบอนุญาตนำเข้า">{r.LicenseNo || '—'}</td>
                  <td data-label="อินวอยซ์">{r.InvoiceNo || '—'}</td>
                  <td className="il-mono" data-label="หมายเลขเครื่อง">
                    <strong>{r.MachineNo}</strong>
                  </td>
                  <td className="il-mono" data-label="หมายเลขการผลิต">
                    {r.ProductionNo || '—'}
                  </td>
                  <td data-label="หมายเหตุ">{r.Remark || '—'}</td>
                  <td data-label="ส่งออกไปประเทศ">{r.ExportCountry || '—'}</td>
                  <td data-label="สถานะ">
                    {r.ConfirmStatus === 'CONFIRMED' ? <span className="il-badge il-badge-ok">
                        <CheckIcon className="inline size-3.5 align-text-bottom" /> ตรงกัน
                      </span> : <span className="il-badge il-badge-pending">
                        <ClockIcon className="inline size-3.5 align-text-bottom" /> รอสแกน
                      </span>}
                  </td>
                  <td data-label="ยืนยันเมื่อ">
                    {r.ConfirmedDatetime ? new Date(r.ConfirmedDatetime).toLocaleString('th-TH') : '—'}
                  </td>
                </tr>)}
            {!loading && licenseRows.length === 0 && <tr>
                <td colSpan={10} className="wh-empty-cell">
                  ไม่มีรายการในมุมมองนี้
                </td>
              </tr>}
          </tbody>
        </table>
      </div>

      {!loading && licenseRows.length > 0 && <div className="tsf-pagination">
          <span className="wh-subtitle" style={{
          fontSize: 13
        }}>
            Showing {(licensePage - 1) * licensePageSize + 1} to{' '}
            {Math.min(licensePage * licensePageSize, licenseRows.length)} of {licenseRows.length}{' '}
            entries
          </span>
          <div className="tsf-pagination-buttons">
            <button className="wh-modal-cancel" onClick={() => goToLicensePage(1)} disabled={licensePage === 1}>
              <ChevronDoubleLeftIcon className="size-4" />
            </button>
            <button className="wh-modal-cancel" onClick={() => goToLicensePage(licensePage - 1)} disabled={licensePage === 1}>
              <ChevronLeftIcon className="size-4" />
            </button>
            <span className="tsf-pagination-current">
              {licensePage} / {licenseTotalPages}
            </span>
            <button className="wh-modal-cancel" onClick={() => goToLicensePage(licensePage + 1)} disabled={licensePage === licenseTotalPages}>
              <ChevronRightIcon className="size-4" />
            </button>
            <button className="wh-modal-cancel" onClick={() => goToLicensePage(licenseTotalPages)} disabled={licensePage === licenseTotalPages}>
              <ChevronDoubleRightIcon className="size-4" />
            </button>
          </div>
        </div>}
        </>}
    </AppShell>;
}
