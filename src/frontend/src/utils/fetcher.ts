const TIMEOUT = 30000 as const;

// @ts-ignore: vite specific
export const API_BASE = import.meta.env.DEV ? "http://localhost:6969/api/v1" : "/api/v1";

// TODO: proper error handling
export async function fetcher<T>(...args: any[]): Promise<T> {
  const url = args.shift();

  try {
    const res = await fetch(`${API_BASE}${url}`, ...args);
    if (!res.ok || res.status < 200 || res.status > 299) {
      if (res.body) {
        throw new Error(await res.text());
      } else {
        throw new Error(res.statusText);
      }
    }
    return (await res.json()) as T;
  } catch (e) {
    console.error("Fetch error:", e);
    throw e;
  }
}

fetcher.get = <T>(url: string) =>
  fetcher<T>(url, {
    method: "GET",
    signal: AbortSignal.timeout(TIMEOUT),
  });

fetcher.post = <T>(url: string, body: any, options?: { timeout?: number }) =>
  fetcher<T>(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
    signal: AbortSignal.timeout(options?.timeout ?? TIMEOUT),
  });

fetcher.put = <T>(url: string, body: any) =>
  fetcher<T>(url, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
    signal: AbortSignal.timeout(TIMEOUT),
  });

fetcher.delete = <T>(url: string) =>
  fetcher<T>(url, {
    method: "DELETE",
    signal: AbortSignal.timeout(TIMEOUT),
  });
