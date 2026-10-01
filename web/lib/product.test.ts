import { describe, expect, it } from "vitest";
import {
  type Product,
  emptyForm,
  formFromProduct,
  mapServerFields,
  parseBarcodes,
  toRequest,
  validateForm,
} from "./product";

const valid = () => ({ ...emptyForm(), name: "Kopi", sell_price: "15.000" });

describe("parseBarcodes", () => {
  it("memisah koma/spasi/baris baru, membuang kosong dan duplikat", () => {
    expect(parseBarcodes("111, 222\n333  111,,")).toEqual([
      "111",
      "222",
      "333",
    ]);
    expect(parseBarcodes("")).toEqual([]);
  });
});

describe("validateForm", () => {
  it("lolos untuk input minimal", () => {
    expect(validateForm(valid())).toEqual({});
  });

  it("mengumpulkan semua kesalahan", () => {
    const errors = validateForm({
      ...emptyForm(),
      name: " ",
      sku: "ada spasi",
      barcodes: "bad code!",
      sell_price: "",
      cost_price: "abc",
      kitchen_station: "Bar Atas",
    });
    expect(Object.keys(errors).sort()).toEqual([
      "barcodes",
      "cost_price",
      "kitchen_station",
      "name",
      "sell_price",
      "sku",
    ]);
  });

  it("batas harga dan jumlah barcode", () => {
    expect(validateForm({ ...valid(), sell_price: "1000000000" })).toEqual({});
    expect(
      validateForm({ ...valid(), sell_price: "1000000001" }).sell_price,
    ).toBeDefined();
    const eleven = Array.from({ length: 11 }, (_, i) => `b${i}`).join(",");
    expect(
      validateForm({ ...valid(), barcodes: eleven }).barcodes,
    ).toBeDefined();
  });
});

describe("toRequest", () => {
  it("membentuk request dengan satu varian dan harga integer", () => {
    const req = toRequest({
      ...valid(),
      category_id: "c1",
      sku: " K-1 ",
      barcodes: "1,2",
      cost_price: "Rp 8.000",
      kitchen_station: " bar ",
    });
    expect(req).toMatchObject({
      name: "Kopi",
      category_id: "c1",
      kitchen_station: "bar",
    });
    expect(req.variants).toEqual([
      {
        name: "",
        sku: "K-1",
        barcodes: ["1", "2"],
        cost_price: 8000,
        sell_price: 15000,
      },
    ]);
  });

  it("kategori kosong menjadi null dan harga beli kosong menjadi 0", () => {
    const req = toRequest(valid());
    expect(req.category_id).toBeNull();
    expect(req.variants[0].cost_price).toBe(0);
  });
});

describe("formFromProduct & mapServerFields", () => {
  const product: Product = {
    id: "p",
    name: "Kopi",
    type: "simple",
    category_id: null,
    taxable: true,
    track_stock: false,
    kitchen_station: "",
    is_active: true,
    version: 3,
    variants: [
      {
        id: "v",
        name: "Default",
        sku: "K",
        barcodes: ["1", "2"],
        cost_price: null,
        sell_price: 15000,
        is_default: true,
      },
    ],
  };

  it("harga beli tersembunyi (null) menjadi string kosong", () => {
    const f = formFromProduct(product);
    expect(f.cost_price).toBe("");
    expect(f.barcodes).toBe("1\n2");
    expect(f.category_id).toBe("");
  });

  it("memetakan kunci error varian ke field form", () => {
    expect(mapServerFields({ "variants[0].sku": "x", name: "y" })).toEqual({
      sku: "x",
      name: "y",
    });
  });
});
