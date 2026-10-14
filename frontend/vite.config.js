import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import basicSsl from '@vitejs/plugin-basic-ssl'

export default defineConfig({
  plugins: [react(), tailwindcss(), basicSsl()],
  server: {
    host: '0.0.0.0',
    // ให้ตรงกับที่รันจริง (npm run dev) จะได้ไม่ต้องพิมพ์ --port ทุกครั้ง
    port: 5173,
    https: true,
    proxy: {
      // ทุก request ของหน้าเว็บวิ่งเข้า /api แล้ว vite ส่งต่อให้ backend
      //
      // ข้อดีคือเครื่องอื่นในวงแลนเปิดได้เลยโดยไม่ต้องรู้ IP ของ backend
      // เพราะเบราว์เซอร์คุยกับ vite ที่เครื่องเดียวกับที่มันเปิดเว็บอยู่แล้ว
      // ส่วน vite ไปคุยกับ backend ต่อให้ฝั่งเซิร์ฟเวอร์
      //
      // VITE_API_PROXY_TARGET ตั้งได้ถ้า backend ไม่ได้อยู่เครื่องเดียวกับ vite
      '/api': {
        target: process.env.VITE_API_PROXY_TARGET || 'http://localhost:8080',
        changeOrigin: true,
        // backend ไม่มี prefix /api ต้องตัดออกก่อนส่งต่อ
        rewrite: (path) => path.replace(/^\/api/, ''),
      },
    },
  },
})
