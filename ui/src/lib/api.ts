// Typed client for the Ward control-plane API, generated from the backend's
// OpenAPI spec (backend/internal/api/openapi.yaml → `npm run generate:api` →
// api.schema.d.ts). Paths, params, bodies and responses are all spec-checked:
// an API change shows up here as a TypeScript error, not a runtime surprise.
// Base defaults to `/api`, which the Vite dev server proxies to the Go backend.
import createClient from "openapi-fetch"
import type { paths, components } from "./api.schema"

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

const client = createClient<paths>({ baseUrl: BASE })

client.use({
  onRequest({ request }) {
    const token = getToken()
    if (token) request.headers.set("Authorization", `Bearer ${token}`)
    return request
  },
  onResponse({ response }) {
    if (response.status === 401) {
      setToken(null)
      // Bounce to login on an expired/invalid session — unless we're already there
      // (a bad-credentials 401 on the login/setup pages is handled locally).
      const p = window.location.pathname
      if (p !== "/login" && p !== "/setup") window.location.replace("/login")
    }
    return response
  },
})

// unwrap turns openapi-fetch's {data, error, response} into the throw-on-error
// promise shape the rest of the app uses.
function unwrap<T>(r: { data?: T; error?: unknown; response: Response }): T {
  if (r.error !== undefined || !r.response.ok) {
    const msg = (r.error as { error?: string } | undefined)?.error || r.response.statusText
    throw new ApiError(r.response.status, msg)
  }
  return r.data as T
}

// --- types (aliases into the generated schema — names kept from the old client) ---

type S = components["schemas"]

export type User = S["User"]
export type LoginResponse = S["LoginResponse"]
export type WafMode = S["WafMode"]
export type Service = S["Service"]
export type ServiceInput = S["ServiceInput"]
export type ServiceUpdate = S["ServiceUpdate"]
export type Settings = S["Settings"]
export type Certificate = S["Certificate"]
export type CertificateInput = S["CertificateInput"]
export type WafEvent = S["WafEvent"]
export type WafTrigger = S["WafTrigger"]
export type WafExclusion = S["WafExclusion"]
export type ExclusionInput = S["ExclusionInput"]
export type WafCustomRule = S["WafCustomRule"]
export type WafCustomRuleInput = S["WafCustomRuleInput"]
export type Block = S["Block"]
export type BlockInput = S["BlockInput"]
export type RateLimit = S["RateLimit"]
export type RateLimitInput = S["RateLimitInput"]
export type GeoRule = S["GeoRule"]
export type GeoRuleInput = S["GeoRuleInput"]
export type GeoIPStatus = S["GeoIPStatus"]
export type ApiToken = S["ApiToken"]
export type AuditEntry = S["AuditEntry"]
export type Overview = S["Overview"]
export type AccessEvent = S["AccessEvent"]
export type AccessStats = S["AccessStats"]
export type CrowdSecStatus = S["CrowdSecStatus"]
export type CrowdSecDecision = S["CrowdSecDecision"]

export type WafEventQuery = NonNullable<paths["/waf-events"]["get"]["parameters"]["query"]>
export type AccessQuery = NonNullable<paths["/access-events"]["get"]["parameters"]["query"]>

export const api = {
  // meta
  getVersion: () => client.GET("/version").then(unwrap),

  // auth
  setup: (username: string, password: string) =>
    client.POST("/auth/setup", { body: { username, password } }).then(unwrap),
  setupState: () => client.GET("/auth/state").then(unwrap),
  login: (username: string, password: string) =>
    client.POST("/auth/login", { body: { username, password } }).then(unwrap),
  logout: () => client.POST("/auth/logout").then(unwrap),
  me: () => client.GET("/auth/me").then(unwrap),

  // dashboard
  overview: () => client.GET("/overview").then(unwrap),

  // services
  listServices: () => client.GET("/services").then(unwrap),
  getService: (id: string) => client.GET("/services/{id}", { params: { path: { id } } }).then(unwrap),
  createService: (input: ServiceInput) => client.POST("/services", { body: input }).then(unwrap),
  updateService: (id: string, input: ServiceUpdate) =>
    client.PATCH("/services/{id}", { params: { path: { id } }, body: input }).then(unwrap),
  deleteService: (id: string) => client.DELETE("/services/{id}", { params: { path: { id } } }).then(unwrap),

  // waf
  listWafEvents: (q: WafEventQuery = {}) => client.GET("/waf-events", { params: { query: q } }).then(unwrap),
  topTriggers: (q: { service_id?: string; since?: string; limit?: number } = {}) =>
    client.GET("/waf-events/top", { params: { query: q } }).then(unwrap),
  listAccessEvents: (q: AccessQuery = {}) => client.GET("/access-events", { params: { query: q } }).then(unwrap),
  accessStats: (q: { service_id?: string; since?: string } = {}) =>
    client.GET("/access-events/stats", { params: { query: q } }).then(unwrap),
  listExclusions: () => client.GET("/waf-exclusions").then(unwrap),
  createExclusion: (input: ExclusionInput) => client.POST("/waf-exclusions", { body: input }).then(unwrap),
  deleteExclusion: (id: string) =>
    client.DELETE("/waf-exclusions/{id}", { params: { path: { id } } }).then(unwrap),

  // custom raw-SecLang rules (advanced)
  listWafCustomRules: () => client.GET("/waf-custom-rules").then(unwrap),
  createWafCustomRule: (input: WafCustomRuleInput) =>
    client.POST("/waf-custom-rules", { body: input }).then(unwrap),
  updateWafCustomRule: (id: string, input: WafCustomRuleInput) =>
    client.PATCH("/waf-custom-rules/{id}", { params: { path: { id } }, body: input }).then(unwrap),
  deleteWafCustomRule: (id: string) =>
    client.DELETE("/waf-custom-rules/{id}", { params: { path: { id } } }).then(unwrap),

  // blocklist
  listBlocklist: () => client.GET("/blocklist").then(unwrap),
  createBlock: (input: BlockInput) => client.POST("/blocklist", { body: input }).then(unwrap),
  updateBlock: (id: string, input: BlockInput) =>
    client.PATCH("/blocklist/{id}", { params: { path: { id } }, body: input }).then(unwrap),
  deleteBlock: (id: string) => client.DELETE("/blocklist/{id}", { params: { path: { id } } }).then(unwrap),

  // rate limits
  listRateLimits: () => client.GET("/rate-limits").then(unwrap),
  createRateLimit: (input: RateLimitInput) => client.POST("/rate-limits", { body: input }).then(unwrap),
  updateRateLimit: (id: string, input: RateLimitInput) =>
    client.PATCH("/rate-limits/{id}", { params: { path: { id } }, body: input }).then(unwrap),
  deleteRateLimit: (id: string) =>
    client.DELETE("/rate-limits/{id}", { params: { path: { id } } }).then(unwrap),

  // geo blocking
  listGeoRules: () => client.GET("/geo-rules").then(unwrap),
  createGeoRule: (input: GeoRuleInput) => client.POST("/geo-rules", { body: input }).then(unwrap),
  updateGeoRule: (id: string, input: GeoRuleInput) =>
    client.PATCH("/geo-rules/{id}", { params: { path: { id } }, body: input }).then(unwrap),
  deleteGeoRule: (id: string) => client.DELETE("/geo-rules/{id}", { params: { path: { id } } }).then(unwrap),

  // geoip database source
  geoipStatus: () => client.GET("/geoip").then(unwrap),
  geoipDBIP: () => client.POST("/geoip/dbip").then(unwrap),
  geoipMaxMind: (license_key: string) =>
    client.POST("/geoip/maxmind", { body: { license_key } }).then(unwrap),
  geoipDelete: () => client.DELETE("/geoip").then(unwrap),
  // Multipart upload stays hand-rolled (FormData sets its own boundary header);
  // the response type still comes from the generated schema.
  geoipUpload: async (file: File): Promise<GeoIPStatus> => {
    const fd = new FormData()
    fd.append("file", file)
    const token = getToken()
    const res = await fetch(`${BASE}/geoip/upload`, {
      method: "POST",
      headers: token ? { Authorization: `Bearer ${token}` } : {},
      body: fd,
    })
    if (res.status === 401) setToken(null)
    const text = await res.text()
    const data = text ? JSON.parse(text) : null
    if (!res.ok) throw new ApiError(res.status, (data && data.error) || res.statusText)
    return data as GeoIPStatus
  },

  // accounts
  listUsers: () => client.GET("/users").then(unwrap),
  createUser: (username: string, password: string) =>
    client.POST("/users", { body: { username, password } }).then(unwrap),
  deleteUser: (id: string) => client.DELETE("/users/{id}", { params: { path: { id } } }).then(unwrap),
  listApiTokens: () => client.GET("/api-tokens").then(unwrap),
  createApiToken: (name: string) => client.POST("/api-tokens", { body: { name } }).then(unwrap),
  revokeApiToken: (id: string) =>
    client.DELETE("/api-tokens/{id}", { params: { path: { id } } }).then(unwrap),
  listAuditLog: (limit?: number) => client.GET("/audit-log", { params: { query: { limit } } }).then(unwrap),

  // settings
  getSettings: () => client.GET("/settings").then(unwrap),
  updateSettings: (input: Partial<Settings>) => client.PATCH("/settings", { body: input }).then(unwrap),

  // crowdsec (read-only status + active decisions)
  crowdsecStatus: () => client.GET("/crowdsec").then(unwrap),

  // custom TLS certificates
  listCertificates: () => client.GET("/certificates").then(unwrap),
  uploadCertificate: (input: CertificateInput) => client.POST("/certificates", { body: input }).then(unwrap),
  deleteCertificate: (domain: string) =>
    client.DELETE("/certificates/{domain}", { params: { path: { domain } } }).then(unwrap),
}
