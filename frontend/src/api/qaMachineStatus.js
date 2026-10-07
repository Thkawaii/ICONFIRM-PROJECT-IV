const STORAGE_KEY = 'iconfirm_qa_machine_approvals';
export const SECTION_DEFS = [{
  key: 'machine',
  title: 'Machine',
  fields: [{
    key: 'MachineNo',
    label: 'No'
  }]
}, {
  key: 'product_spec',
  title: 'Product Spec',
  fields: [{
    key: 'Spec1',
    label: 'Spec(1)'
  }, {
    key: 'Spec2',
    label: 'Spec(2)'
  }]
}, {
  key: 'kcm_order',
  title: 'KCM Order',
  fields: [{
    key: 'KCMOrder',
    label: 'KCM Order'
  }]
}, {
  key: 'base_spec',
  title: 'Base machine spec',
  fields: [{
    key: 'BaseSpec',
    label: 'Base spec'
  }, {
    key: 'Engine',
    label: 'Engine'
  }]
}, {
  key: 'boom',
  title: 'Boom',
  fields: [{
    key: 'Boom',
    label: 'Boom'
  }, {
    key: 'BoomNo',
    label: 'Boom no'
  }, {
    key: 'BoomName',
    label: 'Boom name'
  }]
}, {
  key: 'arm',
  title: 'Arm',
  fields: [{
    key: 'Arm',
    label: 'Arm'
  }, {
    key: 'ArmNo',
    label: 'Arm no'
  }, {
    key: 'ArmName',
    label: 'Arm name'
  }]
}, {
  key: 'front_att',
  title: 'Front ATT',
  fields: [{
    key: 'FrontATT',
    label: 'Front ATT'
  }, {
    key: 'BucketNo',
    label: 'Bucket no'
  }]
}, {
  key: 'country',
  title: 'Country Name',
  fields: [{
    key: 'CountryName',
    label: 'Country Name'
  }]
}, {
  key: 'other_piping',
  title: 'Other piping',
  fields: [{
    key: 'OtherPiping',
    label: 'Other piping'
  }]
}, {
  key: 'dignavi',
  title: 'DigNavi',
  fields: [{
    key: 'DigNavi',
    label: 'DigNavi'
  }]
}, {
  key: 'counter_weight',
  title: 'Counter weight',
  fields: [{
    key: 'CWNo',
    label: 'CW no'
  }, {
    key: 'CWName',
    label: 'CW name'
  }, {
    key: 'CWWeight',
    label: 'CW weight'
  }]
}, {
  key: 'it_device',
  title: 'IT device',
  fields: [{
    key: 'ITDevice',
    label: 'IT device'
  }, {
    key: 'ITControllerNo',
    label: 'IT controller No.'
  }, {
    key: 'ITController',
    label: 'IT controller (P/N)'
  }, {
    key: 'ITControllerSN',
    label: 'IT controller (S/N)'
  }]
}, {
  key: 'other',
  title: 'Other',
  fields: [{
    key: 'CabGuard',
    label: 'Cab guard'
  }, {
    key: 'Radio',
    label: 'Radio'
  }, {
    key: 'OtherOption',
    label: 'Other option'
  }, {
    key: 'Shoe',
    label: 'Shoe'
  }, {
    key: 'Seat',
    label: 'Seat'
  }, {
    key: 'HydOil',
    label: 'Hyd oil'
  }]
}];
function hasValue(v) {
  return v !== null && v !== undefined && String(v).trim() !== '' && v !== '-';
}
export function getRelevantSections(spec) {
  if (!spec) return [];
  return SECTION_DEFS.map(section => ({
    ...section,
    fields: section.fields.filter(f => hasValue(spec[f.key]))
  })).filter(section => section.fields.length > 0);
}
function loadAll() {
  try {
    return JSON.parse(localStorage.getItem(STORAGE_KEY)) || {};
  } catch {
    return {};
  }
}
function saveAll(all) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(all));
}
export function getApprovals(machineNo) {
  const all = loadAll();
  return all[machineNo] || {};
}
export function setApproval(machineNo, sectionKey, value) {
  const all = loadAll();
  const current = all[machineNo] || {};
  all[machineNo] = {
    ...current,
    [sectionKey]: value
  };
  saveAll(all);
  return all[machineNo];
}
export function computeStatus(spec, approvals) {
  const sections = getRelevantSections(spec);
  if (sections.length === 0) return 'PENDING';
  const hasRejected = sections.some(s => approvals[s.key] === 'rejected');
  if (hasRejected) return 'FIX';
  const allApproved = sections.every(s => approvals[s.key] === 'approved');
  if (allApproved) return 'OK';
  return 'PENDING';
}
export const STATUS_LABEL = {
  OK: 'OK',
  PENDING: 'รอตรวจสอบ',
  FIX: 'รอแก้ไข'
};
