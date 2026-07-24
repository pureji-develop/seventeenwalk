#!/bin/bash

# エラーが発生したらその場でスクリプトを即時中断させる設定（安全対策）
set -e

echo "🚀 [1/3] SeventeenWalk 本番用 Docker イメージのビルドを開始します..."

# 1. Nginx（リバースプロキシ）
echo "📦 Building Nginx..."
docker build -t ghcr.io/pureji-develop/seventeenwalk-nginx:latest ./nginx

# 2. フロント / バックエンド
echo "📦 Building Web Frontend (Production)..."
docker build -f ./frontend/Dockerfile.prod -t ghcr.io/pureji-develop/seventeenwalk-web:latest ./frontend

echo "📦 Building Backend (Production)..."
docker build -f ./backend/Dockerfile.prod -t ghcr.io/pureji-develop/seventeenwalk-backend:latest ./backend

echo "✅ [2/3] すべてのビルドが正常に完了しました！"
echo "⬆️ [3/3] GitHub Container Registry (GHCR) へプッシュします..."

docker push ghcr.io/pureji-develop/seventeenwalk-nginx:latest
docker push ghcr.io/pureji-develop/seventeenwalk-web:latest
docker push ghcr.io/pureji-develop/seventeenwalk-backend:latest

echo "🎉 全イメージのプッシュが完了しました！"
echo "----------------------------------------"
echo "👉 次は LightSail に SSH 接続し、以下を実行してください："
echo "   curl -sS https://seventeenwalk.taimanpoker.com/health"
echo "   cd /srv/seventeenwalk"
echo "   sudo docker compose -f docker-compose.prod.yml pull"
echo "   sudo docker compose -f docker-compose.prod.yml up -d --force-recreate"
echo "----------------------------------------"
echo "※ DB / JWT は不要（インメモリ対戦）"
echo "※ 任意: TOTAL_ROUNDS / BUILD_TIMEOUT_SEC / PORT"
echo "----------------------------------------"
