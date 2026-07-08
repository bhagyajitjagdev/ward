// Typed client for the Ward control-plane API. Base defaults to `/api`, which the
// Vite dev server proxies to the Go backend (see vite.config.ts). Field names mirror
// the Go DTOs exactly (snake_case).
const BASE = import.meta.env.VITE_API_BASE ?? "/api"
const TOKEN_KEY = "ward.token"

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}
export function setToken(t: string | null) {
  if (t) localStorage.setItem(TOKEN_KEY, t)
  else localStorage.removeItem(TOKEN_KEY)
}

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = {}
  const token = getToken()
  if (token) headers["Authorization"] = `Bearer ${token}`
  if (body !== undefined) headers["Content-Type"] = "application/json"

  const res = await fetch(`${BASE}${path}`, {
    method,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })

  if (res.status === 401) {
    setToken(null)
    // Bounce to login on an expired/invalid session — unless we're already there
    // (a bad-credentials 401 on the login/setup pages is handled locally).
    const p = window.location.pathname
    if (p !== "/login" && p !== "/setup") window.location.replace("/login")
  }

  const text = await res.text()
  const data = text ? JSON.parse(text) : null
  if (!res.ok) {
    throw new ApiError(res.status, (data && data.error) || res.statusText)
  }
  return data as T
}

function qs(params: object): string {
  const p = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== null && v !== "") p.set(k, String(v))
  }
  const s = p.toString()
  return s ? `?${s}` : ""
}

// --- types (mirror the Go DTOs) ---

export interface User {
  id: string
  username: string
  role: string
  is_owner: boolean
  created_at: string
}

export interface LoginResponse {
  token: string
  expires_at: string
  user: User
}

export interface Service {
  id: string
  name: string
  public_hostname: string
  upstreams: string[]
  lb_policy: string
  tls_mode: string
  waf_enabled: boolean
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface ServiceInput {
  name: string
  public_hostname: string
  upstreams: string[]
  lb_policy?: string
  tls_mode?: string
  waf_enabled?: boolean
}

export interface WafEvent {
  id: string
  tx_id: string
  ts: string
  service_id?: string | null
  host: string
  client_ip: string
  authed: boolean
  user_agent?: string
  method: string
  path: string
  uri: string
  status: number
  engine_mode: string
  is_interrupted: boolean
  rule_id: number
  rule_msg: string
  severity: string
  matched_target?: string
  matched_value?: string
  tags: string[]
  is_anomaly_score: boolean
  crs_version?: string
  raw?: string
}

export interface WafTrigger {
  service_id?: string | null
  host?: string
  path: string
  rule_id: number
  rule_msg: string
  severity: string
  matched_target?: string
  hits: number
  distinct_ips: number
  first_seen: string
  last_seen: string
}

export interface WafExclusion {
  id: string
  scope: "global" | "service"
  service_id?: string | null
  rule_id: number
  path?: string
  target?: string
  seclang: string
  state: string
  source: string
  created_at: string
}

export interface ExclusionInput {
  rule_id: number
  scope: "global" | "service"
  service_id?: string | null
  path?: string
  target?: string
}

export interface Block {
  id: string
  scope: "global" | "service"
  service_id?: string | null
  cidr: string
  reason?: string
  source: string
  expires_at?: string | null
  created_at: string
}

export interface BlockInput {
  cidr: string
  scope?: "global" | "service"
  service_id?: string | null
  reason?: string
  expires_at?: string | null
}

export interface ApiToken {
  id: string
  name: string
  user_id?: string | null
  last_used_at?: string | null
  expires_at?: string | null
  revoked: boolean
  created_at: string
  token?: string // only present on creation
}

export interface AuditEntry {
  id: string
  actor: string
  action: string
  target?: string
  detail?: string
  created_at: string
}

export interface Overview {
  services: number
  waf_services: number
  detections_24h: number
  blocked_24h: number
  active_blocks: number
  activity: { hour: string; detections: number; blocked: number }[]
  by_service: { service_id: string; detections_24h: number }[]
}

export interface WafEventQuery {
  service_id?: string
  path?: string
  client_ip?: string
  rule_id?: number
  since?: string
  limit?: number
}

export const api = {
  // auth
  setup: (username: string, password: string) =>
    request<LoginResponse>("POST", "/auth/setup", { username, password }),
  login: (username: string, password: string) =>
    request<LoginResponse>("POST", "/auth/login", { username, password }),
  logout: () => request<void>("POST", "/auth/logout"),
  me: () => request<User>("GET", "/auth/me"),

  // dashboard
  overview: () => request<Overview>("GET", "/overview"),

  // services
  listServices: () => request<Service[]>("GET", "/services"),
  createService: (input: ServiceInput) => request<Service>("POST", "/services", input),

  // waf
  listWafEvents: (q: WafEventQuery = {}) => request<WafEvent[]>("GET", `/waf-events${qs(q)}`),
  topTriggers: (q: { service_id?: string; since?: string; limit?: number } = {}) =>
    request<WafTrigger[]>("GET", `/waf-events/top${qs(q)}`),
  listExclusions: () => request<WafExclusion[]>("GET", "/waf-exclusions"),
  createExclusion: (input: ExclusionInput) => request<WafExclusion>("POST", "/waf-exclusions", input),
  deleteExclusion: (id: string) => request<void>("DELETE", `/waf-exclusions/${id}`),

  // blocklist
  listBlocklist: () => request<Block[]>("GET", "/blocklist"),
  createBlock: (input: BlockInput) => request<Block>("POST", "/blocklist", input),
  deleteBlock: (id: string) => request<void>("DELETE", `/blocklist/${id}`),

  // accounts
  listUsers: () => request<User[]>("GET", "/users"),
  createUser: (username: string, password: string) =>
    request<User>("POST", "/users", { username, password }),
  deleteUser: (id: string) => request<void>("DELETE", `/users/${id}`),
  listApiTokens: () => request<ApiToken[]>("GET", "/api-tokens"),
  createApiToken: (name: string) => request<ApiToken>("POST", "/api-tokens", { name }),
  revokeApiToken: (id: string) => request<void>("DELETE", `/api-tokens/${id}`),
  listAuditLog: (limit?: number) => request<AuditEntry[]>("GET", `/audit-log${qs({ limit })}`),
}
