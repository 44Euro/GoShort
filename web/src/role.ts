// เซิร์ฟเวอร์แทนค่านี้ลงใน shell ครั้งเดียวตอน boot ทำให้ SPA รู้บทบาทของ instance
// ตั้งแต่ไบต์แรก โดยไม่ต้องเสีย round trip ถาม API บนหน้าที่ต้องเร็วที่สุด
// dev server ของ Vite เสิร์ฟ index.html ดิบ placeholder จึงยังอยู่ ให้ถือว่าเปิด
export const adminEnabled =
  document.querySelector('meta[name="goshort-admin"]')?.getAttribute("content") !== "0";
