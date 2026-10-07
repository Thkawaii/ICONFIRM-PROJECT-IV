import { useState } from 'react';
import FileDropZone from '../components/Filedropzone.jsx';
export default function UiKitPage() {
  const [file, setFile] = useState(null);
  return <div className="min-h-screen bg-slate-100 font-sans">
      <header className="bg-brand-500 px-6 py-5">
        <p className="font-mono text-[11px] tracking-[0.14em] text-white/70 uppercase">
          KOBELCO I-CONFIRM
        </p>
        <h1 className="mt-1 text-xl font-bold text-white">ชุดส่วนประกอบหน้าเว็บ</h1>
      </header>

      <main className="mx-auto grid max-w-5xl gap-6 px-5 py-8">
        <Section title="ปุ่ม" note="ปุ่มหลัก 1 ปุ่มต่อหน้าจอเท่านั้น ที่เหลือใช้ปุ่มรอง">
          <div className="flex flex-wrap items-center gap-3">
            <button className="ui-btn ui-btn-primary">บันทึกการยืนยัน</button>
            <button className="ui-btn ui-btn-soft">ดูรายละเอียด</button>
            <button className="ui-btn ui-btn-ghost">ยกเลิก</button>
            <button className="ui-btn ui-btn-danger">แจ้งไม่ผ่าน</button>
            <button className="ui-btn ui-btn-primary" disabled>
              กำลังบันทึก…
            </button>
          </div>
          <div className="mt-3 flex flex-wrap items-center gap-3">
            <button className="ui-btn ui-btn-sm ui-btn-ghost">ขนาดเล็ก</button>
            <button className="ui-btn ui-btn-lg ui-btn-primary">ขนาดใหญ่</button>
          </div>
        </Section>

        <Section title="กรอบแจ้งเตือน" note="แถบสีด้านซ้ายบอกสถานะ อ่านได้เร็วกว่าไอคอน">
          <div className="grid gap-3">
            <div className="ui-alert ui-alert-ok">
              <span className="ui-alert-icon">
                <CheckCircleIcon className="size-5" />
              </span>
              <div>
                <p className="ui-alert-title">อัปโหลดสำเร็จ</p>
                <p>เพิ่มรายการเข้าระบบ 128 รายการ ตรงกับบัญชีใบอนุญาตทั้งหมด</p>
              </div>
            </div>
            <div className="ui-alert ui-alert-warn">
              <span className="ui-alert-icon">
                <ExclamationTriangleIcon className="size-5" />
              </span>
              <div>
                <p className="ui-alert-title">มี 3 รายการรอตรวจซ้ำ</p>
                <p>Serial ซ้ำกับล็อตก่อนหน้า ตรวจสอบก่อนยืนยันจ่ายของ</p>
              </div>
            </div>
            <div className="ui-alert ui-alert-bad">
              <span className="ui-alert-icon">
                <XCircleIcon className="size-5" />
              </span>
              <div>
                <p className="ui-alert-title">อ่านไฟล์ไม่ได้</p>
                <p>คอลัมน์ Part No. หายไปจากแถวที่ 1 — ใช้เทมเพลตล่าสุดแล้วอัปโหลดใหม่</p>
              </div>
            </div>
            <div className="ui-alert ui-alert-info">
              <span className="ui-alert-icon">i</span>
              <div>รองรับไฟล์ .xlsx และ .xls ขนาดไม่เกิน 10 MB</div>
            </div>
          </div>
        </Section>

        <Section title="อัปโหลดไฟล์" note="ลากมาวางได้ทั้งกล่อง หรือกดเพื่อเลือก">
          <FileDropZone file={file} onSelect={setFile} accept=".xlsx,.xls" label="อัปโหลดบัญชีใบอนุญาตนำเข้า" />
        </Section>

        <Section title="ช่องกรอก">
          <div className="grid gap-4 sm:grid-cols-2">
            <div>
              <label className="ui-label" htmlFor="uk-a">
                Machine No.
              </label>
              <input id="uk-a" className="ui-input" placeholder="เช่น SK200-10" />
              <p className="ui-hint">พิมพ์หรือยิงบาร์โค้ดเข้าช่องนี้ได้เลย</p>
            </div>
            <div>
              <label className="ui-label" htmlFor="uk-b">
                ชนิดพาร์ท
              </label>
              <select id="uk-b" className="ui-select">
                <option>Control Valve</option>
                <option>Swing Motor</option>
                <option>IT Controller</option>
              </select>
            </div>
          </div>
        </Section>

        <Section title="ป้ายสถานะ">
          <div className="flex flex-wrap gap-2">
            <span className="ui-badge ui-badge-ok">ยืนยันแล้ว</span>
            <span className="ui-badge ui-badge-warn">รอสแกน</span>
            <span className="ui-badge ui-badge-bad">ไม่ตรงบัญชี</span>
            <span className="ui-badge ui-badge-brand">ล็อตปัจจุบัน</span>
            <span className="ui-badge ui-badge-muted">ยกเลิก</span>
          </div>
        </Section>

        <Section title="ตาราง">
          <div className="ui-table-card">
            <table className="ui-table">
              <thead>
                <tr>
                  <th>Part No.</th>
                  <th>Serial</th>
                  <th>สถานะ</th>
                </tr>
              </thead>
              <tbody>
                {[['YN22V00001F1', '2401A00873', 'ok'], ['LC15V00025S1', '2401A00874', 'warn'], ['YX53D00001P1', '2401A00901', 'bad']].map(([part, sn, state]) => <tr key={sn}>
                    <td className="ui-mono font-semibold text-slate-900">{part}</td>
                    <td className="ui-mono">{sn}</td>
                    <td>
                      <span className={`ui-badge ui-badge-${state}`}>
                        {state === 'ok' ? 'ยืนยันแล้ว' : state === 'warn' ? 'รอสแกน' : 'ไม่ตรงบัญชี'}
                      </span>
                    </td>
                  </tr>)}
              </tbody>
            </table>
          </div>
        </Section>
      </main>
    </div>;
}
function Section({
  title,
  note,
  children
}) {
  return <section className="ui-card ui-card-pad">
      <h2 className="ui-title text-base">{title}</h2>
      {note && <p className="ui-subtitle mb-4">{note}</p>}
      {!note && <div className="mb-4" />}
      {children}
    </section>;
}
