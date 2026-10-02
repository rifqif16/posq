import { authedRequest } from "./api";
import type { Device } from "./device";

export type RegisteredDevice = Device & { secret: string };

export async function listDevices(): Promise<Device[]> {
  return (await authedRequest<{ items: Device[] }>("/v1/devices")).items;
}

export function registerDevice(
  storeId: string,
  name: string,
): Promise<RegisteredDevice> {
  return authedRequest<RegisteredDevice>("/v1/devices", {
    method: "POST",
    body: JSON.stringify({ store_id: storeId, name }),
  });
}

export function revokeDevice(id: string): Promise<void> {
  return authedRequest<void>(`/v1/devices/${id}/revoke`, { method: "POST" });
}
