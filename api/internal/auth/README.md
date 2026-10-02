# Auth Module

## Purpose

Identitas dan sesi: registrasi tenant baru (self-service), login email + password, rotasi refresh token, profil pengguna aktif, dan manajemen staf (akun, role, outlet, password, PIN).

## Responsibilities

- Validasi registrasi dan kredensial.
- Hash password dan PIN (argon2id) dan penerbitan access token (JWT EdDSA, 15 menit).
- Refresh token berotasi dengan deteksi reuse (seluruh keluarga dicabut).
- Middleware `RequireAuth` dan `Require(permission)` untuk modul lain.
- Manajemen staf: membuat akun, mengubah nama/role/outlet, menonaktifkan, reset password, mengatur PIN, menegakkan batas pengguna paket.

## Features

- `POST /v1/auth/register`: membuat tenant (trial), outlet pertama, dan owner, lalu membuka sesi
- `POST /v1/auth/login`
- `POST /v1/auth/refresh`
- `POST /v1/auth/logout` (idempoten)
- `GET /v1/me`
- `GET /v1/stores` (`user:manage`): outlet tenant
- `GET /v1/staff` (`user:manage`): semua pengguna tenant, owner dulu; tiap item berisi `manageable` (apakah pemanggil boleh mengelolanya) dan respons memuat `assignable_roles`
- `POST /v1/staff` (`user:manage`): `name`, `email`, `password`, `role` (`admin`/`cashier`/`kitchen`), `store_ids`, `pin` opsional
- `PATCH /v1/staff/{id}` (`user:manage`): `name`, `role`, `store_ids`, `status` (`active`/`disabled`); menonaktifkan mencabut semua refresh token
- `POST /v1/staff/{id}/password` (`user:manage`): reset password; mencabut semua refresh token
- `PUT /v1/staff/{id}/pin`, `DELETE /v1/staff/{id}/pin` (`user:manage`): atur atau hapus PIN 6 digit

Aturan staf: owner mengelola admin, kasir, dan dapur; admin hanya kasir; owner tidak dapat diubah dan tidak ada yang mengelola dirinya sendiri lewat endpoint ini. Admin hanya boleh memberi outlet yang dia anggotai. Batas `plans.limits.users` dihitung dari pengguna aktif; melebihi batas menghasilkan `PLAN_LIMIT_REACHED`. PIN lemah (`000000`, `123456`, dan sejenisnya) ditolak.

Belum ada: login PIN dan registrasi perangkat (fase sync), audit log, undangan dan reset password lewat email, ubah email, profil diri, MFA, penegakan status tenant saat login.

## Structure

```text
auth/
├── README.md
├── module.go            # composition root auth: New(Deps) -> *Handler
├── staff_module.go      # composition root staf: NewStaff(Deps) -> *StaffHandler
├── domain/              # email, password, registrasi, refresh token, permission, aturan staf (staff.go)
├── application/         # Service, StaffService (staff.go) + port (Repository, StaffRepository, PasswordHasher, TokenIssuer)
├── infrastructure/      # argon2id, JWT EdDSA
│   └── pg/              # repository.go (auth), staff.go (staf); PostgreSQL (RLS)
└── interface/http/      # handler.go, guard.go, staff.go
```

## Dependencies

- `tenant` (domain + `infrastructure/pg`): pembuatan tenant dan outlet saat registrasi.
- `platform/database`, `platform/httpx`, `platform/ratelimit`, `platform/authn`.

## API / Public Interface

- `auth.New(Deps)` mengembalikan `*httpapi.Handler`; `Handler.Mount(chi.Router)` memasang rute auth di `/v1`; `Handler.Require(permission)` dipakai modul lain sebagai guard.
- `auth.NewStaff(Deps)` mengembalikan `*httpapi.StaffHandler`; `Mount(r, guard)` memasang rute staf.
- `httpapi.ClaimsFrom(ctx)` membaca klaim setelah `RequireAuth`.
- Error mengikuti problem+json (RFC 9457) dengan field `code`: `VALIDATION_FAILED`, `EMAIL_TAKEN`, `INVALID_CREDENTIALS`, `ACCOUNT_DISABLED`, `INVALID_REFRESH_TOKEN`, `UNAUTHORIZED`, `RATE_LIMITED`, `FORBIDDEN`, `ROLE_NOT_ALLOWED`, `INVALID_STORE`, `PLAN_LIMIT_REACHED`, `NOT_FOUND`.

## Data

`users`, `user_store_roles`, `refresh_tokens` (RLS per `tenant_id`). Fungsi `auth_find_user_by_email` (SECURITY DEFINER) adalah satu-satunya akses lintas tenant, untuk lookup login. Aplikasi punya hak `DELETE` pada `user_store_roles` (melepas outlet). Role disimpan per outlet; pengguna dengan beberapa outlet memakai satu role yang sama di semuanya.

## Integrations

Tidak ada integrasi eksternal.

## Testing

- Unit: domain (termasuk aturan role dan PIN), application (repository fake, termasuk matriks otorisasi staf), argon2, JWT.
- Integration: `internal/app/integration_test.go` (register → login → refresh → reuse → logout, isolasi RLS) dan `internal/app/staff_integration_test.go` (buat/daftar/login/PIN, validasi dan konflik email lintas tenant, batas role admin vs owner, cakupan outlet, nonaktif mencabut sesi dan aktif kembali, reset password, batas paket, isolasi tenant). Butuh `TEST_DATABASE_URL`.

## Constraints

- Refresh token berformat `<tenant_id>.<secret>`; hanya SHA-256 yang disimpan. Awalan tenant memungkinkan lookup ber-RLS tanpa fungsi lintas tenant.
- Email unik global (login tanpa memilih tenant).
- Login tak dikenal tetap menjalankan verifikasi hash dummy agar waktu respons seragam.
- Rate limit in-memory (10 percobaan/menit/IP pada register dan login); pindah ke Redis bila API lebih dari satu instans.
- Guard membaca role dari klaim JWT, sehingga perubahan role atau penonaktifan baru berlaku untuk access token yang sudah terbit setelah maksimal 15 menit; refresh token langsung dicabut.
- Pembuatan dan pengaktifan kembali pengguna mengunci baris tenant (`FOR UPDATE`) agar batas paket tidak terlewati oleh permintaan paralel.
- Modul lain tidak boleh membaca tabel auth langsung (pengecualian yang sudah ada: modul inventory membaca `users` dan `user_store_roles` untuk nama pelaku dan keanggotaan outlet).
