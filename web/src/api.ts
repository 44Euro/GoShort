function isAdminView(): boolean {
  return window.location.pathname.startsWith("/admin");
}

export class ApiError extends Error {
  constructor(
    readonly status: number,
    message: string,
  ) {
    super(message);
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    credentials: "same-origin",
    headers: init?.body ? { "Content-Type": "application/json" } : undefined,
    ...init,
  });

  // token อยู่ใน httpOnly cookie อ่านวันหมดอายุจาก JS ไม่ได้ จึงต้องรู้ตัวจาก 401
  // ที่ตอบกลับมาแทน แล้วพากลับไปหน้า login ไม่ปล่อยให้หน้าจอเต็มไปด้วย error
  if (res.status === 401 && !path.endsWith("/api/admin/me") && isAdminView()) {
    window.location.assign("/login");
  }

  if (!res.ok) {
    let message = res.statusText;
    try {
      const body = (await res.json()) as { error?: string };
      if (body.error) message = body.error;
    } catch {
      /* ไม่ใช่ JSON ก็ใช้ statusText ไป */
    }
    throw new ApiError(res.status, message);
  }
  return (await res.json()) as T;
}

export const api = {
  get: <T,>(path: string) => request<T>(path),
  post: <T,>(path: string, body?: unknown) =>
    request<T>(path, { method: "POST", body: body ? JSON.stringify(body) : undefined }),
  del: <T,>(path: string) => request<T>(path, { method: "DELETE" }),
};

export type CreatedLink = { code: string; short_url: string; long_url: string };
export type DayPoint = { day: string; clicks: number };
export type Referrer = { name: string; clicks: number; percent: number };

export type PublicStats = {
  cache_hit_rate: number;
  p99_redirect_ms: number;
  dropped_events: number;
  total_clicks: number;
};

export type LinkStats = {
  code: string;
  long_url: string;
  clicks: number;
  series: DayPoint[];
  referrers: Referrer[];
};

export type AdminLink = {
  code: string;
  long_url: string;
  clicks: number;
  status: "active" | "expired";
  custom_alias: boolean;
  created_at: string;
  last_14_days: number[];
};

export type Summary = {
  cache_hit_rate: number;
  p99_redirect_ms: number;
  queue_depth: number;
  queue_capacity: number;
  dropped_events: number;
  written_events: number;
  total_clicks: number;
};

export type Overview = {
  series: DayPoint[];
  top_links: { rank: number; code: string; long_url: string; clicks: number }[];
  referrers: Referrer[];
};

export type Analytics = {
  code: string;
  long_url: string;
  clicks: number;
  unique: number;
  per_day: number;
  status: "active" | "expired";
  cache: { warm: boolean; ttl_seconds: number };
  series: DayPoint[];
  referrers: Referrer[];
  events: { time: string; referrer: string; user_agent: string; ip_hash: string }[];
  created_at: string;
};
