import { describe, expect, it } from "vitest";
import { formatRupiah, parseRupiah } from "./money";

describe("formatRupiah", () => {
  it("memakai pemisah ribuan titik", () => {
    expect(formatRupiah(0)).toBe("Rp 0");
    expect(formatRupiah(12500)).toBe("Rp 12.500");
    expect(formatRupiah(1234567)).toBe("Rp 1.234.567");
  });
});

describe("parseRupiah", () => {
  it("mengambil digit dari input bebas", () => {
    expect(parseRupiah("Rp 12.500")).toBe(12500);
    expect(parseRupiah(" 7000 ")).toBe(7000);
    expect(parseRupiah("0")).toBe(0);
  });
  it("null untuk kosong, non-digit, atau terlalu besar", () => {
    expect(parseRupiah("")).toBeNull();
    expect(parseRupiah("abc")).toBeNull();
    expect(parseRupiah("9".repeat(20))).toBeNull();
  });
});
