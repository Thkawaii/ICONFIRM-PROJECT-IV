// ที่อยู่ของ backend
//
// ของเดิมตั้งค่าตายตัวเป็น http://localhost:8080 ซึ่งใช้ได้เฉพาะเครื่องที่รันเซิร์ฟเวอร์
// เครื่องอื่นเปิดเว็บแล้วเบราว์เซอร์จะไปเรียก localhost ของ "ตัวมันเอง" ซึ่งไม่มีอะไรรันอยู่
// จึงขึ้น Failed to fetch โดยที่ backend ไม่เคยเห็น request นั้นเลย
//
// ตอนนี้เลือกให้เองตามสภาพแวดล้อม ไม่ต้องตั้งค่าอะไรก่อนใช้งาน
//
//   ตั้ง VITE_API_BASE_URL ไว้   ใช้ค่านั้น (เผื่อ backend อยู่คนละเครื่องกับเว็บ)
//   โหมด dev (vite)             ยิงที่ /api แล้วให้ vite ส่งต่อให้ (ดู vite.config.js)
//   build แล้ว backend เสิร์ฟ    ยิงที่ origin เดียวกับหน้าเว็บ
//
// การยิงผ่าน /api ในโหมด dev แก้ปัญหา HTTPS ไปด้วยในตัว — vite เสิร์ฟหน้าเว็บเป็น
// https ส่วน backend เป็น http ถ้าเบราว์เซอร์ยิงตรงจะโดนบล็อกเพราะ mixed content
// แต่ถ้าผ่าน vite มันคุยกับ backend ฝั่งเซิร์ฟเวอร์ เบราว์เซอร์เห็นเป็น https ตลอด
const envBase = import.meta.env.VITE_API_BASE_URL;
export const API_BASE_URL = envBase !== undefined && envBase !== null && envBase !== '' ? envBase.replace(/\/+$/, '') : import.meta.env.DEV ? '/api' : '';
export function getToken() {
  return localStorage.getItem('iconfirm_token');
}
export async function apiFetch(path, options = {}) {
  const token = getToken();
  const headers = {
    'Content-Type': 'application/json',
    ...(options.headers || {})
  };
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }
  const res = await fetch(`${API_BASE_URL}${path}`, {
    ...options,
    headers
  });
  let data = null;
  try {
    data = await res.json();
  } catch {}
  if (!res.ok) {
    const base = data?.message || `Request failed (${res.status})`;
    const error = new Error(base);
    error.detail = data?.detail || '';
    error.status = res.status;
    error.data = data;
    throw error;
  }
  return data;
}
