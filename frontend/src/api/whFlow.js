import { apiFetch } from './client.js';

// ชนิดของที่ WH ต้องจ่าย 1 เครื่อง (ลำดับตรงกับ backend flowComponents)
//   ITC / EN          → P/N + S/N รายเครื่อง จากไฟล์ Planning WH
//   CW/CV/SM/MP/PH    → Part# จาก master_data ตาม Product Spec บน Kanban
export const FLOW_COMPONENTS = [{
  code: 'ITC',
  label: 'IT Controller',
  short: 'IT#'
}, {
  code: 'EN',
  label: 'Engine',
  short: 'Engine'
}, {
  code: 'CW',
  label: 'Counter Weight',
  short: 'CW'
}, {
  code: 'CV',
  label: 'Control Valve',
  short: 'CV'
}, {
  code: 'SM',
  label: 'Swing Motor',
  short: 'SM'
}, {
  code: 'MP',
  label: 'Motor Propel',
  short: 'MP'
}, {
  code: 'PH',
  label: 'Pump Assy HYD',
  short: 'PH'
}];
export function flowComponentLabel(code) {
  return FLOW_COMPONENTS.find(c => c.code === code)?.label || code || '—';
}

// รายชื่อ MC# ที่ WH เลือกได้ (มาจากไฟล์ Planning WH — ชีต Engine_IT allocation)
export function getFlowMachines(keyword) {
  const params = new URLSearchParams();
  if (keyword) params.set('keyword', keyword);
  const qs = params.toString();
  return apiFetch(`/wh-flow/machines${qs ? `?${qs}` : ''}`);
}

// ของที่ WH จ่ายไปแล้วทั้งหมด (ใช้เป็นตารางประวัติในหน้า WH)
export function getFlowIssues() {
  return apiFetch('/wh-flow/issues');
}

// แผนของเครื่อง 1 เครื่อง + ของที่จ่ายแล้ว + ที่ MFG ยืนยันแล้ว
export function getFlowMachine(machineNo) {
  return apiFetch(`/wh-flow/machines/${encodeURIComponent(machineNo)}`);
}

// component ที่หา P/N จาก master_data (ชีต CW_CV_ITS) ตาม Product Spec ของเครื่อง
export const MASTER_DATA_COMPONENTS = ['CW', 'CV', 'SM', 'MP', 'PH'];
export function isMasterDataComponent(code) {
  return MASTER_DATA_COMPONENTS.includes(String(code || '').toUpperCase());
}

// WH จ่ายของ: ITC/EN ส่งทั้ง partNo + serialNo
// CW/CV/SM/MP/PH ส่งเฉพาะ partNo (Part#) — S/N# ให้ MFG สแกนตอนประกอบ
export function issueFlowPart({
  machineNo,
  component,
  partNo,
  serialNo = ''
}) {
  return apiFetch('/wh-flow/issue', {
    method: 'POST',
    body: JSON.stringify({
      machineNo,
      component,
      partNo,
      serialNo
    })
  });
}

export function cancelFlowIssue(id) {
  return apiFetch(`/wh-flow/issue/${id}`, {
    method: 'DELETE'
  });
}

// MFG สแกน QR บน Kanban → ระบบเทียบ MC# + Product Spec + P/N กับ master_data
// แล้วคืน MC#, Product Spec, ผลการเทียบ (specCheck / partCheck) และของที่ WH จ่ายมา
export function scanFlowKanban({
  qrCode = '',
  machineNo = ''
}) {
  return apiFetch('/mfg-flow/kanban', {
    method: 'POST',
    body: JSON.stringify({
      qrCode,
      machineNo
    })
  });
}

// MFG ยืนยันว่าของตรงกับที่ WH จ่ายมา
// CW/CV/SM/MP/PH ต้องส่ง qrCode ของ Kanban (ใช้หา Product Spec) และ serialNo ที่สแกนมาด้วย
export function confirmFlowAssembly({
  machineNo,
  component,
  qrCode = '',
  serialNo = '',
  confirmed = true
}) {
  return apiFetch('/mfg-flow/confirm', {
    method: 'POST',
    body: JSON.stringify({
      machineNo,
      component,
      qrCode,
      serialNo,
      confirmed
    })
  });
}
