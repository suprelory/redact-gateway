import type { GatewayEvent, GatewayStats, GatewayStatus, RuleInfo } from './types'

const TOKEN_KEY = 'redact_gateway_admin_token'

export function readToken(): string {
  const fragment = window.location.hash.startsWith('#token=')
    ? new URLSearchParams(window.location.hash.slice(1)).get('token')?.trim() ?? ''
    : ''
  if (fragment) {
    sessionStorage.setItem(TOKEN_KEY, fragment)
    window.history.replaceState(null, '', `${window.location.pathname}${window.location.search}#/`)
    return fragment
  }
  return sessionStorage.getItem(TOKEN_KEY) ?? ''
}

export function saveToken(token: string): void {
  sessionStorage.setItem(TOKEN_KEY, token)
}

export function clearToken(): void {
  sessionStorage.removeItem(TOKEN_KEY)
}

async function request<T>(path: string, token = readToken()): Promise<T> {
  const response = await fetch(path, {
    headers: token ? { 'X-Redact-Token': token } : undefined,
  })
  if (!response.ok) {
    const body = await response.json().catch(() => null) as { error?: { message?: string } } | null
    const error = new Error(body?.error?.message || `HTTP ${response.status}`)
    Object.assign(error, { status: response.status })
    throw error
  }
  return response.json() as Promise<T>
}

export const api = {
  status: (token?: string) => request<GatewayStatus>('/api/v1/status', token),
  events: (limit = 100) => request<{ events: GatewayEvent[] }>(`/api/v1/events?limit=${limit}`),
  stats: (hours = 24) => request<GatewayStats>(`/api/v1/stats?hours=${hours}`),
  rules: () => request<{ all_flags: string; rules: RuleInfo[] }>('/api/v1/rules'),
}
