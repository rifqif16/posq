import { describe, expect, it } from "vitest";
import { buildTree, validateCategoryName, type Category } from "./category";

const cat = (id: string, parent_id: string | null, name = id): Category => ({ id, parent_id, name, sort_order: 0 });

describe("validateCategoryName", () => {
  it("menolak kosong dan terlalu panjang, menerima batas 100", () => {
    expect(validateCategoryName("  ")).toBeDefined();
    expect(validateCategoryName("a".repeat(101))).toBeDefined();
    expect(validateCategoryName("a".repeat(100))).toBeUndefined();
    expect(validateCategoryName(" Kopi ")).toBeUndefined();
  });
});

describe("buildTree", () => {
  it("mengelompokkan anak di bawah induk dan mempertahankan urutan input", () => {
    const tree = buildTree([cat("a", null), cat("b", null), cat("a1", "a"), cat("a2", "a"), cat("b1", "b")]);
    expect(tree.map((n) => n.id)).toEqual(["a", "b"]);
    expect(tree[0].children.map((c) => c.id)).toEqual(["a1", "a2"]);
    expect(tree[1].children.map((c) => c.id)).toEqual(["b1"]);
  });

  it("kosong untuk input kosong dan mengabaikan anak yatim", () => {
    expect(buildTree([])).toEqual([]);
    expect(buildTree([cat("x", "hilang")])).toEqual([]);
  });
});
