import { authedRequest } from "./api";
import type { Category } from "./category";
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
