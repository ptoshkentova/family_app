#!/usr/bin/env bash
# One-command local run: HTTPS tunnel -> update WEBAPP_URL in .env -> build Mini App -> run bot.
# Usage: ./dev.sh        Stop with Ctrl+C (the tunnel is closed too).
set -euo pipefail
cd "$(dirname "$0")"

for cmd in go bun cloudflared; do
  command -v "$cmd" >/dev/null || { echo "❌ '$cmd' topilmadi. O'rnating: brew install $cmd"; exit 1; }
done
[ -f .env ] || { echo "❌ .env yo'q. Avval: cp .env.example .env va BOT_TOKEN ni yozing"; exit 1; }

# Make sure PostgreSQL is running (Homebrew service).
if command -v brew >/dev/null && brew services list 2>/dev/null | grep -q '^postgresql@17 .*\(stopped\|none\|error\)'; then
  echo "🐘 PostgreSQL ishga tushirilmoqda..."
  brew services start postgresql@17 >/dev/null
  sleep 2
fi

TUNNEL_LOG=$(mktemp -t budget-tunnel)
cleanup() {
  [ -n "${TUNNEL_PID:-}" ] && kill "$TUNNEL_PID" 2>/dev/null || true
  rm -f "$TUNNEL_LOG"
}
trap cleanup EXIT INT TERM

echo "🌐 HTTPS tunnel ochilmoqda..."
cloudflared tunnel --no-autoupdate --url http://localhost:8080 >"$TUNNEL_LOG" 2>&1 &
TUNNEL_PID=$!

URL=""
for _ in $(seq 1 60); do
  URL=$(grep -o 'https://[a-z0-9-]*\.trycloudflare\.com' "$TUNNEL_LOG" | head -1 || true)
  [ -n "$URL" ] && break
  kill -0 "$TUNNEL_PID" 2>/dev/null || { echo "❌ Tunnel ishga tushmadi:"; cat "$TUNNEL_LOG"; exit 1; }
  sleep 0.5
done
[ -n "$URL" ] || { echo "❌ Tunnel manzili 30 soniyada chiqmadi"; exit 1; }

# Update (or add) WEBAPP_URL in .env.
if grep -q '^WEBAPP_URL=' .env; then
  sed -i '' "s#^WEBAPP_URL=.*#WEBAPP_URL=$URL#" .env
else
  echo "WEBAPP_URL=$URL" >> .env
fi
echo "✅ Mini App manzili: $URL"

echo "📦 Mini App build qilinmoqda..."
(cd webapp && { [ -d node_modules ] || bun install; } && bun run build >/dev/null)

echo "🤖 Bot ishga tushmoqda (to'xtatish: Ctrl+C)..."
go run ./cmd/server
