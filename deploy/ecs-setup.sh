#!/usr/bin/env bash
# ECS 端一键部署脚本（在服务器上执行）
# 由本地 deploy/upload.ps1 上传后调用；也可手动执行: bash /tmp/ecs-setup.sh
set -euo pipefail

echo "==> [1/4] 安装 Docker（阿里云镜像）"
if ! command -v docker >/dev/null 2>&1; then
  curl -fsSL https://get.docker.com | bash -s docker --mirror Aliyun
  systemctl enable --now docker
fi
docker --version

echo "==> [2/4] 载入镜像（docker load 支持 gzip）"
docker load -i /tmp/harness.tar.gz
docker images | grep -E 'harness|REPOSITORY'

echo "==> [3/4] 准备密钥文件 /opt/gavel/.env"
mkdir -p /opt/gavel
if [ ! -s /opt/gavel/.env ]; then
  echo "请先手动创建 /opt/gavel/.env 并写入一行:"
  echo "    GAVEL_API_KEY=sk-你的key"
  echo "命令:  touch /opt/gavel/.env && chmod 600 /opt/gavel/.env && cat > /opt/gavel/.env"
  exit 1
fi
chmod 600 /opt/gavel/.env

echo "==> [4/4] 启动容器（80 -> 8080，开机自启）"
docker rm -f harness >/dev/null 2>&1 || true
docker run -d --name harness \
  -p 80:8080 \
  --env-file /opt/gavel/.env \
  --restart unless-stopped \
  harness:latest

sleep 2
docker ps | grep harness
echo "==> 部署完成，访问 http://$(hostname -I | awk '{print $1}')/ 验证"
