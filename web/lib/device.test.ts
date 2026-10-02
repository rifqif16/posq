import { describe, expect, it } from "vitest";
import {
  type KeyValueStore,
  type StoredDevice,
  appendPinDigit,
  clearDevice,
  deviceStatusLabel,
  isStoredDevice,
  loadDevice,
  removePinDigit,
  saveDevice,
  validateDeviceName,
} from "./device";

function memoryStore(
  initial: Record<string, string> = {},
): KeyValueStore & { data: Record<string, string> } {
  const data = { ...initial };
  return {
    data,
    getItem: (k) => (k in data ? data[k] : null),
    setItem: (k, v) => {
      data[k] = v;
    },
    removeItem: (k) => {
      delete data[k];
    },
  };
}

const device: StoredDevice = {
  device_id: "0190f2a1-7c11-7000-8000-000000000001",
  store_id: "0190f2a1-7c11-7000-8000-000000000002",
  device_secret: "x".repeat(43),
  name: "Kasir Depan",
};

describe("penyimpanan perangkat", () => {
  it("simpan, muat, dan hapus", () => {
    const store = memoryStore();
    expect(loadDevice(store)).toBeNull();
    saveDevice(store, device);
    expect(loadDevice(store)).toEqual(device);
    clearDevice(store);
    expect(loadDevice(store)).toBeNull();
  });
  it("data rusak atau tidak lengkap dianggap tidak ada", () => {
    expect(
      loadDevice(memoryStore({ "posq.device": "{bukan json" })),
    ).toBeNull();
    expect(
      loadDevice(
        memoryStore({
          "posq.device": JSON.stringify({ ...device, device_id: "abc" }),
        }),
      ),
    ).toBeNull();
    expect(
      loadDevice(
        memoryStore({
          "posq.device": JSON.stringify({ ...device, device_secret: "pendek" }),
        }),
      ),
    ).toBeNull();
    expect(loadDevice(memoryStore({ "posq.device": "null" }))).toBeNull();
  });
  it("isStoredDevice memeriksa bentuk", () => {
    expect(isStoredDevice(device)).toBe(true);
    expect(isStoredDevice({ ...device, name: 5 })).toBe(false);
    expect(isStoredDevice(undefined)).toBe(false);
  });
});

describe("PIN pad", () => {
  it("menambah digit sampai 6 dan mengabaikan non-digit", () => {
    let pin = "";
    for (const d of "1234567") pin = appendPinDigit(pin, d);
    expect(pin).toBe("123456");
    expect(appendPinDigit("12", "a")).toBe("12");
    expect(appendPinDigit("12", "34")).toBe("12");
  });
  it("menghapus digit terakhir dengan aman", () => {
    expect(removePinDigit("123")).toBe("12");
    expect(removePinDigit("")).toBe("");
  });
});

describe("perangkat", () => {
  it("validateDeviceName", () => {
    expect(validateDeviceName("Kasir Depan")).toBeUndefined();
    expect(validateDeviceName("  ")).toBeDefined();
    expect(validateDeviceName("a".repeat(61))).toBeDefined();
    expect(validateDeviceName("a".repeat(60))).toBeUndefined();
  });
  it("deviceStatusLabel", () => {
    expect(deviceStatusLabel("active")).toBe("Aktif");
    expect(deviceStatusLabel("revoked")).toBe("Dicabut");
  });
});
