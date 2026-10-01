import { describe, expect, it } from "vitest";
import {
  type Product, type ProductForm, emptyForm, emptyVariant, formFromProduct, makeDefault, parseBarcodes,
  sellPriceLabel, toRequest, toggleId, validateForm,
} from "./product";

const single = (): ProductForm => {
  const f = emptyForm();
  f.name = "Kopi";
  f.variants[0].sell_price = "15.000";
  return f;
};

const multi = (): ProductForm => {
  const f = emptyForm();
  f.name = "Kopi";
  f.has_variants = true;
  f.variants = [
    { ...emptyVariant(), name: "Small", sell_price: "10000" },
    { ...emptyVariant(), name: "Large", sell_price: "15000" },
  ];
  return f;
};

describe("parseBarcodes", () => {
  it("memisah koma/spasi/baris baru, membuang kosong dan duplikat", () => {
    expect(parseBarcodes("111, 222\n333  111,,")).toEqual(["111", "222", "333"]);
    expect(parseBarcodes("")).toEqual([]);
  });
});

describe("validateForm (tanpa varian)", () => {
  it("lolos untuk input minimal", () => {
    expect(validateForm(single())).toEqual({});
  });

  it("mengumpulkan semua kesalahan dengan kunci per path", () => {
    const f = single();
    f.name = " ";
    f.kitchen_station = "Bar Atas";
    Object.assign(f.variants[0], { sku: "ada spasi", barcodes: "bad code!", sell_price: "", cost_price: "abc" });
    expect(Object.keys(validateForm(f)).sort()).toEqual([
      "kitchen_station", "name", "variants[0].barcodes", "variants[0].cost_price", "variants[0].sell_price", "variants[0].sku",
    ]);
  });

  it("batas harga dan jumlah barcode", () => {
    const f = single();
    f.variants[0].sell_price = "1000000000";
    expect(validateForm(f)).toEqual({});
    f.variants[0].sell_price = "1000000001";
    expect(validateForm(f)["variants[0].sell_price"]).toBeDefined();
    f.variants[0].sell_price = "1";
    f.variants[0].barcodes = Array.from({ length: 11 }, (_, i) => `b${i}`).join(",");
    expect(validateForm(f)["variants[0].barcodes"]).toBeDefined();
  });

  it("mengabaikan varian lain yang tersisa saat mode tanpa varian", () => {
    const f = single();
    f.variants.push({ ...emptyVariant(), name: "", sell_price: "bukan angka" });
    expect(validateForm(f)).toEqual({});
  });
});

describe("validateForm (bervarian)", () => {
  it("lolos untuk dua varian bernama unik", () => {
    expect(validateForm(multi())).toEqual({});
  });

  it("wajib minimal 2 varian dan minimal satu aktif", () => {
    const one = multi();
    one.variants = one.variants.slice(0, 1);
    expect(validateForm(one).variants).toBeDefined();
    const none = multi();
    none.variants.forEach((v) => (v.is_active = false));
    expect(validateForm(none).variants).toBeDefined();
  });

  it("nama wajib dan unik (tanpa membedakan huruf besar/kecil)", () => {
    const f = multi();
    f.variants[1].name = "small";
    expect(validateForm(f)["variants[1].name"]).toBeDefined();
    f.variants[1].name = " ";
    expect(validateForm(f)["variants[1].name"]).toBeDefined();
  });

  it("SKU dan barcode tidak boleh kembar antar varian", () => {
    const f = multi();
    f.variants[0].sku = "K-1";
    f.variants[1].sku = "k-1";
    f.variants[0].barcodes = "9";
    f.variants[1].barcodes = "9";
    const errors = validateForm(f);
    expect(errors["variants[1].sku"]).toBeDefined();
    expect(errors["variants[1].barcodes"]).toBeDefined();
  });

  it("maksimal 20 varian", () => {
    const f = multi();
    f.variants = Array.from({ length: 21 }, (_, i) => ({ ...emptyVariant(), name: `V${i}`, sell_price: "1" }));
    expect(validateForm(f).variants).toBeDefined();
  });
});

describe("toRequest", () => {
  it("tanpa varian: satu varian, nama kosong, harga integer", () => {
    const f = single();
    Object.assign(f.variants[0], { sku: " K-1 ", barcodes: "1,2", cost_price: "Rp 8.000" });
    f.category_id = "c1";
    f.kitchen_station = " bar ";
    const req = toRequest(f);
    expect(req).toMatchObject({ name: "Kopi", category_id: "c1", kitchen_station: "bar" });
    expect(req.variants).toEqual([{ name: "", sku: "K-1", barcodes: ["1", "2"], cost_price: 8000, sell_price: 15000, is_active: true }]);
  });

  it("bervarian: mempertahankan id, urutan, dan status aktif", () => {
    const f = multi();
    f.variants[0].id = "existing";
    f.variants[1].is_active = false;
    const req = toRequest(f);
    expect(req.variants.map((v) => v.name)).toEqual(["Small", "Large"]);
    expect(req.variants[0].id).toBe("existing");
    expect("id" in req.variants[1]).toBe(false);
    expect(req.variants[1].is_active).toBe(false);
  });

  it("kategori kosong menjadi null dan harga beli kosong menjadi 0", () => {
    const req = toRequest(single());
    expect(req.category_id).toBeNull();
    expect(req.variants[0].cost_price).toBe(0);
  });
});

describe("makeDefault", () => {
  it("memindahkan elemen ke depan tanpa mengubah urutan lainnya", () => {
    expect(makeDefault(["a", "b", "c", "d"], 2)).toEqual(["c", "a", "b", "d"]);
  });
  it("tidak berubah untuk indeks 0 atau di luar jangkauan", () => {
    const items = ["a", "b"];
    expect(makeDefault(items, 0)).toBe(items);
    expect(makeDefault(items, 5)).toBe(items);
  });
});

const product = (variants: Partial<Product["variants"][number]>[]): Product => ({
  id: "p", name: "Kopi", type: variants.length > 1 ? "variant" : "simple", category_id: null, taxable: true,
  track_stock: false, kitchen_station: "", is_active: true, version: 3, modifier_group_ids: [],
  variants: variants.map((v, i) => ({
    id: `v${i}`, name: `V${i}`, sku: `S${i}`, barcodes: [], cost_price: null, sell_price: 1000,
    is_default: i === 0, is_active: true, ...v,
  })),
});

describe("formFromProduct", () => {
  it("harga beli tersembunyi (null) menjadi string kosong, id dan barcode terbawa", () => {
    const f = formFromProduct(product([{ barcodes: ["1", "2"] }]));
    expect(f.variants[0]).toMatchObject({ id: "v0", cost_price: "", barcodes: "1\n2", key: "v0" });
    expect(f.has_variants).toBe(false);
    expect(f.category_id).toBe("");
  });
  it("lebih dari satu varian mengaktifkan mode bervarian", () => {
    expect(formFromProduct(product([{}, {}])).has_variants).toBe(true);
  });
});

describe("sellPriceLabel", () => {
  it("satu harga atau rentang dari varian aktif", () => {
    expect(sellPriceLabel(product([{ sell_price: 12500 }]))).toBe("Rp 12.500");
    expect(sellPriceLabel(product([{ sell_price: 10000 }, { sell_price: 15000 }]))).toBe("Rp 10.000 – Rp 15.000");
    expect(sellPriceLabel(product([{ sell_price: 10000 }, { sell_price: 99000, is_active: false }]))).toBe("Rp 10.000");
  });
  it("memakai semua varian bila semuanya nonaktif", () => {
    expect(sellPriceLabel(product([{ sell_price: 1000, is_active: false }, { sell_price: 2000, is_active: false }]))).toBe("Rp 1.000 – Rp 2.000");
  });
});

describe("penautan grup modifier", () => {
  it("toggleId menambah di akhir dan melepas tanpa mengubah urutan sisanya", () => {
    expect(toggleId([], "a")).toEqual(["a"]);
    expect(toggleId(["a", "b"], "c")).toEqual(["a", "b", "c"]);
    expect(toggleId(["a", "b", "c"], "b")).toEqual(["a", "c"]);
    const ids = ["a"];
    toggleId(ids, "b");
    expect(ids).toEqual(["a"]);
  });

  it("toRequest dan formFromProduct membawa urutan grup", () => {
    const f = single();
    f.modifier_group_ids = ["g2", "g1"];
    expect(toRequest(f).modifier_group_ids).toEqual(["g2", "g1"]);
    const p = product([{}]);
    p.modifier_group_ids = ["x", "y"];
    const back = formFromProduct(p);
    expect(back.modifier_group_ids).toEqual(["x", "y"]);
    back.modifier_group_ids.push("z");
    expect(p.modifier_group_ids).toEqual(["x", "y"]); // salinan, bukan referensi yang sama
  });

  it("maksimal 10 grup per produk", () => {
    const f = single();
    f.modifier_group_ids = Array.from({ length: 10 }, (_, i) => `g${i}`);
    expect(validateForm(f)).toEqual({});
    f.modifier_group_ids.push("g10");
    expect(validateForm(f).modifier_group_ids).toBeDefined();
  });
});
