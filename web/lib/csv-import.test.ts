import { describe, expect, it } from "vitest";
import {
  CSV_COLUMNS,
  type ImportReport,
  describeReport,
  fileProblem,
  hiddenErrorCount,
  locationLabel,
  templateCsv,
} from "./csv-import";

const report = (patch: Partial<ImportReport> = {}): ImportReport => ({
  valid: true,
  products: 2,
  variants: 3,
  created: 0,
  error_count: 0,
  errors: [],
  ...patch,
});

describe("templateCsv", () => {
  it("memiliki 13 kolom sesuai format server dan baris contoh yang konsisten", () => {
    const lines = templateCsv().trim().split("\n");
    expect(lines[0].split(",")).toEqual([...CSV_COLUMNS]);
    expect(CSV_COLUMNS).toHaveLength(13);
    expect(lines).toHaveLength(4);
    for (const line of lines.slice(1)) expect(line.split(",")).toHaveLength(13);
  });
});

describe("describeReport", () => {
  it("siap diimpor, berhasil, dan gagal", () => {
    expect(describeReport(report())).toBe("2 produk (3 varian) siap diimpor");
    expect(describeReport(report({ created: 2 }))).toBe(
      "2 produk berhasil diimpor",
    );
    expect(describeReport(report({ valid: false, error_count: 4 }))).toBe(
      "4 kesalahan ditemukan; perbaiki file lalu unggah ulang",
    );
  });
});

describe("hiddenErrorCount & locationLabel", () => {
  it("menghitung kesalahan yang tidak ditampilkan", () => {
    const errors = [{ row: 2, column: "sku", message: "x" }];
    expect(
      hiddenErrorCount(report({ valid: false, error_count: 250, errors })),
    ).toBe(249);
    expect(
      hiddenErrorCount(report({ valid: false, error_count: 1, errors })),
    ).toBe(0);
  });
  it("label lokasi untuk baris, kolom, dan file", () => {
    expect(locationLabel({ row: 3, column: "sell_price", message: "" })).toBe(
      "Baris 3 · sell_price",
    );
    expect(locationLabel({ row: 3, column: "", message: "" })).toBe("Baris 3");
    expect(locationLabel({ row: 0, column: "", message: "" })).toBe("File");
    expect(locationLabel({ row: 0, column: "sku", message: "" })).toBe(
      "File · sku",
    );
  });
});

describe("fileProblem", () => {
  it("menolak ekstensi salah, file kosong, dan terlalu besar", () => {
    expect(fileProblem({ name: "produk.xlsx", size: 100 })).toBeDefined();
    expect(fileProblem({ name: "produk.csv", size: 0 })).toBeDefined();
    expect(
      fileProblem({ name: "produk.csv", size: 2 * 1024 * 1024 + 1 }),
    ).toBeDefined();
  });
  it("menerima .csv (huruf besar juga) dalam batas ukuran", () => {
    expect(
      fileProblem({ name: "produk.csv", size: 2 * 1024 * 1024 }),
    ).toBeNull();
    expect(fileProblem({ name: "PRODUK.CSV", size: 10 })).toBeNull();
  });
});
