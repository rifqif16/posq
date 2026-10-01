# POSQ

POS SaaS berbasis web untuk F&B. Spesifikasi: `POSQ.md` (sumber kebenaran produk).

Status: **7 dari 17 fase selesai**. Sudah ada: registrasi/login/refresh/profil, permission guard, kategori, produk sederhana dan bervarian, grup modifier beserta penautannya ke produk, dan riwayat harga. Shell admin di web: `/admin`, `/admin/products`, `/admin/modifiers`, `/admin/categories`.

## Prasyarat

- Go 1.26+ (toolchain otomatis mengunduh sesuai `go.mod`)
- Node 22+ dan pnpm
- Docker

## Menjalankan

```text
docker compose up -d postgres
cp .env.example .env
set -a; source .env; set +a
cd api && go run ./cmd/migrate up
cd api && go run ./cmd/api
cd web && pnpm install && pnpm dev
```

Buka <http://localhost:3000/register>, lalu <http://localhost:3000/admin/products> atau <http://localhost:3000/admin/modifiers>.

## Test

```text
cd api && go test ./...
cd api && TEST_DATABASE_URL='postgres://posq_app:posq_app_dev@localhost:5432/posq?sslmode=disable' go test -count=1 ./...
cd web && pnpm typecheck && pnpm test && pnpm build
```

## Struktur

Lihat `FILE_MANIFEST.md`. README tiap modul ada di `api/internal/<modul>/README.md`.
