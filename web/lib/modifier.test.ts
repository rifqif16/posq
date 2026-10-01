import { describe, expect, it } from "vitest";
import {
  type ModifierGroup,
  type ModifierGroupForm,
  emptyGroupForm,
  emptyModifier,
  formFromGroup,
  moveItem,
  parseCount,
  priceDeltaLabel,
  selectionLabel,
  toGroupRequest,
  validateGroupForm,
} from "./modifier";

const valid = (): ModifierGroupForm => ({
  name: "Level Gula",
  min_select: "1",
  max_select: "1",
  modifiers: [
    { ...emptyModifier(), name: "Normal", is_default: true },
    { ...emptyModifier(), name: "Less" },
    { ...emptyModifier(), name: "Extra Shot", price_delta: "Rp 5.000" },
  ],
});

describe("parseCount", () => {
  it("menerima bilangan bulat tanpa tanda", () => {
    expect(parseCount(" 3 ")).toBe(3);
    expect(parseCount("0")).toBe(0);
  });
  it("menolak kosong, desimal, negatif, dan non-angka", () => {
    for (const bad of ["", "1.5", "-1", "abc", "1234567"])
      expect(parseCount(bad)).toBeNull();
  });
});

describe("validateGroupForm", () => {
  it("lolos untuk grup sah", () => {
    expect(validateGroupForm(valid())).toEqual({});
  });

  it("form kosong awal gagal pada nama dan opsi, tidak pada aturan pilih", () => {
    const errors = validateGroupForm(emptyGroupForm());
    expect(Object.keys(errors).sort()).toEqual([
      "modifiers[0].name",
      "modifiers[1].name",
      "name",
    ]);
  });

  it("max tidak boleh kurang dari min, min tidak boleh melebihi opsi aktif", () => {
    const f = valid();
    f.min_select = "2";
    expect(validateGroupForm(f).max_select).toBeDefined();
    f.max_select = "5";
    f.min_select = "4";
    expect(validateGroupForm(f).min_select).toBeDefined();
  });

  it("opsi nonaktif tidak dihitung untuk minimal pilihan", () => {
    const f = valid();
    f.modifiers[1].is_active = false;
    f.modifiers[2].is_active = false;
    f.min_select = "2";
    f.max_select = "2";
    expect(validateGroupForm(f).min_select).toBeDefined();
  });

  it("nama opsi wajib dan unik (tanpa membedakan huruf besar/kecil)", () => {
    const f = valid();
    f.modifiers[1].name = "normal";
    expect(validateGroupForm(f)["modifiers[1].name"]).toBeDefined();
    f.modifiers[1].name = " ";
    expect(validateGroupForm(f)["modifiers[1].name"]).toBeDefined();
  });

  it("harga opsi: kosong dianggap 0, di atas batas ditolak", () => {
    const f = valid();
    f.modifiers[0].price_delta = "1000000000";
    expect(validateGroupForm(f)).toEqual({});
    f.modifiers[0].price_delta = "1000000001";
    expect(validateGroupForm(f)["modifiers[0].price_delta"]).toBeDefined();
    f.modifiers[0].price_delta = "abc";
    expect(validateGroupForm(f)["modifiers[0].price_delta"]).toBeDefined();
  });

  it("default harus aktif dan tidak boleh melebihi maksimal pilihan", () => {
    const f = valid();
    f.modifiers[0].is_active = false;
    expect(validateGroupForm(f)["modifiers[0].is_default"]).toBeDefined();
    const g = valid();
    g.modifiers[1].is_default = true;
    expect(validateGroupForm(g).modifiers).toBeDefined();
  });

  it("maksimal 50 opsi", () => {
    const f = valid();
    f.max_select = "50";
    f.min_select = "0";
    f.modifiers = Array.from({ length: 51 }, (_, i) => ({
      ...emptyModifier(),
      name: `O${i}`,
    }));
    expect(validateGroupForm(f).modifiers).toBeDefined();
  });
});

describe("toGroupRequest & formFromGroup", () => {
  it("membentuk request: angka, id hanya untuk opsi lama, urutan dipertahankan", () => {
    const f = valid();
    f.modifiers[1].id = "existing";
    const req = toGroupRequest(f);
    expect(req).toMatchObject({
      name: "Level Gula",
      min_select: 1,
      max_select: 1,
    });
    expect(req.modifiers.map((m) => m.name)).toEqual([
      "Normal",
      "Less",
      "Extra Shot",
    ]);
    expect(req.modifiers[1].id).toBe("existing");
    expect("id" in req.modifiers[0]).toBe(false);
    expect(req.modifiers[2].price_delta).toBe(5000);
    expect(req.modifiers[0].price_delta).toBe(0);
  });

  it("formFromGroup membawa id dan menampilkan harga 0 sebagai kosong", () => {
    const g: ModifierGroup = {
      id: "g",
      name: "Topping",
      min_select: 0,
      max_select: 3,
      is_required: false,
      version: 4,
      modifiers: [
        {
          id: "a",
          name: "Boba",
          price_delta: 4000,
          is_default: false,
          is_active: true,
        },
        {
          id: "b",
          name: "Es",
          price_delta: 0,
          is_default: true,
          is_active: true,
        },
      ],
    };
    const f = formFromGroup(g);
    expect(f.modifiers[0]).toMatchObject({
      id: "a",
      key: "a",
      price_delta: "4000",
    });
    expect(f.modifiers[1].price_delta).toBe("");
    expect(f).toMatchObject({ min_select: "0", max_select: "3" });
  });
});

describe("moveItem", () => {
  it("memindahkan elemen dan tidak mengubah array asli", () => {
    const items = ["a", "b", "c"];
    expect(moveItem(items, 2, 0)).toEqual(["c", "a", "b"]);
    expect(moveItem(items, 0, 1)).toEqual(["b", "a", "c"]);
    expect(items).toEqual(["a", "b", "c"]);
  });
  it("indeks di luar jangkauan atau sama tidak mengubah", () => {
    const items = ["a", "b"];
    expect(moveItem(items, 0, 0)).toBe(items);
    expect(moveItem(items, -1, 1)).toBe(items);
    expect(moveItem(items, 0, 2)).toBe(items);
  });
});

describe("label", () => {
  it("selectionLabel", () => {
    expect(selectionLabel(0, 1)).toBe("Pilih 1 (opsional)");
    expect(selectionLabel(1, 1)).toBe("Pilih 1 (wajib)");
    expect(selectionLabel(2, 2)).toBe("Pilih tepat 2");
    expect(selectionLabel(0, 3)).toBe("Pilih hingga 3");
    expect(selectionLabel(1, 3)).toBe("Pilih 1–3");
  });
  it("priceDeltaLabel", () => {
    expect(priceDeltaLabel(0)).toBe("Gratis");
    expect(priceDeltaLabel(5000)).toBe("+Rp 5.000");
  });
});
