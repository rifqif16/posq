# Catalog Module

## Purpose

Katalog menu/produk tenant: kategori dan produk sederhana (satu varian default). Varian ganda dan modifier menyusul.

## Responsibilities

- CRUD kategori dengan hierarki maksimal 2 level (induk → anak).
- CRUD produk sederhana dengan SKU, barcode, harga beli/jual, dan pencarian.
- Optimistic concurrency produk (`version` + `If-Match`) dan pencatatan riwayat harga.
- Isolasi tenant lewat RLS pada setiap query.

## Features

Kategori:

- `GET /v1/categories` (`product:read`): daftar kategori aktif, datar
- `POST /v1/categories` (`product:write`): `name`, `parent_id` opsional, `sort_order`
- `PATCH /v1/categories/{id}` (`product:write`): ubah `name` dan `sort_order`
- `DELETE /v1/categories/{id}` (`product:write`): soft delete; ditolak bila masih punya sub-kategori atau produk aktif

Produk:

- `GET /v1/products?q=&category_id=&limit=&cursor=` (`product:read`): pencarian nama/SKU (sebagian) dan barcode (persis); paginasi cursor keyset; urut `lower(name), id`
- `POST /v1/products` (`product:write`): `variants` berisi tepat satu varian; SKU kosong dibuat otomatis (`SKU-XXXXXXXX`)
- `GET /v1/products/{id}` (`product:read`): menyertakan `ETag` dan `version`
- `PATCH /v1/products/{id}` (`product:write`): wajib `If-Match: "<version>"`; mengganti seluruh field yang dapat diubah; SKU kosong berarti SKU dipertahankan
- `DELETE /v1/products/{id}` (`product:write`): soft delete; barcode dibebaskan

`cost_price` bernilai `null` untuk pemanggil tanpa `product:cost_price_read` (mis. kasir).

Belum ada: varian ganda, modifier, impor/ekspor CSV, satuan, `min_stock`, API pembaca riwayat harga, ubah induk kategori.

## Structure

```text
catalog/
├── README.md
├── module.go            # composition root
├── domain/              # Category, Product, Variant, validasi
├── application/         # Service (kategori), ProductService + port Repository/ProductRepository
├── infrastructure/pg/   # repository.go (kategori), products.go (produk)
└── interface/http/      # handler.go (kategori, routing, error), products.go (produk)
```

## Dependencies

- `platform/database`, `platform/httpx`, `platform/authn`.
- Permission dipasang dari luar (`Handler.Mount(r, guard)`); modul ini tidak mengimpor modul auth.

## API / Public Interface

- `catalog.New(Deps)` mengembalikan `*httpapi.Handler`; `Mount(r, guard)` memasang rute di `/v1`.
- Kode error: `VALIDATION_FAILED`, `BAD_REQUEST`, `NOT_FOUND`, `CATEGORY_NAME_TAKEN`, `CATEGORY_IN_USE`, `INVALID_PARENT`, `SKU_TAKEN`, `BARCODE_TAKEN`, `INVALID_CATEGORY`, `VERSION_CONFLICT` (412), `PRECONDITION_REQUIRED` (428), `FORBIDDEN`, `UNAUTHORIZED`.

## Data

`categories`, `products`, `product_variants`, `variant_barcodes`, `product_price_history` (semua RLS). Soft delete lewat `deleted_at`. SKU unik per tenant (case-insensitive) dan barcode unik per tenant, keduanya hanya di antara data aktif. FK komposit `(tenant_id, ...)` mencegah referensi lintas tenant.

## Integrations

Tidak ada.

## Testing

- Unit: validasi domain, service (repository fake), cursor.
- Integration: `internal/app/catalog_integration_test.go` dan `products_integration_test.go` (CRUD, hierarki, unik SKU/barcode, pencarian dan escape wildcard, paginasi, ETag, riwayat harga, otorisasi kasir, isolasi tenant).

## Constraints

- Kategori dikunci `FOR SHARE` saat dipakai induk/produk dan `FOR UPDATE` saat dihapus, agar tidak ada data yatim akibat balapan.
- Penulisan produk selalu dalam satu transaksi (produk + varian + barcode + riwayat harga).
- Aplikasi tidak punya hak `DELETE` kecuali pada `variant_barcodes`.
- Harga disimpan `bigint` Rupiah, maksimal 1.000.000.000 per item.
- Fase 4 harus melonggarkan aturan "tepat satu varian" dan menambah `is_active`/`min_stock` pada varian lewat migrasi baru.
