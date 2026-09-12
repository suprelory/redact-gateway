/**
 * 前端类型定义
 */

export type RuleMethod = 'regex' | 'dict' | 'entropy';

export interface Rule {
  id: string;
  type: string;
  name: string;
  description?: string;
  enabled: boolean;
  builtin: boolean;
  priority: number;
  method: RuleMethod;
  pattern?: string;
  captureGroup?: number;
  dictionary?: string[];
  caseSensitive?: boolean;
  wordBoundary?: boolean;
  minLength?: number;
  entropyThreshold?: number;
  allowList?: string[];
}

export interface RuleHit {
  ruleId: string;
  ruleType: string;
  count: number;
}

export interface LogEvent {
  id: string;
  timestamp: number;
  requestId: string;
  method: string;
  path: string;
  protocol: string;
  status: number;
  duration: number;
  streaming: boolean;
  hitRules: RuleHit[];
  redactionCount: number;
  restorationCount: number;
  creatorHash: string;
  error?: string;
}

export interface Stats {
  totalRequests: number;
  totalRedactions: number;
  totalRestorations: number;
  ruleHitStats: Record<string, number>;
  protocolStats: Record<string, number>;
  errorCount: number;
  period: {
    start: number;
    end: number;
  };
}

export interface UpstreamConfig {
  id: string;
  name: string;
  protocol: string;
  baseUrl: string;
  enabled: boolean;
  timeout?: number;
  headers?: Record<string, string>;
}

export interface SystemStatus {
  running: boolean;
  version: string;
  uptime: number;
  proxyPort: number;
  managementPort: number;
}
