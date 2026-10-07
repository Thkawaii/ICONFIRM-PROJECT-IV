import { useEffect, useMemo, useRef, useState } from 'react';
import { getWarehouseStock, issueWarehouseStock, uploadWarehouseStock } from '../api/warehouse.js';
import AppShell from '../components/AppShell.jsx';
import FileDropZone from '../components/Filedropzone.jsx';
const navItems = [{
  to: '/warehouse',
  label: 'จ่ายของ (FIFO & S/O)',
  icon: '📦'
}, {
  to: '/warehouse/confirm',
  label: 'Part Confirmation',
  icon: '✅'
}];
const STOCK_TABS = [{
  key: 'all',
  label: 'ทั้งหมด'
}, {
  key: 'not_issued',
  label: 'ยังไม่เคยจ่าย'
}, {
  key: 'issued',
  label: 'จ่ายแล้ว'
}];
export default function WarehousePage() {
  const [stock, setStock] = useState([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [search, setSearch] = useState('');
  const [selectedSO, setSelectedSO] = useState('');
  const [soDropdownOpen, setSoDropdownOpen] = useState(false);
  const soDropdownRef = useRef(null);
  const [stockTab, setStockTab] = useState('all');
  const [issuingRow, setIssuingRow] = useState(null);
  const [issueQty, setIssueQty] = useState('');
  const [issueRemark, setIssueRemark] = useState('');
  const [issuing, setIssuing] = useState(false);
  const [issueError, setIssueError] = useState('');
  const [toast, setToast] = useState('');
  const [soFile, setSoFile] = useState(null);
  const [soUploading, setSoUploading] = useState(false);
  const [soMsg, setSoMsg] = useState(null);
  async function loadAll() {
    setLoading(true);
    setLoadError('');
    try {
      const stockData = await getWarehouseStock();
      setStock(stockData || []);
    } catch (err) {
      setLoadError(err.message || 'โหลดข้อมูลคลังไม่สำเร็จ');
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => {
    loadAll();
  }, []);
  useEffect(() => {
    if (!soDropdownOpen) return;
    function onOutside(e) {
      if (soDropdownRef.current && !soDropdownRef.current.contains(e.target)) {
        setSoDropdownOpen(false);
      }
    }
    document.addEventListener('mousedown', onOutside);
    document.addEventListener('touchstart', onOutside);
    return () => {
      document.removeEventListener('mousedown', onOutside);
      document.removeEventListener('touchstart', onOutside);
    };
  }, [soDropdownOpen]);
  function handleSoFile(file) {
    setSoFile(file);
    setSoMsg(null);
  }
  async function handleSoUpload() {
    if (!soFile) {
      setSoMsg({
        error: 'กรุณาเลือกไฟล์ Excel ก่อน'
      });
      return;
    }
    setSoUploading(true);
    setSoMsg(null);
    try {
      const result = await uploadWarehouseStock(soFile);
      setSoMsg({
        success: `นำเข้าสำเร็จ ${result.imported} รายการ`
      });
      setSoFile(null);
      await loadAll();
    } catch (err) {
      setSoMsg({
        error: err.message || 'อัปโหลดไม่สำเร็จ'
      });
    } finally {
      setSoUploading(false);
    }
  }
  const filtered = useMemo(() => {
    const term = search.trim().toLowerCase();
    let rows = term ? stock.filter(r => (r.OrderNo || '').toLowerCase().includes(term) || (r.PartNo || '').toLowerCase().includes(term) || (r.PartName || '').toLowerCase().includes(term)) : stock;
    if (selectedSO) rows = rows.filter(r => r.OrderNo === selectedSO);
    if (stockTab === 'issued') rows = rows.filter(r => Boolean(r.StockOutNo));
    if (stockTab === 'not_issued') rows = rows.filter(r => !r.StockOutNo);
    return [...rows].sort((a, b) => new Date(a.UploadDate) - new Date(b.UploadDate));
  }, [stock, search, selectedSO, stockTab]);
  const salesOrders = useMemo(() => {
    const map = new Map();
    for (const r of stock) {
      if (!r.OrderNo) continue;
      if (!map.has(r.OrderNo)) map.set(r.OrderNo, {
        total: 0,
        issued: 0
      });
      const entry = map.get(r.OrderNo);
      entry.total += 1;
      if (r.StockOutNo) entry.issued += 1;
    }
    return Array.from(map.entries()).map(([orderNo, v]) => ({
      orderNo,
      ...v
    }));
  }, [stock]);
  const stockCounts = useMemo(() => ({
    all: stock.length,
    issued: stock.filter(r => Boolean(r.StockOutNo)).length,
    not_issued: stock.filter(r => !r.StockOutNo).length
  }), [stock]);
  const currentSO = salesOrders.find(s => s.orderNo === selectedSO);
  function openIssueModal(row) {
    if (row.RemainQty <= 0) return;
    setIssuingRow(row);
    setIssueQty('1');
    setIssueRemark('');
    setIssueError('');
  }
  function closeIssueModal() {
    setIssuingRow(null);
    setIssueQty('');
    setIssueRemark('');
    setIssueError('');
  }
  async function confirmIssue() {
    const qty = Number(issueQty);
    if (!qty || qty < 1 || qty > issuingRow.RemainQty) {
      setIssueError('จำนวนที่จ่ายไม่ถูกต้อง');
      return;
    }
    setIssuing(true);
    setIssueError('');
    try {
      const result = await issueWarehouseStock(issuingRow.ID, {
        qty,
        scannedPartNo: issuingRow.PartNo,
        scannedSerial: `WH-${issuingRow.ID}-${Date.now()}`,
        remark: issueRemark
      });
      setToast(`จ่ายชิ้นส่วน ${issuingRow.PartName} (${issuingRow.PartNo}) จำนวน ${qty} ชิ้น อ้างอิง SO ${issuingRow.OrderNo} — เลขที่จ่ายของ ${result.stock_out_no} — ส่งให้ TSF แล้ว รอรับของ`);
      setTimeout(() => setToast(''), 6000);
      closeIssueModal();
      await loadAll();
    } catch (err) {
      setIssueError(err.message || 'จ่ายของไม่สำเร็จ');
    } finally {
      setIssuing(false);
    }
  }
  return <>
      <AppShell navItems={navItems} roleLabel="Warehouse">
        <div className="wh-heading-row">
          <div>
            <h2 className="wh-title">Warehouse(Issue Parts)</h2>
          </div>
        </div>

        {toast && <div className="wh-toast">{toast}</div>}
        {loadError && <p className="form-error" role="alert">
            {loadError}
          </p>}

        <div className="dash-stats-row wh-stats-row">
          <div className="dash-stat-card">
            <div className="dash-stat-label">
              <span>รายการทั้งหมด</span>
              <span className="dash-stat-icon dash-icon-blue">▦</span>
            </div>
            <div className="dash-stat-value">{stockCounts.all}</div>
          </div>
          <div className="dash-stat-card">
            <div className="dash-stat-label">
              <span>ยังไม่เคยจ่าย</span>
              <span className="dash-stat-icon dash-icon-yellow">⏳</span>
            </div>
            <div className="dash-stat-value">{stockCounts.not_issued}</div>
          </div>
          <div className="dash-stat-card">
            <div className="dash-stat-label">
              <span>จ่ายแล้ว</span>
              <span className="dash-stat-icon dash-icon-green">✓</span>
            </div>
            <div className="dash-stat-value">{stockCounts.issued}</div>
          </div>
          <div className="dash-stat-card">
            <div className="dash-stat-label">
              <span>Sales Order</span>
              <span className="dash-stat-icon dash-icon-red">🧾</span>
            </div>
            <div className="dash-stat-value">{salesOrders.length}</div>
          </div>
        </div>

        <div className="wh-upload-card">
          <div className="fdz-row">
            <FileDropZone file={soFile} onSelect={handleSoFile} accept=".xlsx,.xls" label="อัปโหลด SO / สต็อกเข้าคลัง" disabled={soUploading} />
            <button className="wh-issue-btn" onClick={handleSoUpload} disabled={soUploading || !soFile}>
              {soUploading ? 'กำลังอัปโหลด...' : 'อัปโหลด'}
            </button>
          </div>
          {soMsg?.success && <p className="upload-card-msg upload-card-msg-ok wh-upload-msg">{soMsg.success}</p>}
          {soMsg?.error && <p className="upload-card-msg upload-card-msg-err wh-upload-msg">{soMsg.error}</p>}
        </div>

        {!selectedSO ? <div className="wh-so-picker-card">
            <div className="wh-so-picker-icon">📋</div>
            <h3 className="tsf-form-title" style={{
          marginBottom: 4
        }}>
            </h3>
            <div className="wh-so-select-wrap" ref={soDropdownRef}>
              <button type="button" className="wh-modal-input wh-so-select wh-so-select-trigger" onClick={() => setSoDropdownOpen(v => !v)}>
                <span className="wh-so-select-value">
                  {selectedSO ? (() => {
                const so = salesOrders.find(s => s.orderNo === selectedSO);
                return so ? `${so.orderNo} (${so.issued}/${so.total} จ่ายแล้ว)` : selectedSO;
              })() : '-- เลือก Sales Order --'}
                </span>
                <span className={'wh-so-select-caret' + (soDropdownOpen ? ' wh-so-select-caret-open' : '')}>▾</span>
              </button>
              {soDropdownOpen && <ul className="wh-so-select-menu" role="listbox">
                  <li className={'wh-so-select-option' + (selectedSO === '' ? ' wh-so-select-option-active' : '')} onClick={() => {
              setSelectedSO('');
              setSoDropdownOpen(false);
            }}>
                    -- เลือก Sales Order --
                  </li>
                  {salesOrders.map(so => <li key={so.orderNo} className={'wh-so-select-option' + (selectedSO === so.orderNo ? ' wh-so-select-option-active' : '')} onClick={() => {
              setSelectedSO(so.orderNo);
              setSoDropdownOpen(false);
            }}>
                      {so.orderNo} ({so.issued}/{so.total} จ่ายแล้ว)
                    </li>)}
                </ul>}
            </div>
            {salesOrders.length === 0 && <p className="wh-subtitle" style={{
          marginTop: 10
        }}>
                ยังไม่มี Sales Order ในระบบ — อัปโหลดไฟล์ Excel ด้านบนก่อน
              </p>}
          </div> : <>
            <div className="wh-so-active-bar">
              <div>
                <span className="wh-so-active-label">Sales Order ที่กำลังจ่าย</span>
                <h3 className="wh-so-active-name">{selectedSO}</h3>
              </div>
              <div className="wh-so-active-progress">
                <div className="wh-so-progress-track">
                  <div className="wh-so-progress-fill" style={{
                width: `${currentSO && currentSO.total ? currentSO.issued / currentSO.total * 100 : 0}%`
              }} />
                </div>
                <span className="wh-so-progress-label">
                  {currentSO?.issued || 0}/{currentSO?.total || 0} รายการจ่ายแล้ว
                </span>
              </div>
              <button className="wh-modal-cancel" onClick={() => setSelectedSO('')}>
                เปลี่ยน S/O
              </button>
            </div>

            <div className="wh-toolbar-row">
              <div className="vr-tabs">
                {STOCK_TABS.map(tab => <button key={tab.key} className={'vr-tab' + (stockTab === tab.key ? ' vr-tab-active' : '')} onClick={() => setStockTab(tab.key)}>
                    {tab.label} ({stockCounts[tab.key]})
                  </button>)}
              </div>
              <input className="wh-search" type="text" placeholder="ค้นหาด้วย Part No / Part Name" value={search} onChange={e => setSearch(e.target.value)} />
            </div>

            <div className="wh-table-card">
              <table className="wh-table">
                <thead>
                  <tr>
                    <th>FIFO #</th>
                    <th>Part No.</th>
                    <th>Part Name</th>
                    <th>Assembly For</th>
                    <th>Shelf</th>
                    <th>Remain Qty</th>
                    <th>Upload Date</th>
                    <th>Stock Out No.</th>
                    <th></th>
                  </tr>
                </thead>
                <tbody>
                  {loading && <tr>
                      <td colSpan={9} className="wh-empty-cell">
                        กำลังโหลดข้อมูล...
                      </td>
                    </tr>}
                  {!loading && filtered.map((row, idx) => <tr key={row.ID} className={row.RemainQty <= 0 ? 'wh-row-empty' : ''}>
                        <td className="wh-cell-head" data-label="FIFO">#{idx + 1}</td>
                        <td className="il-mono" data-label="Part No.">{row.PartNo}</td>
                        <td data-label="Part Name">{row.PartName}</td>
                        <td data-label="Assembly For">{row.AssemblyPartName}</td>
                        <td data-label="Shelf">
                          {row.Shelf1}-{row.Shelf2}
                        </td>
                        <td data-label="Remain Qty">
                          <span className={row.RemainQty <= 0 ? 'wh-qty-badge wh-qty-zero' : 'wh-qty-badge'}>
                            {row.RemainQty}
                          </span>
                        </td>
                        <td data-label="Upload Date">{row.UploadDate ? new Date(row.UploadDate).toLocaleDateString('th-TH') : '—'}</td>
                        <td data-label="Stock Out No.">{row.StockOutNo || '—'}</td>
                        <td className="wh-cell-action">
                          <button className="wh-issue-btn" disabled={row.RemainQty <= 0} onClick={() => openIssueModal(row)}>
                            จ่ายของ
                          </button>
                        </td>
                      </tr>)}
                  {!loading && filtered.length === 0 && <tr>
                      <td colSpan={9} className="wh-empty-cell">
                        ไม่พบข้อมูล
                      </td>
                    </tr>}
                </tbody>
              </table>
            </div>
          </>}
      </AppShell>

      {issuingRow && <div className="wh-modal-overlay" onClick={closeIssueModal}>
          <div className="wh-modal" onClick={e => e.stopPropagation()}>
            <h3 className="wh-modal-title">ยืนยันการจ่ายชิ้นส่วน</h3>

            <div className="wh-modal-summary">
              <p className="wh-modal-line">
                <strong>{issuingRow.PartName}</strong>
              </p>
              <p className="wh-modal-line">P/N: {issuingRow.PartNo}</p>
              <p className="wh-modal-line">Sales Order: {issuingRow.OrderNo}</p>
              <p className="wh-modal-line">คงเหลือ: {issuingRow.RemainQty} ชิ้น</p>
            </div>

            <label className="wh-modal-label" htmlFor="issueQty">
              จำนวนที่จะจ่าย
            </label>
            <input id="issueQty" type="number" min="1" max={issuingRow.RemainQty} className="wh-modal-input" value={issueQty} onChange={e => setIssueQty(e.target.value)} autoFocus />

            <label className="wh-modal-label" htmlFor="issueRemark">
              หมายเหตุ (ถ้ามี)
            </label>
            <input id="issueRemark" type="text" className="wh-modal-input" value={issueRemark} onChange={e => setIssueRemark(e.target.value)} />

            {issueError && <p className="form-error" role="alert">
                {issueError}
              </p>}

            <div className="wh-modal-actions">
              <button className="wh-modal-cancel" onClick={closeIssueModal} disabled={issuing}>
                ยกเลิก
              </button>
              <button className="wh-modal-confirm" onClick={confirmIssue} disabled={issuing}>
                {issuing ? 'กำลังบันทึก...' : 'ยืนยันจ่ายของ + ส่งให้ TSF'}
              </button>
            </div>
          </div>
        </div>}
    </>;
}
