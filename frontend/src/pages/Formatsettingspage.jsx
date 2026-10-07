import { useMemo, useState } from 'react';
import AppShell from '../components/AppShell.jsx';
import SelectField from '../components/Selectfield.jsx';
import { ColumnAliasPanel, CodeAliasPanel } from '../components/FormatTools.jsx';
import '../components/FormatTools.css';
import { ADMIN_NAV_ITEMS } from './AdminDashboardpage.jsx';

// คอลัมน์มาตรฐานของแต่ละไฟล์ที่อัปโหลดได้จริงตอนนี้
// key ของแต่ละงาน (scope) ต้องตรงกับ dataset/ตารางที่ backend อ่าน:
//   engine_it_allocation → Planning WH   (upload_data.go › DatasetEngineITAlloc)
//   spec_sheet           → Planning MFG  (upload_data.go › DatasetSpecSheet)
//   cw_cv_its            → Master Data (upload_data.go › DatasetCWCVITS)
//   import_license       → License (Import / Export) (import_license_item.go)
//   renewal_license      → Renewal License (license_renewal_history.go)
const TARGET_COLUMNS = {
  engine_it_allocation: [
    "Lot no.",
    "Machine S/N",
    "Customer name",
    "Main line",
    "Engine Part no.",
    "Engine Serial no.",
    "Engine Serial no. mistake",
    "Engine Tag no.",
    {value: "Engine Invoice no.", label: "Engine 1st Invioce no."},
    "Engine Remark",
    "Delivery ST",
    "Delivery ST (2)",
    "IT Part no.",
    "IT Serial no.",
    "IT Serial no. mistake",
    "IT Tag no.",
    {value: "IT Invoice no.", label: "IT 1st Invioce no."},
    "IT Remark"
  ],
  spec_sheet: [
    "Line",
    "LOT NO.",
    "Machine",
    "Product Spec",
    {value: "Product Spec 2", label: "Product Spec (2)"},
    "Domestic/Exp",
    "Assembly Stetu",
    "Shipping Stetu",
    "LINE ON",
    "Assembly P",
    "Assemb",
    "Shipping P",
    "Etd.Shipme",
    "SHIPPI",
    "Sell",
    "Divisi",
    "Selling comp",
    "KCM Order",
    "Delive",
    "User",
    "Country",
    "Country Name",
    "Brand",
    "Destination",
    "Purpose",
    "Upp, lower spec.",
    "Front ATT piping",
    "Other piping",
    "Base machine spec.",
    "Cab base",
    "Cab",
    "Boom",
    "Piping, boom",
    "Arm",
    "Piping, arm",
    "Shoe",
    "Counter weight",
    "Lower ATT",
    "Lever",
    "Multi control",
    "Air conditioner",
    "Cold region spec.",
    "Auto greasing system",
    "Seat",
    "Travel alarm",
    "Gauge cluster",
    "Radio",
    "Engine start key",
    "DigNavi",
    "Paint",
    "Front ATT",
    "Cab guard",
    "IT device",
    "Other option",
    "Additional ATT",
    "Cold region spec(HYD oil)",
    "Shoe option",
    "Manufacturer options",
    "Note1",
    "Note2",
    "Note3"
  ],
  cw_cv_its: [
    "Product Spec",
    "CW Part No",
    "Name",
    "Weight (g)",
    "Weight (Tons)",
    "BOOM Part No",
    "BOOM Part Name",
    "ARM Part No",
    "ARM Part Name",
    "CV Part No",
    "SM Part No",
    "Motor Propel Part No",
    "Pump Hydrolics Part No"
  ],
  import_license: [
    "ลำดับ",
    "PART NO.",
    "ตราอักษร",
    "แบบ/รุ่น",
    "เลขใบอนุญาตนำเข้า",
    "วันที่ออกใบอนุญาต",
    "เลขอินวอยซ์นำเข้า",
    "เลขใบขนสินค้าขาเข้า",
    "จำนวน (เครื่อง )",
    "หมายเลขเครื่อง",
    "หมายเลขการผลิต",
    "เลขใบอนุญาตนำออก",
    {value: "วันที่ออกใบอนุญาตนำออก", label: "วันที่ออกใบอนุญาต (นำออก)"},
    "ส่งออกไปประเทศ"
  ],
  renewal_license: [
    "NO.",
    "IT CONTROLLER MODEL",
    "IMPORT LICENSE IT CONTROLLER NO.",
    "COUNTRY",
    "TOTAL",
    "EXPORT LICENSE IT CONTROLLER NO.",
    "EXPORT LICENSE IT CONTROLLER DATE",
    "EXPORT LICENSE IT CONTROLLER EXPIRE DATE",
    "STOCK EXPORT LICENSE",
    "REMAIN",
    "Date of E-mail Sending",
    "Payment Date",
    "Received Document Date"
  ]
};

// ตัวเลือกใน dropdown "เลือกไฟล์ / งานที่ต้องการตั้งค่า" — มีแค่ 5 รายการนี้
const SCOPES = [{
  scope: 'engine_it_allocation',
  label: 'Planning WH'
}, {
  scope: 'spec_sheet',
  label: 'Planning MFG'
}, {
  scope: 'cw_cv_its',
  label: 'Master Data'
}, {
  scope: 'import_license',
  label: 'License (Import / Export)'
}, {
  scope: 'renewal_license',
  label: 'Renewal License'
}];
export default function FormatSettingsPage() {
  const isAdmin = (localStorage.getItem('iconfirm_role') || '').toUpperCase() === 'ADMIN';
  const navItems = ADMIN_NAV_ITEMS;
  const shellRoleLabel = isAdmin ? 'Admin' : 'Upload View';
  const [scope, setScope] = useState(SCOPES[0].scope);
  const active = SCOPES.find(s => s.scope === scope) || SCOPES[0];
  const targetOptions = TARGET_COLUMNS[active.scope] || [];
  const scopeOptions = useMemo(() => SCOPES.map(s => ({
    value: s.scope,
    label: s.label
  })), []);
  return <AppShell navItems={navItems} roleLabel={shellRoleLabel}>
      <div className="fmt-page">
        <div className="fmt-page-head">
          <h2 style={{
          fontWeight: 100,
          fontSize: 22,
          color: 'var(--color-brand-ink, #06312f)',
          margin: 0
        }}>
            ตั้งค่า Format
          </h2>
          <span style={{
          fontSize: 12.5,
          color: '#94a3b8'
        }}>
            มีผลกับการอัปโหลดครั้งถัดไป (ไม่ย้อนหลังข้อมูลเดิม)
          </span>
        </div>

        <div className="fmt-card">
          <label className="fmt-label" style={{
          display: 'block',
          marginBottom: 8
        }}>
            เลือกไฟล์ / งานที่ต้องการตั้งค่า
          </label>
          <div style={{
          maxWidth: 440
        }}>
            <SelectField value={scope} onChange={setScope} options={scopeOptions} />
          </div>
        </div>

        <div className="fmt-card">
          <h3 className="fmt-card-title">หัวคอลัมน์เปลี่ยนชื่อ / เพิ่มใหม่ / สลับตำแหน่ง</h3>
          <ColumnAliasPanel scope={active.scope} targetOptions={targetOptions} embedded />
        </div>

        <div className="fmt-card">
          <h3 className="fmt-card-title">Change Format Part</h3>
          <CodeAliasPanel componentType="it_controller" embedded />
        </div>
      </div>
    </AppShell>;
}
