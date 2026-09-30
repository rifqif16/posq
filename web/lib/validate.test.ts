import { describe, expect, it } from "vitest";
import { validateEmail, validatePassword, validateRegister } from "./validate";

describe("validatePassword", () => {
  it("menolak 9 karakter, menerima 10 dan 128", () => {
    expect(validatePassword("a".repeat(9))).toBeDefined();
    expect(validatePassword("a".repeat(10))).toBeUndefined();
    expect(validatePassword("a".repeat(128))).toBeUndefined();
    expect(validatePassword("a".repeat(129))).toBeDefined();
  });
});

describe("validateEmail", () => {
  it("menerima format valid dan menolak yang cacat", () => {
    expect(validateEmail(" sari@kedai.id ")).toBeUndefined();
    expect(validateEmail("tanpa-at.com")).toBeDefined();
    expect(validateEmail("a@b")).toBeDefined();
    expect(validateEmail("")).toBeDefined();
  });
});

describe("validateRegister", () => {
  it("mengumpulkan semua field yang salah", () => {
    const errors = validateRegister({
      business_name: "",
      owner_name: " ",
      email: "x",
      password: "pendek",
    });
    expect(Object.keys(errors).sort()).toEqual([
      "business_name",
      "email",
      "owner_name",
      "password",
    ]);
  });

  it("kosong bila semua valid", () => {
    const errors = validateRegister({
      business_name: "Kedai",
      owner_name: "Sari",
      email: "sari@kedai.id",
      password: "password-aman",
    });
    expect(errors).toEqual({});
  });
});
