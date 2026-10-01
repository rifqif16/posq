import { formatRupiah, parseRupiah } from "./money";
import type { FieldErrors } from "./validate";

export const MAX_PRICE = 1_000_000_000;
export const MAX_BARCODES = 10;
export const MAX_VARIANTS = 20;
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
  is_active: boolean;
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

export interface VariantForm {
  key: string; // kunci React lokal, bukan dikirim ke server
  id?: string; // ada bila varian sudah tersimpan
  name: string;
  sku: string;
  barcodes: string; // dipisah koma/spasi/baris baru
  cost_price: string;
  sell_price: string;
  is_active: boolean;
}

export interface ProductForm {
  name: string;
  category_id: string; // "" = tanpa kategori
  kitchen_station: string;
  taxable: boolean;
  track_stock: boolean;
  is_active: boolean;
  has_variants: boolean;
  variants: VariantForm[]; // mode tanpa varian hanya memakai elemen pertama
}

export interface VariantRequest {
  id?: string;
  name: string;
  sku: string;
  barcodes: string[];
  cost_price: number;
  sell_price: number;
  is_active: boolean;
}

export interface ProductRequest {
  name: string;
  category_id: string | null;
  taxable: boolean;
  track_stock: boolean;
  is_active: boolean;
  kitchen_station: string;
  variants: VariantRequest[];
}

let keySeq = 0;
const newKey = () => `v${++keySeq}`;

export function emptyVariant(): VariantForm {
  return {
    key: newKey(),
    name: "",
    sku: "",
    barcodes: "",
    cost_price: "",
    sell_price: "",
    is_active: true,
  };
}

export function emptyForm(): ProductForm {
  return {
    name: "",
    category_id: "",
    kitchen_station: "",
    taxable: true,
    track_stock: false,
    is_active: true,
    has_variants: false,
    variants: [emptyVariant()],
  };
}

export function formFromProduct(p: Product): ProductForm {
  const variants: VariantForm[] = p.variants.map((v) => ({
    key: v.id,
    id: v.id,
    name: v.name,
    sku: v.sku,
    barcodes: v.barcodes.join("\n"),
    cost_price: v.cost_price == null ? "" : String(v.cost_price),
    sell_price: String(v.sell_price),
    is_active: v.is_active,
  }));
  return {
    name: p.name,
    category_id: p.category_id ?? "",
    kitchen_station: p.kitchen_station,
    taxable: p.taxable,
    track_stock: p.track_stock,
    is_active: p.is_active,
    has_variants: variants.length > 1,
    variants: variants.length > 0 ? variants : [emptyVariant()],
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

function effectiveVariants(f: ProductForm): VariantForm[] {
  return f.has_variants ? f.variants : f.variants.slice(0, 1);
}

function priceError(raw: string, required: boolean): string | undefined {
  if (raw.trim() === "") return required ? "Harga wajib diisi" : undefined;
  const n = parseRupiah(raw);
  if (n === null || n > MAX_PRICE)
    return `Harga harus angka antara 0 dan ${MAX_PRICE.toLocaleString("id-ID")}`;
}

export function validateForm(f: ProductForm): FieldErrors {
  const errors: FieldErrors = {};
  const add = (key: string, msg: string | undefined) => {
    if (msg && !errors[key]) errors[key] = msg;
  };

  const name = f.name.trim();
  add(
    "name",
    name === "" || [...name].length > 100
      ? "Nama produk wajib diisi (maks 100 karakter)"
      : undefined,
  );
  const station = f.kitchen_station.trim().toLowerCase();
  add(
    "kitchen_station",
    station !== "" && !STATION.test(station)
      ? "Station hanya huruf kecil, angka, - dan _ (maks 30)"
      : undefined,
  );

  const variants = effectiveVariants(f);
  if (f.has_variants) {
    if (variants.length < 2) add("variants", "Tambahkan minimal 2 varian");
    if (variants.length > MAX_VARIANTS)
      add("variants", `Maksimal ${MAX_VARIANTS} varian`);
    if (variants.every((v) => !v.is_active))
      add("variants", "Minimal satu varian harus aktif");
  }

  const names = new Set<string>();
  const skus = new Set<string>();
  const codes = new Set<string>();
  variants.forEach((v, i) => {
    const p = `variants[${i}]`;
    if (f.has_variants) {
      const n = v.name.trim();
      if (n === "" || [...n].length > 50)
        add(`${p}.name`, "Nama varian wajib diisi (maks 50 karakter)");
      else if (names.has(n.toLowerCase()))
        add(`${p}.name`, "Nama varian harus unik");
      names.add(n.toLowerCase());
    }
    const sku = v.sku.trim();
    if (sku !== "" && !CODE.test(sku))
      add(`${p}.sku`, "SKU hanya huruf, angka, titik, - dan _ (maks 64)");
    else if (sku !== "" && skus.has(sku.toLowerCase()))
      add(`${p}.sku`, "SKU dipakai varian lain pada produk ini");
    skus.add(sku.toLowerCase());

    const barcodes = parseBarcodes(v.barcodes);
    if (barcodes.length > MAX_BARCODES)
      add(`${p}.barcodes`, `Maksimal ${MAX_BARCODES} barcode`);
    else if (barcodes.some((b) => !CODE.test(b)))
      add(
        `${p}.barcodes`,
        "Barcode hanya huruf, angka, titik, - dan _ (maks 64)",
      );
    else if (barcodes.some((b) => codes.has(b)))
      add(`${p}.barcodes`, "Barcode dipakai varian lain pada produk ini");
    barcodes.forEach((b) => codes.add(b));

    add(`${p}.sell_price`, priceError(v.sell_price, true));
    add(`${p}.cost_price`, priceError(v.cost_price, false));
  });
  return errors;
}

export function toRequest(f: ProductForm): ProductRequest {
  const multi = f.has_variants;
  return {
    name: f.name.trim(),
    category_id: f.category_id === "" ? null : f.category_id,
    taxable: f.taxable,
    track_stock: f.track_stock,
    is_active: f.is_active,
    kitchen_station: f.kitchen_station.trim(),
    variants: effectiveVariants(f).map((v) => ({
      ...(v.id ? { id: v.id } : {}),
      name: multi ? v.name.trim() : "",
      sku: v.sku.trim(),
      barcodes: parseBarcodes(v.barcodes),
      cost_price: parseRupiah(v.cost_price) ?? 0,
      sell_price: parseRupiah(v.sell_price) ?? 0,
      is_active: multi ? v.is_active : true,
    })),
  };
}

export function makeDefault<T>(items: T[], index: number): T[] {
  if (index <= 0 || index >= items.length) return items;
  return [items[index], ...items.slice(0, index), ...items.slice(index + 1)];
}

export function sellPriceLabel(p: Product): string {
  if (p.variants.length === 0) return "-";
  const pool = p.variants.some((v) => v.is_active)
    ? p.variants.filter((v) => v.is_active)
    : p.variants;
  const prices = pool.map((v) => v.sell_price);
  const min = Math.min(...prices);
  const max = Math.max(...prices);
  return min === max
    ? formatRupiah(min)
    : `${formatRupiah(min)} – ${formatRupiah(max)}`;
}
