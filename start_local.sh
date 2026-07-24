#!/bin/bash

# エラーが発生したらその場でスクリプトを即時中断させる設定（安全対策）
set -e

echo "🚀 SeventeenWalk ローカル開発環境の起動を開始します..."

# ローカル用の docker-compose.yml を使用してビルド＆バックグラウンド起動
echo "📦 イメージをビルドしてコンテナを起動しています..."
docker compose -f docker-compose.yml up --build -d

echo "✅ 起動が完了しました！"
echo "----------------------------------------"
echo "🌐 アクセス先："
echo "   http://localhost:5175  （フロント / Vite）※5173/5174 衝突回避"
echo "   http://localhost:8080  （backend API / WebSocket 直）"
echo "   http://localhost:8080/health"
echo "----------------------------------------"
echo "👉 ログを確認する場合："
echo "   docker compose -f docker-compose.yml logs -f"
echo "----------------------------------------"
echo "👉 停止する場合："
echo "   docker compose -f docker-compose.yml down"
echo "----------------------------------------"
