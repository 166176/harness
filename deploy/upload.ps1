param(
  [Parameter(Mandatory = $true)][string]$Ip,
  [string]$User = "root"
)
# 本地打包上传脚本：scp 镜像与 ECS 端脚本，然后执行 ECS 端配置
# 用法: powershell -File deploy/upload.ps1 -Ip 47.97.60.137
# SSH 密码请在终端提示时手动输入（不经过任何工具/日志）

$ErrorActionPreference = "Stop"

if (-not (Test-Path "harness.tar.gz")) {
  Write-Host "[upload] 未找到 harness.tar.gz，先执行: docker build; docker save ..." -ForegroundColor Yellow
  exit 1
}

Write-Host "[upload] 上传镜像到 ${User}@${Ip} ..."
scp harness.tar.gz "${User}@${Ip}:/tmp/harness.tar.gz"
scp deploy/ecs-setup.sh "${User}@${Ip}:/tmp/ecs-setup.sh"

Write-Host "[upload] 在 ECS 上执行部署脚本 ..."
ssh "${User}@${Ip}" "bash /tmp/ecs-setup.sh"

Write-Host "[upload] 完成。访问 http://${Ip}/ 验证。"
