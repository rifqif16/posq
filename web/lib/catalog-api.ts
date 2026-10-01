import { authedRequest } from "./api";
import type { Category } from "./category";
import type { ModifierGroup, ModifierGroupRequest } from "./modifier";
import type { Product, ProductRequest } from "./product";

export async function listCategories(): Promise<Category[]> {
  const res = await authedRequest<{ items: Category[] }>("/v1/categories");
  return res.items;
}

export function createCategory(input: {
  name: string;
  parent_id: string | null;
  sort_order?: number;
}): Promise<Category> {
  return authedRequest<Category>("/v1/categories", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function updateCategory(
  id: string,
  input: { name: string; sort_order: number },
): Promise<Category> {
  return authedRequest<Category>(`/v1/categories/${id}`, {
    method: "PATCH",
    body: JSON.stringify(input),
  });
}

export function deleteCategory(id: string): Promise<void> {
  return authedRequest<void>(`/v1/categories/${id}`, { method: "DELETE" });
}

export interface ProductList {
  items: Product[];
  next_cursor: string | null;
}

export function listProducts(params: {
  q?: string;
  category_id?: string;
  cursor?: string;
  limit?: number;
}): Promise<ProductList> {
  const qs = new URLSearchParams();
  for (const [k, v] of Object.entries(params))
    if (v !== undefined && v !== "") qs.set(k, String(v));
  const suffix = qs.size > 0 ? `?${qs}` : "";
  return authedRequest<ProductList>(`/v1/products${suffix}`);
}

export function getProduct(id: string): Promise<Product> {
  return authedRequest<Product>(`/v1/products/${id}`);
}

export function createProduct(input: ProductRequest): Promise<Product> {
  return authedRequest<Product>("/v1/products", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function updateProduct(
  id: string,
  version: number,
  input: ProductRequest,
): Promise<Product> {
  return authedRequest<Product>(`/v1/products/${id}`, {
    method: "PATCH",
    headers: { "If-Match": `"${version}"` },
    body: JSON.stringify(input),
  });
}

export function deleteProduct(id: string): Promise<void> {
  return authedRequest<void>(`/v1/products/${id}`, { method: "DELETE" });
}

export function listModifierGroups(): Promise<{ items: ModifierGroup[] }> {
  return authedRequest<{ items: ModifierGroup[] }>("/v1/modifier-groups");
}

export function getModifierGroup(id: string): Promise<ModifierGroup> {
  return authedRequest<ModifierGroup>(`/v1/modifier-groups/${id}`);
}

export function createModifierGroup(
  input: ModifierGroupRequest,
): Promise<ModifierGroup> {
  return authedRequest<ModifierGroup>("/v1/modifier-groups", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function updateModifierGroup(
  id: string,
  version: number,
  input: ModifierGroupRequest,
): Promise<ModifierGroup> {
  return authedRequest<ModifierGroup>(`/v1/modifier-groups/${id}`, {
    method: "PATCH",
    headers: { "If-Match": `"${version}"` },
    body: JSON.stringify(input),
  });
}

export function deleteModifierGroup(id: string): Promise<void> {
  return authedRequest<void>(`/v1/modifier-groups/${id}`, { method: "DELETE" });
}
