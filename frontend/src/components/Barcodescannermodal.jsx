import { useEffect, useRef, useState } from 'react';
const SCANNER_ELEMENT_ID = 'barcode-scanner-viewport';
export default function BarcodeScannerModal({
  title,
  onDetected,
  onClose
}) {
  const scannerRef = useRef(null);
  const fileInputRef = useRef(null);
  const [error, setError] = useState('');
  const [starting, setStarting] = useState(true);
  useEffect(() => {
    let cancelled = false;
    let html5Qrcode;
    async function start() {
      try {
        const {
          Html5Qrcode,
          Html5QrcodeSupportedFormats
        } = await import('https://cdn.jsdelivr.net/npm/html5-qrcode@2.3.8/+esm');
        if (cancelled) return;
        const formatsToSupport = [Html5QrcodeSupportedFormats.DATA_MATRIX, Html5QrcodeSupportedFormats.QR_CODE, Html5QrcodeSupportedFormats.CODE_128, Html5QrcodeSupportedFormats.CODE_39, Html5QrcodeSupportedFormats.EAN_13];
        html5Qrcode = new Html5Qrcode(SCANNER_ELEMENT_ID, {
          formatsToSupport,
          experimentalFeatures: {
            useBarCodeDetectorIfSupported: true
          },
          verbose: false
        });
        scannerRef.current = html5Qrcode;
        await html5Qrcode.start({
          facingMode: 'environment'
        }, {
          fps: 10,
          qrbox: {
            width: 240,
            height: 240
          }
        }, decodedText => {
          if (navigator.vibrate) navigator.vibrate(120);
          onDetected(decodedText);
        }, () => {});
        if (!cancelled) setStarting(false);
      } catch (err) {
        if (!cancelled) {
          setError(err?.message?.includes('Permission') ? 'กรุณาอนุญาตให้ใช้กล้องก่อนสแกน' : 'เปิดกล้องไม่สำเร็จ ลองใหม่ หรือใช้ปุ่ม "สแกนจากรูปถ่าย" แทน');
          setStarting(false);
        }
      }
    }
    start();
    return () => {
      cancelled = true;
      if (html5Qrcode) {
        html5Qrcode.stop().then(() => html5Qrcode.clear()).catch(() => {});
      }
    };
  }, [onDetected]);
  async function handleScanPhoto(e) {
    const file = e.target.files?.[0];
    if (!file) return;
    setError('');
    try {
      const {
        Html5Qrcode,
        Html5QrcodeSupportedFormats
      } = await import('https://cdn.jsdelivr.net/npm/html5-qrcode@2.3.8/+esm');
      if (scannerRef.current) {
        await scannerRef.current.stop().catch(() => {});
        await scannerRef.current.clear().catch(() => {});
        scannerRef.current = null;
      }
      const fileScanner = new Html5Qrcode(SCANNER_ELEMENT_ID, {
        formatsToSupport: [Html5QrcodeSupportedFormats.DATA_MATRIX, Html5QrcodeSupportedFormats.QR_CODE, Html5QrcodeSupportedFormats.CODE_128, Html5QrcodeSupportedFormats.CODE_39, Html5QrcodeSupportedFormats.EAN_13],
        verbose: false
      });
      const decodedText = await fileScanner.scanFile(file, false);
      await fileScanner.clear().catch(() => {});
      if (navigator.vibrate) navigator.vibrate(120);
      onDetected(decodedText);
    } catch (err) {
      setError('อ่านโค้ดจากรูปไม่สำเร็จ — ลองถ่ายให้ชัด/ตรง/ใกล้ขึ้น แล้วลองใหม่');
    } finally {
      if (fileInputRef.current) fileInputRef.current.value = '';
    }
  }
  return <div className="wh-modal-overlay" onClick={onClose}>
      <div className="scanner-modal" onClick={e => e.stopPropagation()}>
        <h3 className="wh-modal-title">{title}</h3>

        <div id={SCANNER_ELEMENT_ID} className="scanner-viewport" />

        {starting && !error && <p className="wh-subtitle">กำลังเปิดกล้อง...</p>}
        {error && <p className="form-error" role="alert">
            {error}
          </p>}

        <div className="wh-modal-actions">
          <button className="wh-issue-btn" type="button" onClick={() => fileInputRef.current?.click()}>
            สแกนจากรูปถ่าย
          </button>
          <button className="wh-modal-cancel" onClick={onClose}>
            ปิด
          </button>
        </div>

        <input ref={fileInputRef} type="file" accept="image/*" capture="environment" style={{
        display: 'none'
      }} onChange={handleScanPhoto} />
      </div>
    </div>;
}
