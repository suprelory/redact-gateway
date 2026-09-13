export interface GatewayStatus {
  service: string
  version: string
  proxy_addr: string
  admin_addr: string
  started_at: string
  uptime_seconds: number
  in_flight: number
  allowed_hosts: number
  allow_private_upstreams: boolean
  max_body_bytes: number
}

export interface GatewaySettings {
  allowed_hosts: string[]
}

export interface GatewayEvent {
  id: number
  request_id: string
  timestamp: string
  method: string
  protocol: string
  upstream_scheme: string
  upstream_host: string
  upstream_port?: string
  upstream_path: string
  flags: string
  streaming: boolean
  status: number
  duration_ms: number
  request_bytes: number
  response_bytes: number
  redaction_count: number
  restore_count: number
  rule_hits: Record<string, number>
  redaction_fields: string[]
  error_class?: string
}

export interface GatewayEventPage {
  events: GatewayEvent[]
  total: number
  page: number
  limit: number
}

export interface GatewayStats {
  overview: {
    requests: number
    redactions: number
    restores: number
    errors: number
    average_ms: number
  }
  top_upstreams: Array<{
    host: string
    requests: number
    errors: number
    average_ms: number
  }>
}

export interface RuleInfo {
  flag: string
  name: string
  description: string
  default: boolean
}
