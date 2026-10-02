# Inventory Module

## Purpose

Pelacakan stok per outlet: ledger pergerakan stok yang tidak bisa diubah, saldo per varian, pencatatan manual (stok masuk, rusak/keluar, koreksi), hitung fisik (opname), dan riwayat pergerakan.

## Responsibilities

- Mencatat setiap perubahan stok sebagai baris `stock_movements` append-only.
- Menjaga `stock_levels` sebagai saldo turunan yang diperbarui atomik dalam transaksi yang sama dengan ledger.
- Menerapkan hasil hitung fisik sebagai selisih terhadap saldo saat itu.
- Memastikan pemanggil adalah anggota outlet yang dituju.
- Isolasi tenant lewat RLS.

## Features

- `GET /v1/inventory/levels?store_id=&q=&limit=&cursor=` (`product:read`): saldo semua varian dari produk berpelacakan stok (`track_stock`), urut `lower(produk), lower(varian), id`, paginasi keyset; varian tanpa pergerakan bersaldo `0.000`
- `GET /v1/inventory/movements?store_id=&variant_id=&type=&limit=&cursor=` (`inventory:adjust`): riwayat terbaru dulu, keyset `(created_at, id)`
- `POST /v1/inventory/movements` (`inventory:adjust`): `store_id`, `variant_id`, `type` (`purchase_receive`, `waste`, `adjustment`), `qty_delta` (string desimal bertanda), `unit_cost` (hanya stok masuk), `reason`; respons 201 berisi `movement` dan `qty_on_hand`
- `POST /v1/inventory/counts` (`inventory:opname`): `store_id`, `reason` opsional (default "Stok opname"), `items[]` (`variant_id`, `counted_qty`); selisih nol tidak membuat baris ledger; semua-atau-tidak

Aturan: `purchase_receive` harus positif, `waste` negatif, `adjustment` bertanda bebas; `waste` dan `adjustment` wajib beralasan (maks 200 karakter); jumlah maksimal 9 digit dan 3 desimal, dikirim sebagai string. Stok boleh negatif (kebijakan `allow`) karena penjualan offline akan tiba belakangan. Hanya varian dari produk berpelacakan stok yang dapat dicatat (`STOCK_NOT_TRACKED`).

Belum ada: stok minimum dan alert, alur opname bertahap (draft/review/finalized), transfer stok antar outlet, pengurangan stok dari penjualan/refund/void (fase kasir), konsumsi bahan baku dari resep (V1.5), idempotensi lewat `mutation_id` (fase sync).

## Structure

```text
inventory/
├── README.md
├── module.go
├── domain/              # qty.go (parse/format desimal 3 digit), movement.go (tipe, validasi, hasil validasi)
├── application/         # Service, port Repository, error, filter/halaman
├── infrastructure/pg/   # repository.go
└── interface/http/      # handler.go
```

## Dependencies

- `platform/database`, `platform/httpx`, `platform/authn`, `platform/pagination`.
- Permission dipasang dari luar (`Mount(r, guard)`).
- Membaca (hanya baca) tabel `products` dan `product_variants` milik katalog untuk nama, SKU, dan `track_stock`; tabel `users` dan `user_store_roles` milik auth untuk nama pelaku dan keanggotaan outlet. Kopling baca-saja ini disengaja agar tidak ada modul perantara pada fase ini.

## API / Public Interface

- `inventory.New(Deps)` mengembalikan `*httpapi.Handler`; `Mount(r, guard)` memasang rute di `/v1`.
- Kode error: `VALIDATION_FAILED`, `BAD_REQUEST`, `STORE_NOT_FOUND` (404), `INVALID_VARIANT`, `STOCK_NOT_TRACKED`, `FORBIDDEN`, `UNAUTHORIZED`.
- Path field error: `qty_delta`, `reason`, `items[2].variant_id`, `items[0].counted_qty`.

## Data

`stock_movements` (append-only; aplikasi hanya punya `SELECT` dan `INSERT`), `stock_levels` (saldo turunan; `SELECT`, `INSERT`, `UPDATE`). Keduanya ber-RLS dan memakai FK komposit `(tenant_id, ...)` ke `stores` dan `product_variants`. `qty_delta` bertipe `numeric(12,3)` dan `<> 0`; `qty_on_hand` bertipe `numeric(14,3)` agar akumulasi punya ruang.

## Integrations

Tidak ada.

## Testing

- Unit: parse/format jumlah, validasi pergerakan dan hitung fisik, service (repository fake, cursor), paket `platform/pagination`.
- Integration: `internal/app/inventory_integration_test.go` (saldo dan riwayat, validasi dan produk tak berpelacakan, opname atomik, 20 penerimaan paralel menghasilkan saldo sama dengan jumlah ledger, ledger menolak UPDATE/DELETE, otorisasi kasir, keanggotaan outlet, isolasi tenant, paginasi dan pencarian saldo).

## Constraints

- Setiap perubahan saldo harus disertai baris ledger dalam transaksi yang sama; saldo tidak pernah ditimpa dari klien.
- Penambahan saldo memakai upsert atomik (`ON CONFLICT DO UPDATE ... qty_on_hand + delta`); opname mengunci baris saldo `FOR UPDATE` dan memprosesnya terurut per `variant_id` untuk menghindari deadlock.
- Jumlah diproses sebagai bilangan bulat ribuan (`int64`) di aplikasi dan dikonversi di SQL (`bigint / 1000`), tanpa float.
- Pemanggil harus anggota outlet (`user_store_roles`); outlet tenant lain atau tanpa keanggotaan diperlakukan sebagai tidak ada.
- Koreksi kesalahan dilakukan dengan baris baru, bukan mengubah atau menghapus baris lama.
