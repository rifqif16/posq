# Catalog Module

## Purpose

Katalog menu/produk tenant: kategori, produk (sederhana dan bervarian), grup modifier dengan opsi bertarif, penautan grup ke produk, riwayat harga, serta ekspor dan impor CSV produk.

## Responsibilities

- CRUD kategori dengan hierarki maksimal 2 level (induk → anak).
- CRUD produk dengan 1–20 varian; SKU, barcode, harga beli/jual, status aktif per varian; pencarian.
- CRUD grup modifier dengan 1–50 opsi (nama, tambahan harga, default, aktif, urutan) dan aturan `min_select`/`max_select`.
- Menautkan 0–10 grup modifier (berurut) ke setiap produk.
- Optimistic concurrency (`version` + `If-Match`) pada produk dan grup modifier.
- Mencatat dan menyajikan riwayat perubahan harga beli/jual per varian.
- Ekspor seluruh produk ke CSV dan impor produk baru dari CSV (validasi dulu, commit semua-atau-tidak).
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
- `GET /v1/products/{id}/price-history?limit=&cursor=` (`product:cost_price_read`): perubahan harga beli/jual semua varian produk (termasuk varian yang sudah dihapus), terbaru dulu, paginasi cursor keyset `(at, id)`

Tipe produk diturunkan dari jumlah varian: 1 = `simple`, ≥2 = `variant`. `cost_price` bernilai `null` untuk pemanggil tanpa `product:cost_price_read` (mis. kasir). `modifier_group_ids` selalu array dan dapat dibaca kasir. Harga awal saat produk dibuat tidak dicatat sebagai perubahan.

CSV produk:

- `GET /v1/products/export?delimiter=comma|semicolon` (`product:cost_price_read`): satu baris per varian, diawali BOM UTF-8, `Content-Disposition: attachment`
- `POST /v1/products/import` (`product:write`): body CSV mentah (maks 2 MB, 2000 baris data; `Content-Type` diabaikan); tanpa `?commit=true` hanya validasi (200 + laporan); dengan `commit=true` dan valid hasilnya 201; bila ada kesalahan 422 `CSV_INVALID` dan tidak ada yang dibuat

Format: 13 kolom `product_name, category, kitchen_station, taxable, track_stock, is_active, modifier_groups, variant_name, sku, barcodes, cost_price, sell_price, variant_is_active`. Hanya `product_name` dan `sell_price` yang wajib ada di header; kolom lain opsional dan kolom tak dikenal diabaikan. Baris dengan `product_name` sama (tanpa membedakan huruf besar/kecil) digabung menjadi satu produk; atribut produk dibaca dari baris pertamanya. `category` ditulis `Induk > Anak`; `modifier_groups` dan `barcodes` dipisah `|`. Boolean menerima `true/false`, `ya/tidak`, `1/0`. Harga bilangan bulat tanpa pemisah. Delimiter koma atau titik koma terdeteksi otomatis dari baris header.

Impor bersifat create-only: nama produk, SKU, dan barcode yang sudah ada ditolak per baris; kategori dan grup modifier harus sudah ada. Laporan berisi `valid`, `products`, `variants`, `created`, `error_count`, dan `errors[]` (`row`, `column`, `message`; maks 200 ditampilkan).

Grup modifier:

- `GET /v1/modifier-groups` (`product:read`): semua grup aktif beserta opsi, urut nama grup
- `GET /v1/modifier-groups/{id}` (`product:read`): menyertakan `ETag` dan `version`
- `POST /v1/modifier-groups` (`modifier:manage`): `name`, `min_select`, `max_select`, `modifiers[]`
- `PATCH /v1/modifier-groups/{id}` (`modifier:manage`): wajib `If-Match`; `id` = opsi lama, tanpa `id` = opsi baru, tidak dikirim = dihapus; urutan array = urutan tampil
- `DELETE /v1/modifier-groups/{id}` (`modifier:manage`): soft delete grup dan opsinya; ditolak (`MODIFIER_GROUP_IN_USE`) bila masih ditautkan ke produk aktif

`is_required` diturunkan: `min_select >= 1`. Aturan: `0 <= min <= max <= 50`, `min` tidak boleh melebihi jumlah opsi aktif, opsi default harus aktif dan jumlahnya tidak melebihi `max_select`, `price_delta` 0 sampai 1.000.000.000.

Belum ada: update massal lewat CSV, pembuatan kategori/grup otomatis saat impor, satuan, `min_stock`, ubah induk kategori.

## Structure

```text
catalog/
├── README.md
├── module.go            # composition root; mengembalikan Handlers{Catalog, PriceHistory, ProductCSV}
├── domain/              # Category, Product/Variant, ModifierGroup/Modifier, PriceChange, validasi
├── application/         # Service (kategori), ProductService, ModifierService, PriceHistoryService,
│                        # ProductCSVService (product_csv_format.go, product_csv_import.go), links.go
├── infrastructure/pg/   # repository.go, products.go, modifiers.go, modifier_links.go, price_history.go, product_csv.go
└── interface/http/      # handler.go (kategori, routing, error), products.go, modifiers.go, price_history.go, product_csv.go
```

## Dependencies

- `platform/database`, `platform/httpx`, `platform/authn`.
- Permission dipasang dari luar (`Mount(r, guard)`); modul ini tidak mengimpor modul auth.

## API / Public Interface

- `catalog.New(Deps)` mengembalikan `Handlers`; setiap handler punya `Mount(r, guard)` untuk memasang rute di `/v1`.
- Kode error: `VALIDATION_FAILED`, `BAD_REQUEST`, `NOT_FOUND`, `CATEGORY_NAME_TAKEN`, `CATEGORY_IN_USE`, `INVALID_PARENT`, `SKU_TAKEN`, `BARCODE_TAKEN`, `INVALID_CATEGORY`, `INVALID_VARIANT`, `INVALID_MODIFIER_GROUP`, `MODIFIER_GROUP_NAME_TAKEN`, `MODIFIER_GROUP_IN_USE`, `INVALID_MODIFIER`, `CSV_INVALID` (422), `PAYLOAD_TOO_LARGE` (413), `VERSION_CONFLICT` (412), `PRECONDITION_REQUIRED` (428), `FORBIDDEN`, `UNAUTHORIZED`.
- Field error memakai path, mis. `variants[1].sku`, `modifiers[0].price_delta`, `modifier_group_ids[2]`.

## Data

`categories`, `products`, `product_variants`, `variant_barcodes`, `product_price_history`, `modifier_groups`, `modifiers`, `product_modifier_groups` (semua RLS). Soft delete lewat `deleted_at`. SKU unik per tenant, nama varian unik per produk, barcode unik per tenant, nama grup unik per tenant, nama opsi unik per grup (semuanya case-insensitive dan hanya di antara data aktif). FK komposit `(tenant_id, ...)` mencegah referensi lintas tenant. `product_price_history` append-only.

## Integrations

Tidak ada.

## Testing

- Unit: validasi domain (varian, modifier, penautan), service (repository fake), cursor kunci dan cursor waktu, parser/penulis CSV (header, BOM, delimiter, CRLF, kesalahan per baris dan kolom, duplikat antar produk, batas baris, sanitasi formula, round trip).
- Integration: `internal/app/{catalog,products,variants,modifiers,product_modifiers,price_history,product_csv}_integration_test.go` (CRUD, hierarki, unik SKU/barcode, pencarian, paginasi, ETag, sinkronisasi varian/opsi, penautan grup, balapan penaut vs penghapus, riwayat harga, validasi lalu commit CSV, commit semua-atau-tidak, impor ulang ditolak, ekspor, round trip lintas tenant, otorisasi, isolasi tenant).

## Constraints

- Kategori dan grup modifier dikunci `FOR SHARE` saat dipakai dan `FOR UPDATE` saat dihapus, agar tidak ada data yatim akibat balapan.
- Penulisan produk dan grup selalu dalam satu transaksi; baris induk dikunci `FOR UPDATE`, sehingga edit varian/opsi berjalan serial.
- Saat update, varian/opsi lama "diparkir" (nama/SKU sementara) lebih dulu agar pertukaran nama tidak menabrak indeks unik.
- Id varian/opsi baru dan SKU otomatis dibuat server (service); `id` dari klien hanya merujuk entitas yang sudah ada pada induk yang sama.
- Riwayat harga ditulis dalam transaksi yang sama dengan perubahan harga; seluruh endpoint-nya memakai `product:cost_price_read` karena memuat harga beli.
- Impor CSV: validasi dan commit memakai kode yang sama; commit menulis semua produk dalam satu transaksi (satu kegagalan membatalkan semuanya). Pengecekan SKU/barcode/nama yang sudah ada dilakukan sebelum commit untuk menghasilkan laporan per baris, dan constraint database tetap menjadi pengaman terakhir (balapan menghasilkan 409).
- CSV: sel teks berawalan `=`, `+`, `-`, `@`, tab, atau CR diberi apostrof saat ekspor (anti formula injection) dan apostrof itu dibuang lagi saat impor.
- Aplikasi tidak punya hak `DELETE` kecuali pada `variant_barcodes` dan `product_modifier_groups`.
- Harga disimpan `bigint` Rupiah, maksimal 1.000.000.000 per item.
