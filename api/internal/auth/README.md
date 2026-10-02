# Auth Module

## Purpose

Identitas dan sesi: registrasi tenant baru (self-service), login email + password, rotasi refresh token, profil pengguna aktif, manajemen staf (akun, role, outlet, password, PIN), serta registrasi perangkat dan login PIN pada perangkat terdaftar.

## Responsibilities

- Validasi registrasi dan kredensial.
- Hash password dan PIN (argon2id) dan penerbitan access token (JWT EdDSA, 15 menit).
- Refresh token berotasi dengan deteksi reuse (seluruh keluarga dicabut).
- Middleware `RequireAuth` dan `Require(permission)` untuk modul lain.
- Manajemen staf dan penegakan batas pengguna paket.
- Registrasi perangkat (rahasia perangkat), penegakan batas perangkat paket, dan login PIN dengan penguncian setelah kegagalan berulang.

## Features

- `POST /v1/auth/register`: membuat tenant (trial), outlet pertama, dan owner, lalu membuka sesi
- `POST /v1/auth/login`
- `POST /v1/auth/refresh`
- `POST /v1/auth/logout` (idempoten)
- `GET /v1/me`
- `GET /v1/stores` (`user:manage`): outlet tenant
- `GET /v1/staff`, `POST /v1/staff`, `PATCH /v1/staff/{id}`, `POST /v1/staff/{id}/password`, `PUT|DELETE /v1/staff/{id}/pin` (`user:manage`)
- `GET /v1/devices`, `POST /v1/devices`, `POST /v1/devices/{id}/revoke` (`settings:write`): `POST` mengembalikan `secret` satu kali
- `POST /v1/auth/pin-users`: daftar kasir dan dapur aktif ber-PIN di outlet perangkat (butuh `device_id` + `device_secret`)
- `POST /v1/auth/pin-login`: `device_id`, `device_secret`, `user_id`, `pin`; respons sama seperti login email (access token + cookie refresh)

Aturan staf: owner mengelola admin, kasir, dan dapur; admin hanya kasir; owner tidak dapat diubah dan tidak ada yang mengelola dirinya sendiri lewat endpoint ini. Admin hanya boleh memberi outlet yang dia anggotai. Batas `plans.limits.users` dihitung dari pengguna aktif; melebihi batas menghasilkan `PLAN_LIMIT_REACHED`.

Aturan perangkat dan PIN: rahasia perangkat disimpan sebagai SHA-256; kode perangkat berurut per outlet (`K01`, `K02`, …) dan tidak dipakai ulang; batas `plans.limits.devices` dihitung dari perangkat aktif (`DEVICE_LIMIT_REACHED`). Hanya role kasir dan dapur yang dapat login PIN, dan hanya di outlet tempat mereka ditugaskan. Perangkat tidak valid, dicabut, rahasia salah, pengguna tidak ada, tanpa PIN, atau role lain menghasilkan `INVALID_CREDENTIALS` yang sama. Lima PIN salah berturut-turut mengunci akun 15 menit (`PIN_LOCKED`, 429, `Retry-After: 900`); login berhasil mereset hitungan. PIN lemah (`000000`, `123456`, dan sejenisnya) ditolak saat diatur.

Belum ada: sesi terikat ke `device_id`, pemutusan sesi berjalan saat perangkat dicabut, audit log, undangan dan reset password lewat email, ubah email, profil diri, MFA, penegakan status tenant saat login, verifikasi PIN offline (fase sync).

## Structure

```text
auth/
├── README.md
├── module.go            # composition root auth: New(Deps) -> *Handler
├── staff_module.go      # NewStaff(Deps) -> *StaffHandler
├── device_module.go     # NewDevices(Deps) -> *DeviceHandler (perangkat + login PIN)
├── domain/              # email, password, registrasi, refresh token, permission, aturan staf (staff.go)
├── application/         # Service, StaffService (staff.go), DeviceService (device.go), PinLoginService (pin_login.go)
├── infrastructure/      # argon2id, JWT EdDSA
│   └── pg/              # repository.go (auth), staff.go, device.go; PostgreSQL (RLS)
└── interface/http/      # handler.go, guard.go, staff.go, device.go
```

## Dependencies

- `tenant` (domain + `infrastructure/pg`): pembuatan tenant dan outlet saat registrasi.
- `platform/database`, `platform/httpx`, `platform/ratelimit`, `platform/authn`.

## API / Public Interface

- `auth.New(Deps)` mengembalikan `*httpapi.Handler`; `Handler.Mount(chi.Router)` memasang rute auth di `/v1`; `Handler.Require(permission)` dipakai modul lain sebagai guard.
- `auth.NewStaff(Deps)` dan `auth.NewDevices(Deps)` mengembalikan handler dengan `Mount(r, guard)`.
- `DeviceHandler` membungkus `Handler` agar memakai ulang `writeSession` (cookie refresh) dan pembatas laju.
- Error mengikuti problem+json (RFC 9457) dengan field `code`: `VALIDATION_FAILED`, `EMAIL_TAKEN`, `INVALID_CREDENTIALS`, `ACCOUNT_DISABLED`, `INVALID_REFRESH_TOKEN`, `UNAUTHORIZED`, `RATE_LIMITED`, `FORBIDDEN`, `ROLE_NOT_ALLOWED`, `INVALID_STORE`, `PLAN_LIMIT_REACHED`, `DEVICE_LIMIT_REACHED`, `PIN_LOCKED`, `NOT_FOUND`.

## Data

`users` (termasuk `pin_hash`, `pin_failed_attempts`, `pin_locked_until`), `user_store_roles`, `refresh_tokens`, `devices` (semua RLS per `tenant_id`). Fungsi `auth_find_user_by_email` dan `auth_find_device` (SECURITY DEFINER) adalah satu-satunya akses lintas tenant, untuk lookup login dan lookup perangkat sebelum tenant diketahui. Aplikasi punya hak `DELETE` pada `user_store_roles`.

## Integrations

Tidak ada integrasi eksternal.

## Testing

- Unit: domain (aturan role dan PIN), application (repository fake, matriks otorisasi staf, rahasia perangkat), argon2, JWT.
- Integration: `internal/app/integration_test.go`, `staff_integration_test.go`, dan `device_integration_test.go` (pendaftaran, batas, pencabutan perangkat; alur login PIN dengan cookie; semua penolakan; penguncian lima kali gagal dan pembukaan setelah kedaluwarsa; reset hitungan setelah sukses; isolasi tenant). Butuh `TEST_DATABASE_URL`.

## Constraints

- Refresh token berformat `<tenant_id>.<secret>`; hanya SHA-256 yang disimpan. Email unik global.
- Login tak dikenal dan login PIN yang tidak memenuhi syarat tetap menjalankan verifikasi hash dummy agar waktu respons seragam.
- Rate limit in-memory (10 percobaan/menit/IP pada register, login, dan endpoint PIN; pembatas terpisah per handler); pindah ke Redis bila API lebih dari satu instans.
- Penghitungan kegagalan PIN memakai satu `UPDATE` atomik; kunci diperiksa sebelum verifikasi PIN.
- Guard membaca role dari klaim JWT, sehingga perubahan role atau penonaktifan berlaku untuk access token yang sudah terbit setelah maksimal 15 menit; refresh token dicabut langsung saat penonaktifan atau reset password.
- Pembuatan pengguna, pengaktifan kembali, dan pendaftaran perangkat mengunci baris tenant (`FOR UPDATE`) agar batas paket tidak terlewati oleh permintaan paralel.
- Modul lain tidak boleh membaca tabel auth langsung (pengecualian yang ada: modul inventory membaca `users` dan `user_store_roles`).
