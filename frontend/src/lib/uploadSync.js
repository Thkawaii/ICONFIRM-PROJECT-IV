import { confirmDelete } from './toast.js';

// อัปโหลด "ไฟล์เดิม" (ชื่อไฟล์เดียวกัน): แถวที่ถูกลบออกจาก Excel จะถูกลบออกจากระบบด้วย
// ยกเว้นแถวที่สแกนผ่านแล้ว — ถามยืนยันก่อนทุกครั้งที่จะมีการลบ
export async function confirmUploadDeletes(summary, fileName) {
  const n = Number(summary?.deleted || 0);
  if (!n) return true;
  const kept = Number(summary?.deleteLocked || 0);
  let text = `ไฟล์ "${fileName}" ไม่มีข้อมูล ${n} แถวที่เคยอัปโหลดจากไฟล์นี้ ระบบจะลบ ${n} แถวนั้นออก`;
  if (kept) text += ` (อีก ${kept} แถวสแกนแล้ว จะไม่ถูกลบ)`;
  return confirmDelete({
    title: 'ยืนยันลบข้อมูลที่ไม่มีในไฟล์',
    text,
    confirmText: `อัปโหลดและลบ ${n} แถว`
  });
}

export function syncResultParts(result) {
  const parts = [];
  if (result?.deleted) parts.push(`ลบ ${result.deleted}`);
  if (result?.lockedKept) parts.push(`สแกนแล้ว ไม่ลบ ${result.lockedKept}`);
  return parts;
}
