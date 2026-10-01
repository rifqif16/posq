export const MAX_CSV_BYTES = 2 * 1024 * 1024;

export const CSV_COLUMNS = [
  "product_name",
  "category",
  "kitchen_station",
  "taxable",
  "track_stock",
  "is_active",
  "modifier_groups",
  "variant_name",
  "sku",
  "barcodes",
  "cost_price",
  "sell_price",
  "variant_is_active",
] as const;

export interface ImportRowError {
  row: number;
  column: string;
  message: string;
}

export interface ImportReport {
  valid: boolean;
  products: number;
  variants: number;
  created: number;
  error_count: number;
  errors: ImportRowError[];
}

export function templateCsv(): string {
  return (
    [
      CSV_COLUMNS.join(","),
      "Es Kopi,,bar,true,false,true,,Small,KOPI-S,8990001,5000,10000,true",
      "Es Kopi,,,,,,,Large,KOPI-L,8990002,7000,15000,true",
      "Roti Bakar,,,true,false,true,,,ROTI,,3000,8000,true",
    ].join("\n") + "\n"
  );
}

export function describeReport(r: ImportReport): string {
  if (r.created > 0) return `${r.created} produk berhasil diimpor`;
  if (r.valid)
    return `${r.products} produk (${r.variants} varian) siap diimpor`;
  return `${r.error_count} kesalahan ditemukan; perbaiki file lalu unggah ulang`;
}

export function hiddenErrorCount(r: ImportReport): number {
  return Math.max(0, r.error_count - r.errors.length);
}

export function locationLabel(e: ImportRowError): string {
  if (e.row === 0) return e.column ? `File · ${e.column}` : "File";
  return e.column ? `Baris ${e.row} · ${e.column}` : `Baris ${e.row}`;
}

export function fileProblem(file: {
  name: string;
  size: number;
}): string | null {
  if (!/\.csv$/i.test(file.name)) return "File harus berekstensi .csv";
  if (file.size === 0) return "File kosong";
  if (file.size > MAX_CSV_BYTES) return "Ukuran file maksimal 2 MB";
  return null;
}
