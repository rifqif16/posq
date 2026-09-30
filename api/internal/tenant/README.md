# Tenant Module

## Purpose

Konsep tenant SaaS: status, masa trial, dan outlet.

## Responsibilities

- Menyediakan aturan status tenant dan perhitungan akhir trial.
- Membuat tenant beserta outlet pertama (dipanggil dari registrasi auth di dalam transaksi ber-scope tenant).

## Features

- Status tenant: `trial`, `active`, `past_due`, `suspended`, `cancelled` (baru `trial` yang dipakai).
- Penyisipan tenant + outlet pertama (`MAIN`) dengan paket `trial`.

Belum ada: plan enforcement (`usage_counters`), CRUD outlet, ekspor data tenant.

## Structure

```text
tenant/
├── README.md
├── domain/              # Status, konstanta, TrialEndsAt
└── infrastructure/pg/   # InsertWithFirstStore(tx, Seed)
```

## Dependencies

Tidak bergantung pada modul lain.

## API / Public Interface

`pg.InsertWithFirstStore(ctx, tx, Seed)`; harus dipanggil dalam `database.WithTenantTx`.

## Data

`plans` (referensi, tanpa RLS), `tenants`, `stores` (RLS).

## Constraints

- Durasi trial berasal dari `TRIAL_DAYS` (placeholder 14 hari) sampai Q17 diputuskan.
- Batas paket pada seed `trial` adalah placeholder.
