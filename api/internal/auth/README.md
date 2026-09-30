# Auth Module

## Purpose

Identitas dan sesi: registrasi tenant baru (self-service), login email + password, rotasi refresh token, dan profil pengguna aktif.

## Responsibilities

- Validasi registrasi dan kredensial.
- Hash password (argon2id) dan penerbitan access token (JWT EdDSA, 15 menit).
- Refresh token berotasi dengan deteksi reuse (seluruh keluarga dicabut).
- Middleware `RequireAuth` untuk modul lain.

## Features

- `POST /v1/auth/register`: membuat tenant (trial), outlet pertama, dan owner, lalu membuka sesi
- `POST /v1/auth/login`
- `POST /v1/auth/refresh`
- `POST /v1/auth/logout` (idempoten)
- `GET /v1/me`

Belum ada (fase berikutnya): login PIN, manajemen user/perangkat, reset password, audit log, MFA, penegakan status tenant saat login.

## Structure

```text
auth/
├── README.md
├── module.go            # composition root
├── domain/              # aturan murni: email, password, registrasi, format refresh token
├── application/         # Service (use case) + port (Repository, PasswordHasher, TokenIssuer)
├── infrastructure/      # argon2id, JWT EdDSA
│   └── pg/              # Repository PostgreSQL (RLS)
└── interface/http/      # handler chi, middleware RequireAuth, pemetaan error
```

## Dependencies

- `tenant` (domain + `infrastructure/pg`): pembuatan tenant dan outlet saat registrasi.
- `platform/database`, `platform/httpx`, `platform/ratelimit`.

## API / Public Interface

- `auth.New(Deps)` mengembalikan `*httpapi.Handler`; `Handler.Mount(chi.Router)` memasang rute di `/v1`.
- `httpapi.ClaimsFrom(ctx)` membaca klaim setelah `RequireAuth`.
- Error mengikuti problem+json (RFC 9457) dengan field `code`: `VALIDATION_FAILED`, `EMAIL_TAKEN`, `INVALID_CREDENTIALS`, `ACCOUNT_DISABLED`, `INVALID_REFRESH_TOKEN`, `UNAUTHORIZED`, `RATE_LIMITED`.

## Data

`users`, `user_store_roles`, `refresh_tokens` (RLS per `tenant_id`). Fungsi `auth_find_user_by_email` (SECURITY DEFINER) adalah satu-satunya akses lintas tenant, untuk lookup login.

## Integrations

Tidak ada integrasi eksternal.

## Testing

- Unit: domain, application (repository fake), argon2, JWT.
- Integration: `internal/app/integration_test.go` (PostgreSQL nyata sebagai role `posq_app`): alur register → login → refresh → reuse → logout, dan isolasi RLS. Butuh `TEST_DATABASE_URL`.

## Constraints

- Refresh token berformat `<tenant_id>.<secret>`; hanya SHA-256 yang disimpan. Awalan tenant memungkinkan lookup ber-RLS tanpa fungsi lintas tenant.
- Email unik global (login tanpa memilih tenant).
- Login tak dikenal tetap menjalankan verifikasi hash dummy agar waktu respons seragam.
- Rate limit in-memory (10 percobaan/menit/IP pada register dan login); pindah ke Redis bila API lebih dari satu instans.
- Modul lain tidak boleh membaca tabel auth langsung.
