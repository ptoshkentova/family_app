# Oilaviy byudjet boti

Xarajatlarni oddiy matn bilan yozadigan Telegram bot (`non 5 ming, taksi 20k, +oylik 6 mln`)
va hisobotlarni ko'rsatadigan Telegram Mini App.

- **Backend:** Go (bot va HTTP API bitta dasturda), PostgreSQL
- **Mini App:** React, Vite, Bun, TanStack Router va TanStack Query

```
Telegram ──► Go server (cmd/server)
               ├─ bot: xabarlarni qabul qiladi, parser, hisobotlar
               └─ /api/* ◄── Mini App (webapp/dist)
                    │
               PostgreSQL
```

## Talablar

Go 1.22+, Bun, PostgreSQL. macOS'da:

```bash
brew install go oven-sh/bun/bun postgresql@17 cloudflared
```

PostgreSQL'ni Docker orqali ishga tushirish ham mumkin: `docker compose up -d`.

## Ishga tushirish

1. `.env.example` faylidan `.env` nusxasini oling va `BOT_TOKEN` ni yozing.
2. Oila guruhida ishlashi uchun @BotFather'da `/setprivacy` → bot → **Disable** qiling.
3. Mini App'ni build qiling:

```bash
cd webapp && bun install && bun run build && cd ..
```

4. Telegram Mini App faqat HTTPS bilan ishlaydi. Lokal ishlab chiqish uchun tunnel oching va
   chiqqan `https://...trycloudflare.com` manzilini `.env` dagi `WEBAPP_URL` ga yozing:

```bash
cloudflared tunnel --url http://localhost:8080
```

5. Serverni ishga tushiring (migratsiyalar avtomatik bajariladi):

```bash
go run ./cmd/server
```

## Mini App'ni brauzerda ishlab chiqish

```bash
cd webapp && bun run dev
```

Telegram'dan tashqarida API'ni chaqirish uchun `.env` ga `DEV_AUTH_USER_ID=<sizning Telegram ID>`
qo'shing. **Faqat lokal ishlab chiqish uchun**, production'da hech qachon yoqmang.

## Testlar

```bash
go test ./...
```

Ma'lumotlar bazasi testlari uchun:

```bash
TEST_DATABASE_URL="postgres://localhost:5433/budget_test?sslmode=disable" go test ./...
```

## Tuzilishi

| Papka | Vazifasi |
|---|---|
| `cmd/server` | Kirish nuqtasi: bot va HTTP server |
| `internal/parser` | Matndan summa, kategoriya va daromadni ajratish |
| `internal/store` | PostgreSQL (pgx), embed qilingan SQL migratsiyalar |
| `internal/report` | Kunlik, haftalik va oylik hisobotlar (Toshkent vaqti) |
| `internal/telegram` | Bot buyruqlari: /bugun /hafta /oy /oxirgi /bekor /ochir |
| `internal/api` | Mini App API, Telegram `initData` imzosini tekshirish |
| `webapp` | React Mini App |

## Xavfsizlik

- Mini App har bir so'rovda `Authorization: tma <initData>` yuboradi. Server imzoni bot tokeni
  bilan tekshiradi va foydalanuvchi faqat o'zi a'zo bo'lgan byudjetlarni ko'radi.
- `.env` git'ga qo'shilmaydi. Token oshkor bo'lsa, @BotFather'da `/revoke` qiling.
