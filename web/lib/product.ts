import { parseRupiah } from "./money";
import type { FieldErrors } from "./validate";

export const MAX_PRICE = 1_000_000_000;
export const MAX_BARCODES = 10;
const CODE = /^[A-Za-z0-9._-]{1,64}$/;
const STATION = /^[a-z0-9_-]{1,30}$/;

export interface Variant {
  id: string;
  name: string;
  sku: string;
  barcodes: string[];
  cost_price: number | null; // null bila tidak punya izin melihat harga beli
  sell_price: number;
  is_default: boolean;
}

export interface Product {
  id: string;
  name: string;
  type: string;
  category_id: string | null;
  taxable: boolean;
  track_stock: boolean;
  kitchen_station: string;
  is_active: boolean;
  version: number;
  variants: Variant[];
}

export interface ProductForm {
  name: string;
  category_id: string; // "" = tanpa kategori
  sku: string;
  barcodes: string; // dipisah koma/spasi/baris baru
  cost_price: string;
  sell_price: string;
  kitchen_station: string;
  taxable: boolean;
  track_stock: boolean;
  is_active: boolean;
}

export interface ProductRequest {
  name: string;
  category_id: string | null;
  taxable: boolean;
  track_stock: boolean;
  is_active: boolean;
  kitchen_station: string;
  variants: {
    name: string;
    sku: string;
    barcodes: string[];
    cost_price: number;
    sell_price: number;
  }[];
}

export function emptyForm(): ProductForm {
  return {
    name: "",
    category_id: "",
    sku: "",
    barcodes: "",
    cost_price: "",
    sell_price: "",
    kitchen_station: "",
    taxable: true,
    track_stock: false,
    is_active: true,
  };
}

export function formFromProduct(p: Product): ProductForm {
  const v = p.variants[0];
  return {
    name: p.name,
    category_id: p.category_id ?? "",
    sku: v?.sku ?? "",
    barcodes: (v?.barcodes ?? []).join("\n"),
    cost_price: v?.cost_price == null ? "" : String(v.cost_price),
    sell_price: v ? String(v.sell_price) : "",
    kitchen_station: p.kitchen_station,
    taxable: p.taxable,
    track_stock: p.track_stock,
    is_active: p.is_active,
  };
}

export function parseBarcodes(raw: string): string[] {
  return [
    ...new Set(
      raw
        .split(/[\s,]+/)
        .map((b) => b.trim())
        .filter(Boolean),
    ),
  ];
}

function validatePrice(raw: string, required: boolean): string | undefined {
  if (raw.trim() === "") return required ? "Harga wajib diisi" : undefined;
  const n = parseRupiah(raw);
  if (n === null || n > MAX_PRICE)
    return `Harga harus angka antara 0 dan ${MAX_PRICE.toLocaleString("id-ID")}`;
}

export function validateForm(f: ProductForm): FieldErrors {
  const name = f.name.trim();
  const sku = f.sku.trim();
  const barcodes = parseBarcodes(f.barcodes);
  const errors: FieldErrors = {
    name:
      name === "" || [...name].length > 100
        ? "Nama produk wajib diisi (maks 100 karakter)"
        : undefined,
    sku:
      sku !== "" && !CODE.test(sku)
        ? "SKU hanya huruf, angka, titik, - dan _ (maks 64)"
        : undefined,
    barcodes:
      barcodes.length > MAX_BARCODES
        ? `Maksimal ${MAX_BARCODES} barcode`
        : barcodes.some((b) => !CODE.test(b))
          ? "Barcode hanya huruf, angka, titik, - dan _ (maks 64)"
          : undefined,
    sell_price: validatePrice(f.sell_price, true),
    cost_price: validatePrice(f.cost_price, false),
    kitchen_station:
      f.kitchen_station.trim() !== "" &&
      !STATION.test(f.kitchen_station.trim().toLowerCase())
        ? "Station hanya huruf kecil, angka, - dan _ (maks 30)"
        : undefined,
  };
  return Object.fromEntries(
    Object.entries(errors).filter(([, v]) => v !== undefined),
  );
}

export function toRequest(f: ProductForm): ProductRequest {
  return {
    name: f.name.trim(),
    category_id: f.category_id === "" ? null : f.category_id,
    taxable: f.taxable,
    track_stock: f.track_stock,
    is_active: f.is_active,
    kitchen_station: f.kitchen_station.trim(),
    variants: [
      {
        name: "",
        sku: f.sku.trim(),
        barcodes: parseBarcodes(f.barcodes),
        cost_price: parseRupiah(f.cost_price) ?? 0,
        sell_price: parseRupiah(f.sell_price) ?? 0,
      },
    ],
  };
}

export function mapServerFields(fields: FieldErrors): FieldErrors {
  return Object.fromEntries(
    Object.entries(fields).map(([k, v]) => [
      k.replace(/^variants\[0\]\./, ""),
      v,
    ]),
  );
}
