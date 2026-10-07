import { useEffect, useRef, useState } from 'react';
import AppShell from '../components/AppShell.jsx';
import SelectField from '../components/Selectfield.jsx';
import useFileDrop from '../lib/useFileDrop.js';
import { getUploadData, uploadDataFile, deleteUploadDataRow, updateUploadDataRow, clearUploadData, previewUploadData } from '../api/uploadData.js';
import { PreviewResult, ChangePreview } from '../components/FormatTools.jsx';
import { EditableCell, EditHint, LockBadge, useLiveRefresh } from '../components/InlineEdit.jsx';
import { ADMIN_NAV_ITEMS } from './AdminDashboardpage.jsx';
import { confirmUploadDeletes, syncResultParts } from '../lib/uploadSync.js';
import { confirmDelete, toastError, toastSuccess } from '../lib/toast.js';
import { buildStyledXlsxBlob, downloadBlob } from '../lib/xlsx.js';
import { CloudArrowUpIcon } from '../components/icons.jsx';
import { ArrowDownTrayIcon, RectangleStackIcon } from '../components/icons.jsx';
import '../UploadData.css';
// ชุดข้อมูลที่อัปโหลดได้แล้ว — ตาม flow ใหม่ (ss.xlsx Sheet1)
//   Planning WH  : แผนจ่าย Engine / IT# รายเครื่อง (MC#) — ชีต Engine_IT allocation
//   Planning MFG : ผูก MC# เข้ากับ Product Spec — ชีต Spec sheet_pcp
//   Master Data  : P/N ของ CW / CV / SM / MP / PH / Engine ตาม Product Spec — ชีต CW_CV_ITS
// ของเดิม (ALL PART, IT Controller, Swing Motor, Pump Assy HYD, Motor Propel,
// Control Valve, Planning, Daily Plan, WH1, WH2, Engine) ไม่เปิดให้อัปโหลดแล้ว
const DATASET_TYPES = [{
  value: 'engine_it_allocation',
  label: 'Planning WH'
}, {
  value: 'spec_sheet',
  label: 'Planning MFG'
}, {
  value: 'cw_cv_its',
  label: 'Master Data'
}];
const DEFAULT_DATASET = DATASET_TYPES[0].value;
const UPLOAD_TYPE_OPTIONS = DATASET_TYPES;
const TYPE_OPTIONS = DATASET_TYPES;
const ALL_TYPE_LABELS = Object.fromEntries(DATASET_TYPES.map(t => [t.value, t.label]));
function typeLabel(value) {
  return ALL_TYPE_LABELS[value] || value;
}
const uploadNavItems = [{
  to: '/master-data',
  label: 'อัพโหลดข้อมูล',
  icon: <RectangleStackIcon className="size-4" />
}];
export default function MasterDataPage() {
  const role = (localStorage.getItem('iconfirm_role') || '').toUpperCase();
  // แก้ไขในตาราง (ปุ่มดินสอ) เฉพาะ ADMIN — UPLOAD แก้ข้อมูลใน Excel แล้วอัปโหลดใหม่
  const canEdit = role === 'ADMIN';
  const navItems = canEdit ? ADMIN_NAV_ITEMS : uploadNavItems;
  const shellRoleLabel = canEdit ? 'Admin' : 'Upload';
  const [uploadType, setUploadType] = useState(DEFAULT_DATASET);
  const [viewType, setViewType] = useState(DEFAULT_DATASET);
  const [pendingFile, setPendingFile] = useState(null);
  const [uploading, setUploading] = useState(false);
  const [uploadMsg, setUploadMsg] = useState(null);
  const [previewData, setPreviewData] = useState(null);
  const [previewing, setPreviewing] = useState(false);
  const fileInputRef = useRef(null);
  const canPreview = true;
  async function handlePreview() {
    if (!pendingFile) {
      setUploadMsg({
        error: 'กรุณาเลือกไฟล์ก่อนตรวจสอบ'
      });
      return;
    }
    setPreviewing(true);
    setPreviewData(null);
    try {
      const data = await previewUploadData(uploadType, pendingFile);
      const useChangeView = !!data?.summary;
      setPreviewData({
        ...data,
        _mode: useChangeView ? 'change' : 'map'
      });
    } catch (err) {
      setUploadMsg({
        error: err.message || 'ตรวจสอบไฟล์ไม่สำเร็จ'
      });
    } finally {
      setPreviewing(false);
    }
  }
  const [reloadKey, setReloadKey] = useState(0);
  function acceptFile(file) {
    setPendingFile(file || null);
    setUploadMsg(null);
    setPreviewData(null);
  }
  function handleFileChange(e) {
    acceptFile(e.target.files?.[0] || null);
  }
  const {
    dragging: fileDragging,
    stateClass: fileDropState,
    dropProps: fileDropProps
  } = useFileDrop({
    accept: '.xlsx,.xls,.csv',
    disabled: uploading,
    onFile: acceptFile,
    onReject: (file, hint) => setUploadMsg({
      error: `ไฟล์ "${file.name}" ไม่รองรับ — ต้องเป็น ${hint}`
    })
  });
  async function handleUpload() {
    if (!pendingFile) {
      setUploadMsg({
        error: 'กรุณาเลือกไฟล์ Excel ก่อน'
      });
      return;
    }
    setUploading(true);
    setUploadMsg(null);
    try {
      let summary = previewData?.summary;
      if (!summary) {
        const pv = await previewUploadData(uploadType, pendingFile);
        summary = pv?.summary;
      }
      if (!(await confirmUploadDeletes(summary, pendingFile.name))) {
        return;
      }
      const result = await uploadDataFile(uploadType, pendingFile);
      const parts = [`เพิ่มใหม่ ${result.imported ?? 0}`, `อัปเดต ${result.updated ?? 0}`];
      if (result.duplicate) parts.push(`เหมือนเดิม ${result.duplicate}`);
      if (result.locked) parts.push(`สแกนแล้ว ไม่อัปเดต ${result.locked}`);
      parts.push(...syncResultParts(result));
      if (result.skipped) parts.push(`ข้าม ${result.skipped}`);
      setUploadMsg({
        success: parts.join(' · '),
        problems: result.problems || []
      });
      setPendingFile(null);
      setPreviewData(null);
      if (fileInputRef.current) fileInputRef.current.value = '';
      setViewType(uploadType);
      setReloadKey(n => n + 1);
    } catch (err) {
      setUploadMsg({
        error: err.message || 'อัปโหลดไม่สำเร็จ'
      });
    } finally {
      setUploading(false);
    }
  }
  return <AppShell navItems={navItems} roleLabel={shellRoleLabel}>
      <div className="wh-heading-row">
        <div>
          <h2 className="wh-title">อัพโหลดข้อมูล</h2>
        </div>
      </div>

      <div className="upload-panel upload-panel-wide">
        <label className={['upload-dropzone', 'upload-panel-dropzone', pendingFile ? 'upload-dropzone-filled' : '', fileDropState].filter(Boolean).join(' ')} htmlFor="md-file" {...fileDropProps}>
          <input id="md-file" ref={fileInputRef} type="file" accept=".xlsx,.xls,.csv" onChange={handleFileChange} className="upload-card-input-hidden" />
          <span className="upload-dropzone-icon">
            <CloudArrowUpIcon className="size-[26px]" />
          </span>
          <span className="upload-dropzone-text">
            {fileDragging ? <span className="dz-drop-text">
                <span className="dz-arrow">↓</span> ปล่อยไฟล์ได้เลย
              </span> : pendingFile ? pendingFile.name : typeLabel(uploadType)}
          </span>
          <span className="upload-dropzone-hint">.xlsx, .xls, .csv</span>
        </label>

        <div className="upload-panel-side">
          <div className="upload-panel-field">
            <label className="upload-panel-label" htmlFor="md-upload-type">
              ประเภทที่อัปโหลด
            </label>
            <SelectField value={uploadType} onChange={v => {
            setUploadType(v);
            setPendingFile(null);
            setUploadMsg(null);
            setPreviewData(null);
            if (fileInputRef.current) fileInputRef.current.value = '';
          }} options={UPLOAD_TYPE_OPTIONS.map(t => ({
            value: t.value,
            label: t.label
          }))} />
          </div>

          <div style={{
          display: 'flex',
          flexWrap: 'wrap',
          gap: 8
        }}>
            {canPreview && <button className="qa-fail-btn upload-panel-btn" style={{
            background: '#eef2ff',
            color: '#4338ca',
            borderColor: '#c7d2fe'
          }} disabled={previewing || uploading} onClick={handlePreview}>
                {previewing ? 'กำลังตรวจสอบ...' : 'ตรวจสอบก่อนอัปโหลด'}
              </button>}
            <button className="wh-issue-btn upload-panel-btn" disabled={uploading} onClick={handleUpload}>
              {uploading ? 'กำลังอัปโหลด...' : `อัปโหลด ${typeLabel(uploadType)}`}
            </button>
          </div>
        </div>

        {previewData && (previewData._mode === 'change' ? <ChangePreview result={previewData} typeLabel={typeLabel} /> : <PreviewResult result={previewData} />)}

        {uploadMsg?.success && <p className="upload-card-msg upload-card-msg-ok">{uploadMsg.success}</p>}
        {uploadMsg?.error && <p className="upload-card-msg upload-card-msg-err">{uploadMsg.error}</p>}

        {uploadMsg?.problems?.length > 0 && <ul className="upload-card-msg upload-card-msg-err" style={{
        textAlign: 'left',
        margin: '8px 0 0'
      }}>
            {uploadMsg.problems.map((problem, i) => <li key={i}>{problem}</li>)}
          </ul>}
      </div>

      <div className="wh-heading-row" style={{
      marginTop: 28
    }}>
        <div>
          <h2 className="wh-title" style={{
          fontSize: 19
        }}>
            รายการ — {typeLabel(viewType)}
          </h2>
        </div>
        <div className="md-type-field">
          <SelectField value={viewType} onChange={setViewType} options={TYPE_OPTIONS.map(t => ({
          value: t.value,
          label: t.label
        }))} />
        </div>
      </div>

      <DatasetView key={`${viewType}-${reloadKey}`} dataset={viewType} canEdit={canEdit} />
    </AppShell>;
}
const UD_PAGE_SIZE = 100;
function DatasetView({
  dataset,
  canEdit = false
}) {
  const label = typeLabel(dataset);
  const [columns, setColumns] = useState([]);
  const [rows, setRows] = useState([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [keyword, setKeyword] = useState('');
  const [exporting, setExporting] = useState(false);
  const [localReload, setLocalReload] = useState(0);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [totalPages, setTotalPages] = useState(1);
  const loadSeq = useRef(0);
  function runSearch() {
    setPage(1);
    setLocalReload(n => n + 1);
  }
  const firstKeywordRun = useRef(true);
  useEffect(() => {
    if (firstKeywordRun.current) {
      firstKeywordRun.current = false;
      return;
    }
    const t = setTimeout(runSearch, 350);
    return () => clearTimeout(t);
  }, [keyword]);
  async function load(silent = false) {
    const seq = ++loadSeq.current;
    if (!silent) {
      setLoading(true);
      setLoadError('');
    }
    try {
      const data = await getUploadData(dataset, keyword || undefined, page, UD_PAGE_SIZE);
      if (seq !== loadSeq.current) return;
      setColumns(data?.columns || []);
      setRows(data?.rows || []);
      setTotal(data?.total ?? 0);
      setTotalPages(data?.totalPages || 1);
    } catch (err) {
      if (!silent && seq === loadSeq.current) setLoadError(err.message || 'โหลดรายการไม่สำเร็จ');
    } finally {
      if (!silent && seq === loadSeq.current) setLoading(false);
    }
  }
  useEffect(() => {
    load(false);
  }, [dataset, localReload, page]);
  useLiveRefresh(() => load(true));
  async function saveCell(row, col, value) {
    try {
      const res = await updateUploadDataRow(row.ID, {
        [col]: value
      });
      if (res?.row) {
        setRows(list => list.map(r => r.ID === row.ID ? res.row : r));
      }
    } catch (err) {
      if (err?.status === 409) load(true);
      throw err;
    }
  }
  async function handleDelete(row) {
    const ok = await confirmDelete({
      text: 'ลบแถวนี้? กู้คืนไม่ได้'
    });
    if (!ok) return;
    try {
      await deleteUploadDataRow(row.ID);
      setLocalReload(n => n + 1);
      toastSuccess('ลบแถวแล้ว');
    } catch (err) {
      toastError(err.message || 'ลบไม่สำเร็จ');
      if (err?.status === 409) load(true);
    }
  }
  async function handleClear() {
    const ok = await confirmDelete({
      text: `ล้างข้อมูล ${label} ทั้งหมด? กู้คืนไม่ได้`
    });
    if (!ok) return;
    try {
      const res = await clearUploadData(dataset);
      setPage(1);
      setLocalReload(n => n + 1);
      toastSuccess(`ล้างแล้ว ${res.deleted ?? 0} แถว`);
    } catch (err) {
      toastError(err.message || 'ล้างไม่สำเร็จ');
    }
  }
  async function handleExport() {
    setExporting(true);
    setLoadError('');
    try {
      const PAGE = 500;
      let all = [];
      let cols = [];
      let p = 1;
      for (let guard = 0; guard < 2000; guard++) {
        const data = await getUploadData(dataset, undefined, p, PAGE);
        if (p === 1) cols = data?.columns || [];
        const batch = data?.rows || [];
        all = all.concat(batch);
        const totalPages = data?.totalPages || 1;
        if (p >= totalPages || batch.length === 0) break;
        p += 1;
      }
      if (all.length === 0) {
        setLoadError('ยังไม่มีข้อมูลให้ Export');
        return;
      }
      const parsed = all.map(row => {
        try {
          return JSON.parse(row.DataJSON || '{}');
        } catch {
          return {};
        }
      });
      const isPlanningExport = dataset === 'planning';
      const exportCols = isPlanningExport ? cols.filter(label => label !== 'Line') : cols;
      const numericByCol = exportCols.map(label => {
        // LOT NO. (เช่น 10.040) ต้องคงเป็นข้อความ ไม่งั้นเลข 0 ท้ายหาย
        if (label === 'LOT NO.') return false;
        let sawValue = false;
        for (const obj of parsed) {
          const raw = obj[label];
          if (raw == null || String(raw).trim() === '') continue;
          sawValue = true;
          const s = String(raw).trim().replace(/,/g, '');
          if (!/^-?\d+(\.\d+)?$/.test(s)) return false;
          if (s.length > 11 || /^0\d/.test(s)) return false;
        }
        return sawValue;
      });
      let noNumeric = false;
      if (isPlanningExport) {
        let sawValue = false;
        noNumeric = true;
        for (const obj of parsed) {
          const raw = obj['Line'];
          if (raw == null || String(raw).trim() === '') continue;
          sawValue = true;
          const s = String(raw).trim().replace(/,/g, '');
          if (!/^-?\d+(\.\d+)?$/.test(s)) {
            noNumeric = false;
            break;
          }
        }
        noNumeric = sawValue && noNumeric;
      }
      const columns = [{
        key: '_no',
        header: '#',
        type: isPlanningExport ? noNumeric ? 'number' : 'text' : 'number',
        width: 6
      }, ...exportCols.map((label, i) => ({
        key: `c${i}`,
        header: label,
        type: numericByCol[i] ? 'number' : 'text'
      }))];
      const rows = parsed.map((obj, idx) => {
        const fallbackNo = idx + 1;
        let noVal = fallbackNo;
        if (isPlanningExport) {
          const raw = obj['Line'];
          const s = raw == null ? '' : String(raw).trim();
          if (s !== '') {
            noVal = noNumeric ? Number(s.replace(/,/g, '')) : s;
          }
        }
        const out = {
          _no: noVal
        };
        exportCols.forEach((label, i) => {
          const raw = obj[label];
          if (numericByCol[i]) {
            const s = raw == null ? '' : String(raw).trim().replace(/,/g, '');
            out[`c${i}`] = s === '' ? null : Number(s);
          } else {
            out[`c${i}`] = raw == null || String(raw).trim() === '' ? '' : String(raw);
          }
        });
        return out;
      });
      const sheetName = (label || 'Data').slice(0, 31);
      const blob = buildStyledXlsxBlob({
        sheetName,
        columns,
        rows
      });
      const stamp = new Date().toISOString().slice(0, 19).replace(/[:T]/g, '-');
      downloadBlob(blob, `${dataset}-export-${stamp}.xlsx`);
    } catch (err) {
      setLoadError(err.message || 'Export ไม่สำเร็จ');
    } finally {
      setExporting(false);
    }
  }
  function cellValue(row, colName) {
    try {
      const data = JSON.parse(row.DataJSON || '{}');
      return data[colName] ?? '';
    } catch {
      return '';
    }
  }
  const lockedCount = rows.filter(r => r.Locked).length;
  const isPlanning = dataset === 'planning';
  const displayColumns = isPlanning ? columns.filter(c => c !== 'Line') : columns;
  return <>
      {loadError && <p className="form-error" role="alert">
          {loadError}
        </p>}

      <div className="wh-heading-row">
        <div>
          <h2 className="wh-title" style={{
          fontSize: 17
        }}>
            {total.toLocaleString()} รายการ
          </h2>
          <EditHint lockedCount={lockedCount} canEdit={canEdit} />
        </div>
        <div className="uv-list-tools" style={{
        display: 'flex',
        gap: 10,
        flexWrap: 'wrap'
      }}>
          <input className="wh-search" placeholder="ค้นหา (เลขเครื่อง / LOT / Order / Parts)" value={keyword} onChange={e => setKeyword(e.target.value)} style={{
          minWidth: 240
        }} />
          <button className="wh-issue-btn" onClick={handleExport} disabled={exporting}>
            {exporting ? 'กำลัง Export...' : <>
                <ArrowDownTrayIcon className="size-4" /> Export Excel
              </>}
          </button>
          <button className="qa-fail-btn" onClick={handleClear} disabled={total === 0}>
            ล้างทั้งหมด
          </button>
        </div>
      </div>

      <div className="wh-table-card ud-table-scroll">
        <table className="wh-table ud-table">
          <thead>
            <tr>
              <th className="ud-th-sticky">#</th>
              {displayColumns.map(c => <th key={c}>{c}</th>)}
              <th>สถานะการสแกน</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {loading && <tr>
                <td colSpan={displayColumns.length + 3} className="wh-empty-cell">
                  กำลังโหลดข้อมูล...
                </td>
              </tr>}
            {!loading && rows.map((row, i) => <tr key={row.ID} className={row.Locked ? 'ie-row-locked' : ''}>
                  <td className="ud-td-sticky">{isPlanning ? cellValue(row, 'Line') || (page - 1) * UD_PAGE_SIZE + i + 1 : (page - 1) * UD_PAGE_SIZE + i + 1}</td>
                  {displayColumns.map(c => <td key={c} data-label={c}>
                      <EditableCell locked={!!row.Locked} lockReason={row.LockReason} readOnly={!canEdit} label={c} value={cellValue(row, c)} onSave={v => saveCell(row, c, v)} />
                    </td>)}
                  <td data-label="สถานะการสแกน">
                    {row.Locked ? <LockBadge locked reason={row.LockReason} /> : <span className="ie-empty">ยังไม่ได้สแกน</span>}
                  </td>
                  <td className="wh-cell-action">
                    <div style={{
                display: 'flex',
                gap: 6,
                justifyContent: 'flex-end'
              }}>
                      <button className="qa-fail-btn" disabled={row.Locked} title={row.Locked ? `ลบไม่ได้ — ${row.LockReason || 'สแกนผ่านแล้ว'}` : ''} onClick={() => handleDelete(row)}>
                        ลบ
                      </button>
                    </div>
                  </td>
                </tr>)}
            {!loading && rows.length === 0 && <tr>
                <td colSpan={displayColumns.length + 3} className="wh-empty-cell">
                  ยังไม่มีรายการที่อัปโหลด
                </td>
              </tr>}
          </tbody>
        </table>
      </div>

      {totalPages > 1 && <div className="ud-pager" style={{
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'flex-end',
      gap: 12,
      marginTop: 12
    }}>
          <button className="wh-issue-btn" onClick={() => setPage(p => Math.max(1, p - 1))} disabled={page <= 1 || loading}>
            ก่อนหน้า
          </button>
          <span style={{
        fontSize: 14
      }}>
            หน้า {page.toLocaleString()} / {totalPages.toLocaleString()}
          </span>
          <button className="wh-issue-btn" onClick={() => setPage(p => Math.min(totalPages, p + 1))} disabled={page >= totalPages || loading}>
            ถัดไป
          </button>
        </div>}

    </>;
}
