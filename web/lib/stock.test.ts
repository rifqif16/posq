import { describe, expect, it } from "vitest";
import {
  collectCounts,
  displayQty,
  emptyMovementForm,
  formatSigned,
  isNegative,
  itemLabel,
  movementTypeLabel,
  normalizeQty,
  toMovementRequest,
  validateMovementForm,
} from "./stock";

describe("normalizeQty", () => {
  it("menerima titik atau koma desimal dan membuang spasi", () => {
    expect(normalizeQty(" 12,5 ")).toBe("12.5");
    expect(normalizeQty("0.001")).toBe("0.001");
    expect(normalizeQty("7")).toBe("7");
  });
  it("menolak negatif, 4 desimal, kosong, dan teks", () => {
    for (const bad of ["", "-1", "1.2345", "abc", "1.", ".5", "1234567890"])
      expect(normalizeQty(bad)).toBeNull();
  });
});

describe("tampilan", () => {
  it("displayQty memangkas nol di belakang koma dan memakai format Indonesia", () => {
    expect(displayQty("12.500")).toBe("12,5");
    expect(displayQty("1000.000")).toBe("1.000");
    expect(displayQty("0.000")).toBe("0");
    expect(displayQty("xyz")).toBe("xyz");
  });
  it("formatSigned memberi tanda plus hanya untuk positif", () => {
    expect(formatSigned("10.000")).toBe("+10");
    expect(formatSigned("-2.500")).toBe("-2,5");
    expect(formatSigned("0.000")).toBe("0");
  });
  it("isNegative", () => {
    expect(isNegative("-0.500")).toBe(true);
    expect(isNegative("0.000")).toBe(false);
    expect(isNegative("3.000")).toBe(false);
  });
  it("itemLabel menyembunyikan nama varian Default", () => {
    expect(itemLabel({ product_name: "Kopi", variant_name: "Default" })).toBe(
      "Kopi",
    );
    expect(itemLabel({ product_name: "Kopi", variant_name: "Large" })).toBe(
      "Kopi · Large",
    );
  });
  it("movementTypeLabel", () => {
    expect(movementTypeLabel("purchase_receive")).toBe("Stok masuk");
    expect(movementTypeLabel("opname")).toBe("Opname");
  });
});

describe("validateMovementForm", () => {
  it("stok masuk tanpa alasan lolos", () => {
    expect(
      validateMovementForm({ ...emptyMovementForm("receive"), qty: "5" }),
    ).toEqual({});
  });
  it("waste dan koreksi wajib beralasan", () => {
    for (const kind of ["waste", "adjust_in", "adjust_out"] as const) {
      expect(
        validateMovementForm({ ...emptyMovementForm(kind), qty: "1" }).reason,
      ).toBeDefined();
      expect(
        validateMovementForm({
          ...emptyMovementForm(kind),
          qty: "1",
          reason: "tumpah",
        }),
      ).toEqual({});
    }
  });
  it("jumlah nol, kosong, dan tidak valid ditolak", () => {
    for (const qty of ["", "0", "0,000", "abc", "-1"])
      expect(
        validateMovementForm({ ...emptyMovementForm(), qty }).qty,
      ).toBeDefined();
  });
  it("harga satuan opsional tetapi harus angka valid", () => {
    expect(
      validateMovementForm({
        ...emptyMovementForm(),
        qty: "1",
        unit_cost: "Rp 90.000",
      }),
    ).toEqual({});
    expect(
      validateMovementForm({
        ...emptyMovementForm(),
        qty: "1",
        unit_cost: "abc",
      }).unit_cost,
    ).toBeDefined();
    expect(
      validateMovementForm({
        ...emptyMovementForm(),
        qty: "1",
        unit_cost: "1000000001",
      }).unit_cost,
    ).toBeDefined();
  });
  it("alasan lebih dari 200 karakter ditolak", () => {
    expect(
      validateMovementForm({
        ...emptyMovementForm("waste"),
        qty: "1",
        reason: "a".repeat(201),
      }).reason,
    ).toBeDefined();
  });
});

describe("toMovementRequest", () => {
  const build = (kind: Parameters<typeof emptyMovementForm>[0], extra = {}) =>
    toMovementRequest("s1", "v1", {
      ...emptyMovementForm(kind),
      qty: "2,5",
      ...extra,
    });

  it("tanda jumlah mengikuti jenis pergerakan", () => {
    expect(build("receive")).toMatchObject({
      type: "purchase_receive",
      qty_delta: "2.5",
    });
    expect(build("waste", { reason: "x" })).toMatchObject({
      type: "waste",
      qty_delta: "-2.5",
    });
    expect(build("adjust_in", { reason: "x" })).toMatchObject({
      type: "adjustment",
      qty_delta: "2.5",
    });
    expect(build("adjust_out", { reason: "x" })).toMatchObject({
      type: "adjustment",
      qty_delta: "-2.5",
    });
  });
  it("harga satuan hanya untuk stok masuk dan alasan dipangkas", () => {
    expect(build("receive", { unit_cost: "Rp 90.000" }).unit_cost).toBe(90000);
    expect(
      "unit_cost" in build("waste", { reason: " x ", unit_cost: "5000" }),
    ).toBe(false);
    expect(build("waste", { reason: " rusak " }).reason).toBe("rusak");
    expect("reason" in build("receive")).toBe(false);
  });
});

describe("collectCounts", () => {
  it("melewati kolom kosong, menormalkan angka, dan melaporkan yang salah", () => {
    const { items, errors } = collectCounts({
      a: "5",
      b: "",
      c: " 2,5 ",
      d: "-1",
      e: "abc",
      f: "0",
    });
    expect(items).toEqual([
      { variant_id: "a", counted_qty: "5" },
      { variant_id: "c", counted_qty: "2.5" },
      { variant_id: "f", counted_qty: "0" },
    ]);
    expect(Object.keys(errors).sort()).toEqual(["d", "e"]);
  });
});
