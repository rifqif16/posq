# Catalog Module

## Purpose

Katalog menu/produk tenant: kategori, produk (sederhana dan bervarian), grup modifier dengan opsi bertarif, dan penautan grup ke produk.

## Responsibilities

- CRUD kategori dengan hierarki maksimal 2 level (induk → anak).
- CRUD produk dengan 1–20 varian; SKU, barcode, harga beli/jual, status aktif per varian; pencarian.
- CRUD grup modifier dengan 1–50 opsi (nama, tambahan harga, default, aktif, urutan) dan aturan `min_select`/`max_select`.
- Menautkan 0–10 grup modifier (berurut) ke setiap produk.
- Optimistic concurrency (`version` + `If-Match`) pada produk dan grup modifier; riwayat harga per varian.
- Isolasi tenant lewat RLS pada setiap query.

## Features

Kategori:

- `GET /v1/categories` (`product:read`): daftar kategori aktif, datar
- `POST /v1/categories` (`product:write`): `name`, `parent_id` opsional, `sort_order`
- `PATCH /v1/categories/{id}` (`product:write`): ubah `name` dan `sort_order`
- `DELETE /v1/categories/{id}` (`product:write`): soft delete; ditolak bila masih punya sub-kategori atau produk aktif

Produk:

- `GET /v1/products?q=&category_id=&limit=&cursor=` (`product:read`): pencarian nama/SKU (sebagian) dan barcode (persis) pada varian mana pun; paginasi cursor keyset; urut `lower(name), id`
- `POST /v1/products` (`product:write`): `variants` berisi 1–20 varian; varian pertama menjadi default; nama varian wajib bila lebih dari satu; SKU kosong dibuat otomatis (`SKU-XXXXXXXX`); `modifier_group_ids` opsional
- `GET /v1/products/{id}` (`product:read`): menyertakan `ETag`, `version`, dan `modifier_group_ids`
- `PATCH /v1/products/{id}` (`product:write`): wajib `If-Match: "<version>"`; mengganti seluruh field yang dapat diubah; varian: `id` = varian lama (SKU kosong berarti dipertahankan), tanpa `id` = varian baru, tidak dikirim = dihapus; `modifier_group_ids` mengganti seluruh penautan (dihilangkan = tanpa grup)
- `DELETE /v1/products/{id}` (`product:write`): soft delete produk dan varian; barcode dan penautan grup dibebaskan

Tipe produk diturunkan dari jumlah varian: 1 = `simple`, ≥2 = `variant`. `cost_price` bernilai `null` untuk pemanggil tanpa `product:cost_price_read` (mis. kasir). `modifier_group_ids` selalu array dan dapat dibaca kasir.

Grup modifier:

- `GET /v1/modifier-groups` (`product:read`): semua grup aktif beserta opsi, urut nama grup
- `GET /v1/modifier-groups/{id}` (`product:read`): menyertakan `ETag` dan `version`
- `POST /v1/modifier-groups` (`modifier:manage`): `name`, `min_select`, `max_select`, `modifiers[]`
- `PATCH /v1/modifier-groups/{id}` (`modifier:manage`): wajib `If-Match`; `id` = opsi lama, tanpa `id` = opsi baru, tidak dikirim = dihapus; urutan array = urutan tampil
- `DELETE /v1/modifier-groups/{id}` (`modifier:manage`): soft delete grup dan opsinya; ditolak (`MODIFIER_GROUP_IN_USE`) bila masih ditautkan ke produk aktif

`is_required` diturunkan: `min_select >= 1`. Aturan: `0 <= min <= max <= 50`, `min` tidak boleh melebihi jumlah opsi aktif, opsi default harus aktif dan jumlahnya tidak melebihi `max_select`, `price_delta` 0 sampai 1.000.000.000.

Belum ada: impor/ekspor CSV, satuan, `min_stock`, API pembaca riwayat harga, ubah induk kategori.

## Structure

```text
catalog/
├── README.md
├── module.go            # composition root
├── domain/              # Category, Product/Variant, ModifierGroup/Modifier, validasi
├── application/         # Service (kategori), ProductService, ModifierService, links.go (error penautan)
├── infrastructure/pg/   # repository.go (kategori), products.go, modifiers.go, modifier_links.go
└── interface/http/      # handler.go (kategori, routing, error), products.go, modifiers.go
```

## Dependencies

- `platform/database`, `platform/httpx`, `platform/authn`.
- Permission dipasang dari luar (`Handler.Mount(r, guard)`); modul ini tidak mengimpor modul auth.

## API / Public Interface

- `catalog.New(Deps)` mengembalikan `*httpapi.Handler`; `Mount(r, guard)` memasang rute di `/v1`.
- Kode error: `VALIDATION_FAILED`, `BAD_REQUEST`, `NOT_FOUND`, `CATEGORY_NAME_TAKEN`, `CATEGORY_IN_USE`, `INVALID_PARENT`, `SKU_TAKEN`, `BARCODE_TAKEN`, `INVALID_CATEGORY`, `INVALID_VARIANT`, `INVALID_MODIFIER_GROUP`, `MODIFIER_GROUP_NAME_TAKEN`, `MODIFIER_GROUP_IN_USE`, `INVALID_MODIFIER`, `VERSION_CONFLICT` (412), `PRECONDITION_REQUIRED` (428), `FORBIDDEN`, `UNAUTHORIZED`.
- Field error memakai path, mis. `variants[1].sku`, `modifiers[0].price_delta`, `modifier_group_ids[2]`.

## Data

`categories`, `products`, `product_variants`, `variant_barcodes`, `product_price_history`, `modifier_groups`, `modifiers`, `product_modifier_groups` (semua RLS). Soft delete lewat `deleted_at`. SKU unik per tenant, nama varian unik per produk, barcode unik per tenant, nama grup unik per tenant, nama opsi unik per grup (semuanya case-insensitive dan hanya di antara data aktif). FK komposit `(tenant_id, ...)` mencegah referensi lintas tenant.

## Integrations

Tidak ada.

## Testing

- Unit: validasi domain (varian, modifier, penautan), service (repository fake), cursor.
- Integration: `internal/app/{catalog,products,variants,modifiers,product_modifiers}_integration_test.go` (CRUD, hierarki, unik SKU/barcode, pencarian, paginasi, ETag, riwayat harga, sinkronisasi varian/opsi, pertukaran nama, pergantian default, konversi simple↔variant, penautan grup dan urutannya, penolakan hapus grup terpakai, balapan penaut vs penghapus, otorisasi kasir, isolasi tenant).

## Constraints

- Kategori dan grup modifier dikunci `FOR SHARE` saat dipakai (induk/produk/penautan) dan `FOR UPDATE` saat dihapus, agar tidak ada data yatim akibat balapan; dibuktikan oleh `TestModifierLinkVsDeleteRace`.
- Penulisan produk dan grup selalu dalam satu transaksi; baris induk dikunci `FOR UPDATE`, sehingga edit varian/opsi berjalan serial.
- Saat update, varian/opsi lama "diparkir" (nama/SKU sementara) lebih dulu agar pertukaran nama tidak menabrak indeks unik.
- Id varian/opsi baru dan SKU otomatis dibuat server (service); `id` dari klien hanya merujuk entitas yang sudah ada pada induk yang sama.
- Penautan produk–grup dibangun ulang pada setiap create/update dan dihapus saat produk dihapus; karena itu setiap baris tautan berarti produk aktif.
- Aplikasi tidak punya hak `DELETE` kecuali pada `variant_barcodes` dan `product_modifier_groups`.
- Harga disimpan `bigint` Rupiah, maksimal 1.000.000.000 per item.
