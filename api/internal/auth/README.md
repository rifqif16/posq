# Auth Module

## Purpose

Identitas dan sesi: registrasi tenant baru (self-service), login email + password, rotasi refresh token, profil pengguna aktif, dan otorisasi berbasis permission.

## Responsibilities

- Validasi registrasi dan kredensial.
- Hash password (argon2id) dan penerbitan access token (JWT EdDSA, 15 menit).
- Refresh token berotasi dengan deteksi reuse (seluruh keluarga dicabut).
- Matriks permission per role (POSQ.md 4.2) dan guard `Require(permission)` untuk modul lain.

## Features

- `POST /v1/auth/register`: membuat tenant (trial), outlet pertama, dan owner, lalu membuka sesi
- `POST /v1/auth/login`
- `POST /v1/auth/refresh`
- `POST /v1/auth/logout` (idempoten)
- `GET /v1/me`
- Guard `Require(permission)`: 401 bila tidak terautentikasi, 403 bila role tidak punya permission

Belum ada (fase berikutnya): login PIN, manajemen user/perangkat, reset password, audit log, MFA, penegakan status tenant saat login, custom role.

## Structure

```text
auth/
├── README.md
├── module.go            # composition root
├── domain/              # aturan murni: email, password, registrasi, refresh token, permission
├── application/         # Service (use case) + port (Repository, PasswordHasher, TokenIssuer)
├── infrastructure/      # argon2id, JWT EdDSA
│   └── pg/              # Repository PostgreSQL (RLS)
└── interface/http/      # handler chi, RequireAuth, guard Require(permission), pemetaan error
```

## Dependencies

- `tenant` (domain + `infrastructure/pg`): pembuatan tenant dan outlet saat registrasi.
- `platform/database`, `platform/httpx`, `platform/ratelimit`, `platform/authn`.

## API / Public Interface

- `auth.New(Deps)` mengembalikan `*httpapi.Handler`; `Handler.Mount(chi.Router)` memasang rute di `/v1`.
- `Handler.Require(permission string)` adalah middleware untuk modul lain; mengisi `authn.Principal` di context (dibaca lewat `authn.From`).
- `domain.Can(role, permission)` adalah sumber kebenaran matriks permission.
- Error mengikuti problem+json (RFC 9457) dengan field `code`: `VALIDATION_FAILED`, `EMAIL_TAKEN`, `INVALID_CREDENTIALS`, `ACCOUNT_DISABLED`, `INVALID_REFRESH_TOKEN`, `UNAUTHORIZED`, `FORBIDDEN`, `RATE_LIMITED`.

## Data

`users`, `user_store_roles`, `refresh_tokens` (RLS per `tenant_id`). Fungsi `auth_find_user_by_email` (SECURITY DEFINER) adalah satu-satunya akses lintas tenant, untuk lookup login.

## Integrations

Tidak ada integrasi eksternal.

## Testing

- Unit: domain (termasuk matriks permission), application (repository fake), argon2, JWT.
- Integration: `internal/app/*_integration_test.go` (PostgreSQL nyata sebagai role `posq_app`): alur register → login → refresh → reuse → logout, isolasi RLS, dan otorisasi per role. Butuh `TEST_DATABASE_URL`.

## Constraints

- Refresh token berformat `<tenant_id>.<secret>`; hanya SHA-256 yang disimpan. Awalan tenant memungkinkan lookup ber-RLS tanpa fungsi lintas tenant.
- Email unik global (login tanpa memilih tenant).
- Login tak dikenal tetap menjalankan verifikasi hash dummy agar waktu respons seragam.
- Role diambil dari access token (role tertinggi pengguna); perubahan role berlaku setelah token berikutnya (maks. 15 menit). Role per outlet belum dievaluasi.
- Rate limit in-memory (10 percobaan/menit/IP pada register dan login); pindah ke Redis bila API lebih dari satu instans.
- Refresh paralel dari dua tab dengan cookie yang sama dianggap reuse oleh server; toleransi (grace window) belum ada.
- Modul lain tidak boleh membaca tabel auth langsung.
