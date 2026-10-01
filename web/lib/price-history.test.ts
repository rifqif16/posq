import { describe, expect, it } from "vitest";
import {
  type PriceChange,
  describeChange,
  fieldLabel,
  formatDateTime,
  showVariantName,
} from "./price-history";

const change = (patch: Partial<PriceChange> = {}): PriceChange => ({
  id: "c1",
  variant_id: "v1",
  variant_name: "Default",
  field: "sell_price",
  old_value: 15000,
  new_value: 16000,
  changed_by: { id: "u1", name: "Sari" },
  at: "2026-10-01T08:15:22Z",
  ...patch,
});

describe("describeChange", () => {
  it("kenaikan harga jual", () => {
    expect(describeChange(change())).toEqual({
      fieldLabel: "Harga jual",
      from: "Rp 15.000",
      to: "Rp 16.000",
      direction: "naik",
      difference: "+Rp 1.000",
    });
  });
  it("penurunan harga beli", () => {
    expect(
      describeChange(
        change({ field: "cost_price", old_value: 9000, new_value: 8000 }),
      ),
    ).toMatchObject({
      fieldLabel: "Harga beli",
      direction: "turun",
      difference: "-Rp 1.000",
    });
  });
  it("perubahan ke atau dari nol", () => {
    expect(
      describeChange(change({ old_value: 0, new_value: 500 })).difference,
    ).toBe("+Rp 500");
    expect(
      describeChange(change({ old_value: 500, new_value: 0 })).difference,
    ).toBe("-Rp 500");
  });
});

describe("label dan tampilan", () => {
  it("fieldLabel", () => {
    expect(fieldLabel("sell_price")).toBe("Harga jual");
    expect(fieldLabel("cost_price")).toBe("Harga beli");
  });
  it("formatDateTime mengembalikan teks asli bila tanggal tidak valid", () => {
    expect(formatDateTime("bukan-tanggal")).toBe("bukan-tanggal");
    expect(formatDateTime("2026-10-01T08:15:22Z")).not.toBe(
      "2026-10-01T08:15:22Z",
    );
  });
  it("showVariantName hanya bila ada lebih dari satu varian atau nama bukan Default", () => {
    expect(showVariantName([change(), change({ id: "c2" })])).toBe(false);
    expect(
      showVariantName([change(), change({ id: "c2", variant_id: "v2" })]),
    ).toBe(true);
    expect(showVariantName([change({ variant_name: "Large" })])).toBe(true);
    expect(showVariantName([])).toBe(false);
  });
});
