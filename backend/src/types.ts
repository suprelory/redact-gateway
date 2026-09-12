/**
 * 脱敏网关类型定义
 */

// ============= 规则系统 =============

export type RuleMethod = 'regex' | 'dict' | 'entropy';

export interface Rule {
  id: string;
  type: string;              // PHONE, EMAIL, API_KEY, CONNSTR, etc.
  name: string;
  description?: string;
  enabled: boolean;
  builtin: boolean;          // 内置规则不可删除
  priority: number;          // 越高越优先（100=最高）
  method: RuleMethod;

  // 正则规则
  pattern?: string;          // 正则表达式字符串
  captureGroup?: number;     // 捕获组索引，默认 0（整个匹配）

  // 字典规则
  dictionary?: string[];     // 敏感词列表
  caseSensitive?: boolean;   // 大小写敏感
  wordBoundary?: boolean;    // 词边界匹配

  // 熵检测规则
  minLength?: number;        // 最小长度
  entropyThreshold?: number; // 熵阈值（length-aware）

  // 白名单
  allowList?: string[];      // 允许列表（不脱敏）
}

// ============= 映射系统 =============

export interface Mapping {
  placeholder: string;       // {{TYPE_ULID}}
  plaintext: string;         // 原始明文
  ruleType: string;          // 规则类型
  creator: string;           // SHA256(normalized_credential)
  createdAt: number;         // Unix 时间戳（毫秒）
  expiresAt: number;         // 过期时间
}

export interface MappingEntry {
  id: number;                // 数据库主键
  placeholder: string;
  encrypted: Buffer;         // AES-256-GCM 加密后的明文
  ruleType: string;
  creator: string;
  createdAt: number;
  expiresAt: number;
  plaintextIndex: string;    // HMAC-SHA256(creator:plaintext)
}

// ============= 脱敏结果 =============

export interface RedactionResult {
  redacted: string;          // 脱敏后的文本
  mappings: Mapping[];       // 新创建的映射
  hitRules: RuleHit[];       // 命中的规则统计
  totalHits: number;         // 总命中次数
}

export interface RuleHit {
  ruleId: string;
  ruleType: string;
  count: number;
}

// ============= 还原结果 =============

export interface RestorationResult {
  restored: string;          // 还原后的文本
  restoredCount: number;     // 还原的占位符数量
  missedCount: number;       // 未找到映射的占位符数量
}

// ============= 请求/响应 =============

export interface ProxyRequest {
  id: string;                // 请求 ID (ULID)
  method: string;
  url: string;
  path: string;
  protocol: Protocol;
  creator: string;
  headers: Record<string, string>;
  body?: unknown;
  timestamp: number;
}

export interface ProxyResponse {
  requestId: string;
  status: number;
  headers: Record<string, string>;
  body?: unknown;
  duration: number;          // 毫秒
  streaming: boolean;
}

export type Protocol = 'openai-chat' | 'openai-responses' | 'anthropic-messages' | 'generic';

// ============= 日志事件 =============

export interface LogEvent {
  id: string;                // ULID
  timestamp: number;
  requestId: string;
  method: string;
  path: string;
  protocol: Protocol;
  status: number;
  duration: number;
  streaming: boolean;
  hitRules: RuleHit[];
  redactionCount: number;
  restorationCount: number;
  creatorHash: string;       // creator 的前 8 位
  error?: string;
}

// ============= 统计数据 =============

export interface Stats {
  totalRequests: number;
  totalRedactions: number;
  totalRestorations: number;
  ruleHitStats: Record<string, number>;
  protocolStats: Record<Protocol, number>;
  errorCount: number;
  period: {
    start: number;
    end: number;
  };
}

// ============= 配置 =============

export interface GatewayConfig {
  proxyPort: number;
  managementPort: number;
  dataDir: string;
  logLevel: string;
  corsOrigin: string;
  maxBodyBytes: number;
  maxRedactions: number;
  mappingTTL: number;        // 秒
  allowedHosts: string[];    // 空数组表示允许所有
  masterKey: string;
}

// ============= 上游配置 =============

export interface UpstreamConfig {
  id: string;
  name: string;
  protocol: Protocol;
  baseUrl: string;
  enabled: boolean;
  timeout?: number;          // 秒
  headers?: Record<string, string>;
}

// ============= API 响应 =============

export interface ApiResponse<T = unknown> {
  success: boolean;
  data?: T;
  error?: {
    code: string;
    message: string;
  };
}

export interface PaginatedResponse<T> {
  items: T[];
  total: number;
  page: number;
  pageSize: number;
}
