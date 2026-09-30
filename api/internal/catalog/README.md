# Catalog Module

## Purpose

Katalog menu/produk tenant. Fase ini baru mencakup kategori; produk, varian, dan modifier menyusul.

## Responsibilities

- CRUD kategori dengan hierarki maksimal 2 level (induk → anak).
- Isolasi tenant lewat RLS pada setiap query.

## Features

- `GET /v1/categories` (`product:read`): daftar kategori aktif, datar
- `POST /v1/categories` (`product:write`): `name`, `parent_id` opsional, `sort_order`
- `PATCH /v1/categories/{id}` (`product:write`): ubah `name` dan `sort_order`
- `DELETE /v1/categories/{id}` (`product:write`): soft delete

Belum ada: produk, varian, modifier, barcode, impor CSV, satuan, ubah induk kategori.

## Structure

```text
catalog/
├── README.md
├── module.go            # composition root
├── domain/              # Category, validasi
├── application/         # Service + port Repository, error domain
├── infrastructure/pg/   # Repository PostgreSQL
└── interface/http/      # handler chi, Guard yang disuntikkan
```

## Dependencies

- `platform/database`, `platform/httpx`, `platform/authn`.
- Permission dipasang dari luar (`Handler.Mount(r, guard)`); modul ini tidak mengimpor modul auth.

## API / Public Interface

- `catalog.New(Deps)` mengembalikan `*httpapi.Handler`; `Mount(r, guard)` memasang rute di `/v1`.
- Kode error: `VALIDATION_FAILED`, `CATEGORY_NAME_TAKEN`, `CATEGORY_IN_USE`, `INVALID_PARENT`, `NOT_FOUND`, `FORBIDDEN`, `UNAUTHORIZED`.

## Data

`categories` (RLS; soft delete; FK komposit `(tenant_id, parent_id)`; nama unik per induk, case-insensitive, di antara kategori aktif).

## Integrations

Tidak ada.

## Testing

- Unit: validasi domain dan service (repository fake).
- Integration: `internal/app/catalog_integration_test.go` (CRUD, hierarki, duplikat, soft delete, otorisasi kasir, isolasi tenant).

## Constraints

- Induk dikunci `FOR SHARE` saat membuat anak dan `FOR UPDATE` saat menghapus, agar tidak ada anak yatim.
- Saat tabel produk ditambahkan, `DeleteCategory` wajib menolak kategori yang masih dipakai produk aktif.
- Aplikasi tidak punya hak `DELETE`; penghapusan selalu lewat `deleted_at`.
