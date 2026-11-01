export class HttpError extends Error {
  constructor(readonly status: number) {
    super(`请求失败（${status}）`);
  }
}
export async function getJson(
  path: string,
  signal: AbortSignal,
): Promise<unknown> {
  const response = await fetch(path, { signal, credentials: "same-origin" });
  if (!response.ok) throw new HttpError(response.status);
  return response.json();
}
