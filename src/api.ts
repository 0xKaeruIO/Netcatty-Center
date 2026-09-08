import { ApiError } from "./types";

type ApiOptions = Omit<RequestInit, "body"> & { body?: unknown };

export async function api<T = Record<string, unknown>>(
  path: string,
  options: ApiOptions = {},
): Promise<T> {
  const { body, headers, ...rest } = options;
  const response = await fetch(path, {
    credentials: "same-origin",
    headers: {
      "Content-Type": "application/json",
      ...(headers || {}),
    },
    ...rest,
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const data = (await response.json().catch(() => ({}))) as { error?: string };
  if (!response.ok) {
    throw new ApiError(data.error || `请求失败 (${response.status})`, response.status);
  }
  return data as T;
}

export function formatTime(ts?: number | null): string {
  if (!ts) return "—";
  return new Date(ts).toLocaleString("zh-CN", { hour12: false });
}

export function permissionLabel(permission?: string): string {
  return permission === "readwrite" ? "可读可写" : "只读";
}
