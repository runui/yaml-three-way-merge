import type { Algorithm, APIError, FixtureDetail, FixturePage, MergeResponse } from "./types";

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, init);
  const body = (await response.json()) as T | APIError;
  if (!response.ok) {
    const apiError = body as APIError;
    throw new Error(`${apiError.error?.stage ?? "request"}: ${apiError.error?.message ?? response.statusText}`);
  }
  return body as T;
}

export async function getAlgorithms(): Promise<Algorithm[]> {
  const response = await request<{ items: Algorithm[] }>("/api/algorithms");
  return response.items;
}

export function getFixtures(query: string, pageSize = 30): Promise<FixturePage> {
  const params = new URLSearchParams({ q: query, page_size: String(pageSize) });
  return request(`/api/fixtures?${params}`);
}

export function getFixture(id: string): Promise<FixtureDetail> {
  return request(`/api/fixtures/${encodeURIComponent(id)}`);
}

export function runMerge(payload: {
  algorithm: string;
  previous_base: string;
  user_override: string;
  target_base: string;
  expected?: string;
}): Promise<MergeResponse> {
  return request("/api/merge", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
}
