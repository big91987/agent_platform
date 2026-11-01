import { getJson } from "../../shared/api";
export interface Quota {
  tenant: string;
  limit: number;
  reserved: number;
  used: number;
}
export async function fetchQuota(
  tenant: string,
  signal: AbortSignal,
): Promise<Quota> {
  const value = await getJson(
    `/api/quotas/${encodeURIComponent(tenant)}`,
    signal,
  );
  if (!value || typeof value !== "object") throw new Error("配额数据格式错误");
  const q = value as Record<string, unknown>;
  if (
    q.tenant !== tenant ||
    !["limit", "reserved", "used"].every(
      (k) =>
        typeof q[k] === "number" && Number.isSafeInteger(q[k]) && q[k] >= 0,
    )
  ) {
    throw new Error("配额数据格式错误");
  }
  return q as unknown as Quota;
}
