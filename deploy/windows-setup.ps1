# ECS Windows Server 2022 部署脚本（需管理员权限运行）
# 用法: 先 scp gavel.exe 与本脚本到服务器，然后 powershell -ExecutionPolicy Bypass -File windows-setup.ps1
$ErrorActionPreference = "Stop"

$dir = "C:\gavel"
New-Item -ItemType Directory -Force -Path $dir | Out-Null
Copy-Item -Path (Join-Path $PSScriptRoot "gavel.exe") -Destination (Join-Path $dir "gavel.exe") -Force

# 密钥文件（明文落盘，SPEC §4.2 已声明；由用户本人创建，避免经手）
$envFile = Join-Path $dir ".env"
if (-not (Test-Path $envFile)) {
  Write-Host "=== 需要创建密钥文件 $envFile ===" -ForegroundColor Yellow
  Write-Host "内容两行（注意换行）:"
  Write-Host "    GAVEL_API_KEY=sk-你的key"
  Write-Host "    PORT=80"
  Write-Host "创建命令（自己执行，勿经他人/工具）:"
  Write-Host "    notepad $envFile"
  exit 1
}

# 注册开机自启计划任务（以管理员运行）
$action = New-ScheduledTaskAction -Execute "$dir\gavel.exe" -Argument "serve" -WorkingDirectory $dir
try { Unregister-ScheduledTask -TaskName "gavel" -Confirm:$false -ErrorAction SilentlyContinue } catch {}
Register-ScheduledTask -TaskName "gavel" -Action $action -Trigger (New-ScheduledTaskTrigger -AtStartup) -RunLevel Highest | Out-Null
Start-ScheduledTask -TaskName "gavel"
Write-Host "任务已注册并启动: gavel"

Start-Sleep -Seconds 3
try {
  $r = Invoke-WebRequest -UseBasicParsing "http://127.0.0.1/api/key/status" -TimeoutSec 5
  Write-Host "本机自检: HTTP $($r.StatusCode)" -ForegroundColor Green
} catch {
  Write-Host "服务未响应（可能仍在启动）: $($_.Exception.Message)" -ForegroundColor Yellow
  Write-Host "查看日志: Get-ScheduledTask -TaskName gavel | Select * ; 或运行 C:\gavel\gavel.exe serve 前台调试"
}
