import { authedRequest } from "./api";
import type {
  CreateStaffRequest,
  StaffMember,
  StaffRole,
  StoreInfo,
  UpdateStaffRequest,
} from "./staff";

export interface StaffList {
  items: StaffMember[];
  assignable_roles: StaffRole[];
}

export function listStaff(): Promise<StaffList> {
  return authedRequest<StaffList>("/v1/staff");
}

export async function listStores(): Promise<StoreInfo[]> {
  return (await authedRequest<{ items: StoreInfo[] }>("/v1/stores")).items;
}

export function createStaff(req: CreateStaffRequest): Promise<StaffMember> {
  return authedRequest<StaffMember>("/v1/staff", {
    method: "POST",
    body: JSON.stringify(req),
  });
}

export function updateStaff(
  id: string,
  req: UpdateStaffRequest,
): Promise<StaffMember> {
  return authedRequest<StaffMember>(`/v1/staff/${id}`, {
    method: "PATCH",
    body: JSON.stringify(req),
  });
}

export function resetStaffPassword(
  id: string,
  password: string,
): Promise<void> {
  return authedRequest<void>(`/v1/staff/${id}/password`, {
    method: "POST",
    body: JSON.stringify({ password }),
  });
}

export function setStaffPin(id: string, pin: string): Promise<void> {
  return authedRequest<void>(`/v1/staff/${id}/pin`, {
    method: "PUT",
    body: JSON.stringify({ pin }),
  });
}

export function clearStaffPin(id: string): Promise<void> {
  return authedRequest<void>(`/v1/staff/${id}/pin`, { method: "DELETE" });
}
