$ErrorActionPreference = 'Stop'

$to = 'theeparat.metheepooriwat@kobelco.com'

Write-Host '[1/3] กำลังต่อกับ Outlook บนเครื่องนี้...'
try {
    $outlook = New-Object -ComObject Outlook.Application
} catch {
    Write-Host ''
    Write-Host 'ต่อกับ Outlook ไม่ได้' -ForegroundColor Red
    Write-Host 'สาเหตุที่พบบ่อย:'
    Write-Host '  - เครื่องนี้ไม่ได้ติดตั้ง Outlook แบบเดสก์ท็อป (ใช้แต่ Outlook บนเว็บ ไม่นับ)'
    Write-Host '  - ติดตั้งเป็น Microsoft Store version ซึ่งไม่เปิด COM ให้เรียก'
    Write-Host "  - รายละเอียด: $($_.Exception.Message)"
    exit 1
}

Write-Host '[2/3] กำลังอ่านชื่อบัญชีที่ล็อกอินอยู่...'
$account = $outlook.Session.Accounts | Select-Object -First 1
if ($null -eq $account) {
    Write-Host ''
    Write-Host 'Outlook เปิดได้ แต่ยังไม่มีบัญชีในโปรไฟล์' -ForegroundColor Red
    Write-Host 'เปิด Outlook แล้วตั้งค่าบัญชีให้เรียบร้อยก่อน จากนั้นรันสคริปต์นี้ใหม่'
    exit 1
}
Write-Host "      บัญชีที่จะใช้ส่ง: $($account.SmtpAddress)"

Write-Host '[3/3] กำลังเขียนและส่งเมลทดสอบ...'
$mail = $outlook.CreateItem(0)
$mail.To = $to
$mail.Subject = '[I-CONFIRMATION] ทดสอบการส่งผ่าน Outlook'
$mail.HTMLBody = @'
<div style="font-family:Segoe UI,Tahoma,sans-serif;font-size:14px;">
  <p>ถ้าคุณได้รับอีเมลฉบับนี้ แปลว่าเครื่องนี้สั่ง Outlook ส่งเมลอัตโนมัติได้แล้ว</p>
  <p>ตั้ง <b>MAIL_PROVIDER=outlook</b> ใน backend/.env แล้วเปิด backend ได้เลย</p>
</div>
'@
$mail.Send()

Write-Host ''
Write-Host "ส่งแล้ว ลองเช็คกล่องจดหมายของ $to" -ForegroundColor Green
Write-Host 'ถ้ายังไม่เข้า ให้ดูที่โฟลเดอร์ Outbox — จดหมายจะออกตอน Outlook ซิงก์รอบถัดไป'
