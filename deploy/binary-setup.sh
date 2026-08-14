#!/usr/bin/env bash
# ECS 端二进制部署脚本（备用方案：不依赖 Docker，systemd 托管 gavel serve）
# 用法: 本机构建 gavel-linux 后 scp 到 /opt/gavel/gavel-linux，再执行本脚本
set -euo pipefail

BIN=/opt/gavel/gavel-linux
ENV=/opt/gavel/.env

echo "==> [1/4] 校验二进制"
if [ ! -x "$BIN" ]; then
  echo "未找到 $BIN，请先: 本机 go build 后 scp 上传"
  exit 1
fi
"$BIN" version || true

echo "==> [2/4] 准备密钥文件"
mkdir -p /opt/gavel
if [ ! -s "$ENV" ]; then
  echo "请创建 $ENV 并写入: GAVEL_API_KEY=sk-你的key"
  echo "建议: touch $ENV && chmod 600 $ENV && cat > $ENV（Ctrl+D 结束，避免进 shell history）"
  exit 1
fi
chmod 600 "$ENV"

echo "==> [3/4] 写入 systemd 服务"
cat > /etc/systemd/system/gavel.service <<EOF
[Unit]
Description=Gavel Coding Agent Harness
After=network-online.target

[Service]
Type=simple
ExecStart=$BIN serve
EnvironmentFile=$ENV
Environment=PORT=80
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF

echo "==> [4/4] 启动并开机自启"
systemctl daemon-reload
systemctl enable --now gavel
systemctl restart gavel
sleep 2
systemctl status gavel --no-pager | head -8

echo "==> 完成。验证: curl http://127.0.0.1/api/key/status"
