import { describe, expect, it } from "vitest";
import {
  type StaffMember,
  emptyStaffForm,
  formFromMember,
  roleLabel,
  storeNames,
  toCreateRequest,
  toUpdateRequest,
  toggleStore,
  validatePin,
  validateStaffForm,
} from "./staff";

const filled = () => ({
  ...emptyStaffForm("cashier", ["s1"]),
  name: "Budi",
  email: "budi@kedai.id",
  password: "password-aman-1",
});

describe("validatePin", () => {
  it("menerima 6 digit yang tidak mudah ditebak", () => {
    for (const ok of ["135790", "482915", "100200"])
      expect(validatePin(ok)).toBeUndefined();
  });
  it("menolak panjang salah, bukan angka, dan PIN lemah", () => {
    for (const bad of [
      "",
      "12345",
      "1234567",
      "12a456",
      "000000",
      "999999",
      "123456",
      "654321",
    ])
      expect(validatePin(bad)).toBeDefined();
  });
});

describe("validateStaffForm", () => {
  it("lolos untuk input minimal pada mode buat dan ubah", () => {
    expect(validateStaffForm(filled(), "create")).toEqual({});
    expect(
      validateStaffForm({ ...filled(), email: "", password: "" }, "edit"),
    ).toEqual({});
  });
  it("mengumpulkan kesalahan per field pada mode buat", () => {
    const errors = validateStaffForm(
      {
        ...filled(),
        name: " ",
        email: "salah",
        password: "pendek",
        pin: "111111",
        store_ids: [],
      },
      "create",
    );
    expect(Object.keys(errors).sort()).toEqual([
      "email",
      "name",
      "password",
      "pin",
      "store_ids",
    ]);
  });
  it("PIN opsional dan email/password tidak divalidasi pada mode ubah", () => {
    expect(validateStaffForm({ ...filled(), pin: "" }, "create")).toEqual({});
    expect(
      validateStaffForm(
        { ...filled(), email: "salah", password: "x", pin: "1" },
        "edit",
      ),
    ).toEqual({});
  });
  it("maksimal 20 outlet", () => {
    const ids = Array.from({ length: 21 }, (_, i) => `s${i}`);
    expect(
      validateStaffForm({ ...filled(), store_ids: ids }, "create").store_ids,
    ).toBeDefined();
  });
});

describe("pembentuk request", () => {
  it("toCreateRequest memangkas nama, memperkecil email, dan menyertakan PIN hanya bila diisi", () => {
    const req = toCreateRequest({
      ...filled(),
      name: " Budi ",
      email: " Budi@Kedai.ID ",
    });
    expect(req).toEqual({
      name: "Budi",
      email: "budi@kedai.id",
      password: "password-aman-1",
      role: "cashier",
      store_ids: ["s1"],
    });
    expect(toCreateRequest({ ...filled(), pin: "135790" }).pin).toBe("135790");
  });
  it("toUpdateRequest memetakan checkbox aktif ke status", () => {
    expect(toUpdateRequest({ ...filled(), active: false }).status).toBe(
      "disabled",
    );
    expect(toUpdateRequest(filled()).status).toBe("active");
    expect("password" in toUpdateRequest(filled())).toBe(false);
  });
});

describe("formFromMember dan tampilan", () => {
  const member: StaffMember = {
    id: "u1",
    name: "Sari",
    email: "sari@kedai.id",
    role: "kitchen",
    status: "disabled",
    store_ids: ["s1", "s2"],
    has_pin: true,
    manageable: true,
    created_at: "2026-10-01T00:00:00Z",
  };
  it("membawa nama, role, outlet, dan status; rahasia dikosongkan", () => {
    const f = formFromMember(member);
    expect(f).toMatchObject({
      name: "Sari",
      role: "kitchen",
      store_ids: ["s1", "s2"],
      active: false,
      password: "",
      pin: "",
    });
    f.store_ids.push("s3");
    expect(member.store_ids).toEqual(["s1", "s2"]);
  });
  it("roleLabel dan storeNames", () => {
    expect(roleLabel("owner")).toBe("Pemilik");
    expect(roleLabel("cashier")).toBe("Kasir");
    expect(
      storeNames(["s1", "x"], [{ id: "s1", code: "A", name: "Pusat" }]),
    ).toBe("Pusat, Outlet tidak dikenal");
  });
  it("toggleStore menambah dan melepas", () => {
    expect(toggleStore(["a"], "b")).toEqual(["a", "b"]);
    expect(toggleStore(["a", "b"], "a")).toEqual(["b"]);
  });
});
