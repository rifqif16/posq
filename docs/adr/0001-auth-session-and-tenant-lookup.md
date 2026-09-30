# ADR-0001: Sesi auth dan lookup login lintas tenant

## Context

Login memakai email + password tanpa memilih tenant, sementara semua tabel bisnis dilindungi RLS berbasis `app.tenant_id` (ADR-08 di POSQ.md).

## Problem

Saat login, tenant belum diketahui sehingga query `users` tidak bisa lewat RLS. Refresh token juga perlu dicari sebelum tenant diketahui.

## Options

1. Role aplikasi dengan `BYPASSRLS` untuk auth: melemahkan isolasi untuk seluruh modul auth.
2. Login mewajibkan kode tenant: menambah friksi bagi kasir dan owner.
3. Fungsi `SECURITY DEFINER` sempit untuk lookup email; refresh token membawa `tenant_id` sebagai awalan.

## Decision

Opsi 3. `auth_find_user_by_email(text)` adalah satu-satunya jalur lintas tenant (baca satu baris per email). Refresh token berformat `<tenant_id>.<secret>`; hanya hash SHA-256 disimpan, dan lookup berjalan di transaksi ber-scope tenant. Email unik global. Access token JWT EdDSA 15 menit; refresh token dirotasi dengan deteksi reuse yang mencabut seluruh keluarga.

## Consequences

- Isolasi RLS tetap utuh untuk semua query lain; role aplikasi tanpa `BYPASSRLS` dan bukan owner tabel.
- Satu email hanya bisa dimiliki satu tenant.
- Fungsi definer harus ditinjau setiap kali diubah (satu-satunya lubang terkontrol).
- `tenant_id` pada refresh token bukan rahasia; keamanan bergantung pada secret 256-bit.

## Revisit Criteria

- Kebutuhan satu email di banyak tenant (mis. konsultan multi-bisnis).
- Migrasi ke penyedia identitas eksternal.
