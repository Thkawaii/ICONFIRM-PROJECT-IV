import { useEffect, useMemo, useState } from 'react';
import { useAppNavigate, useAppParams } from '../../lib/nav.jsx';
import AppShell from '../../components/AppShell.jsx';
import { getMachineProfile } from '../../api/machineProfile.js';
import { ArrowDownTrayIcon, CheckCircleIcon, CheckIcon, ChevronLeftIcon, XMarkIcon } from '../../components/icons.jsx';
import { computeStatus, getApprovals, getRelevantSections, setApproval, STATUS_LABEL } from '../../api/qaMachineStatus.js';
const navItems = [{
  to: '/qa',
  label: 'ตรวจสอบ QA',
  icon: <CheckCircleIcon className="size-4" />
}];
export default function QAMachineDetail() {
  const {
    machineNo
  } = useAppParams();
  const navigate = useAppNavigate();
  const [spec, setSpec] = useState(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [approvals, setApprovals] = useState({});
  useEffect(() => {
    let cancelled = false;
    async function load() {
      setLoading(true);
      setLoadError('');
      try {
        const data = await getMachineProfile(machineNo);
        if (!cancelled) {
          setSpec(data);
          setApprovals(getApprovals(machineNo));
        }
      } catch (err) {
        if (!cancelled) setLoadError(err.message || 'โหลดข้อมูลเครื่องจักรไม่สำเร็จ');
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    load();
    return () => {
      cancelled = true;
    };
  }, [machineNo]);
  const sections = useMemo(() => getRelevantSections(spec), [spec]);
  const status = useMemo(() => computeStatus(spec, approvals), [spec, approvals]);
  function handleDecide(sectionKey, value) {
    const updated = setApproval(machineNo, sectionKey, value);
    setApprovals(updated);
  }
  function handleDownload() {
    window.print();
  }
  return <AppShell navItems={navItems} roleLabel="QA">
      <div className="qa-detail-topbar">
        <button className="qa-back-btn" onClick={() => navigate('/qa')}>
          <ChevronLeftIcon className="size-4" /> กลับ
        </button>
        <button className="qa-download-btn" onClick={handleDownload}>
          <ArrowDownTrayIcon className="size-4" /> โหลดเอกสาร
        </button>
      </div>

      {loadError && <p className="form-error" role="alert">
          {loadError}
        </p>}

      {loading && <p className="wh-subtitle">กำลังโหลดข้อมูล...</p>}

      {!loading && spec && <>
          <div className="qa-detail-header">
            <span className="qa-detail-header-label">MACHINE UNIT</span>
            <h2 className="qa-detail-header-title">No. {spec.MachineNo}</h2>
            <span className={`qa-status-badge qa-status-${status.toLowerCase()} qa-detail-header-status`}>
              {status === 'OK' ? 'OK' : STATUS_LABEL[status]}
            </span>
          </div>

          {sections.map(section => {
        const decision = approvals[section.key];
        return <div className="qa-detail-section" key={section.key}>
                <h3 className="qa-detail-section-title">{section.title}</h3>
                <div className="qa-detail-section-body">
                  {section.fields.map(f => <div className="qa-detail-field-row" key={f.key}>
                      <span className="qa-detail-field-label">{f.label}:</span>
                      <span className="qa-detail-field-value">{spec[f.key]}</span>
                    </div>)}
                </div>
                <div className="qa-detail-section-actions">
                  <button className={'qa-reject-btn' + (decision === 'rejected' ? ' qa-decision-active-reject' : '')} onClick={() => handleDecide(section.key, 'rejected')} aria-label={`ตีกลับ ${section.title}`}>
                    <XMarkIcon className="size-4" />
                  </button>
                  <button className={'qa-approve-btn' + (decision === 'approved' ? ' qa-decision-active-approve' : '')} onClick={() => handleDecide(section.key, 'approved')} aria-label={`ผ่าน ${section.title}`}>
                    <CheckIcon className="size-4" />
                  </button>
                </div>
              </div>;
      })}

          {sections.length === 0 && <div className="dash-summary-card qa-placeholder-card">
              <p className="wh-subtitle">ไม่มีข้อมูลสเปกให้ตรวจสำหรับเครื่องนี้</p>
            </div>}
        </>}
    </AppShell>;
}
