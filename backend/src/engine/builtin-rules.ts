/**
 * 内置规则集
 */

import type { Rule } from '../types.js';

/**
 * 内置规则优先级分配：
 * 100: PEM 私钥（最高危）
 * 90: API Keys / Tokens
 * 85: 连接字符串
 * 80: 手机号
 * 75: 身份证、银行卡
 * 70: 邮箱
 * 65: IP 地址
 * 50: 高熵检测
 */

export const BUILTIN_RULES: Rule[] = [
  // ============= 凭据类 (Priority 90-100) =============

  {
    id: 'pem-private-key',
    type: 'PRIVATE_KEY',
    name: 'PEM 私钥',
    description: '匹配 PEM 格式的私钥（RSA/EC/DSA/OPENSSH）',
    enabled: true,
    builtin: true,
    priority: 100,
    method: 'regex',
    pattern: '-----BEGIN (?:RSA |EC |DSA |OPENSSH |PGP )?PRIVATE KEY-----[\\s\\S]+?-----END[^-]*PRIVATE KEY-----',
    captureGroup: 0,
  },

  {
    id: 'openai-key',
    type: 'API_KEY',
    name: 'OpenAI API Key',
    description: '匹配 sk- 开头的 OpenAI API 密钥',
    enabled: true,
    builtin: true,
    priority: 90,
    method: 'regex',
    pattern: '\\bsk-[A-Za-z0-9]{20,}',
    captureGroup: 0,
  },

  {
    id: 'anthropic-key',
    type: 'API_KEY',
    name: 'Anthropic API Key',
    description: '匹配 Anthropic Claude API 密钥',
    enabled: true,
    builtin: true,
    priority: 90,
    method: 'regex',
    pattern: '\\bsk-ant-api03-[A-Za-z0-9_-]{93}AA',
    captureGroup: 0,
  },

  {
    id: 'github-token',
    type: 'API_KEY',
    name: 'GitHub Token',
    description: '匹配 GitHub Personal Access Token',
    enabled: true,
    builtin: true,
    priority: 90,
    method: 'regex',
    pattern: '\\b(?:ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9_]{36,255}\\b',
    captureGroup: 0,
  },

  {
    id: 'github-fine-grained',
    type: 'API_KEY',
    name: 'GitHub Fine-grained PAT',
    description: '匹配 GitHub Fine-grained Personal Access Token',
    enabled: true,
    builtin: true,
    priority: 90,
    method: 'regex',
    pattern: '\\bgithub_pat_[A-Za-z0-9_]{50,}',
    captureGroup: 0,
  },

  {
    id: 'gcp-api-key',
    type: 'API_KEY',
    name: 'Google Cloud API Key',
    description: '匹配 Google Cloud Platform API 密钥',
    enabled: true,
    builtin: true,
    priority: 90,
    method: 'regex',
    pattern: '\\bAIza[0-9A-Za-z_-]{35,38}',
    captureGroup: 0,
    allowList: ['AIzaSyabcdefghijklmnopqrstuvwxyz1234567'], // 示例密钥
  },

  {
    id: 'aws-access-key',
    type: 'ACCESS_KEY',
    name: 'AWS Access Key ID',
    description: '匹配 AWS Access Key ID',
    enabled: true,
    builtin: true,
    priority: 90,
    method: 'regex',
    pattern: '\\b(?:AKIA|ASIA|ABIA|ACCA)[A-Z0-9]{16}\\b',
    captureGroup: 0,
    allowList: ['AKIAIOSFODNN7EXAMPLE'], // AWS 示例密钥
  },

  {
    id: 'aws-secret-key',
    type: 'ACCESS_KEY',
    name: 'AWS Secret Access Key',
    description: '匹配 AWS Secret Access Key（键值对形式）',
    enabled: true,
    builtin: true,
    priority: 90,
    method: 'regex',
    pattern: '(?i)aws[_-]?secret[_-]?access[_-]?key["\']?\\s*[:=]\\s*["\']?([A-Za-z0-9/+=]{40})',
    captureGroup: 1,
  },

  {
    id: 'alibaba-access-key',
    type: 'ACCESS_KEY',
    name: '阿里云 Access Key',
    description: '匹配阿里云 Access Key ID',
    enabled: true,
    builtin: true,
    priority: 90,
    method: 'regex',
    pattern: '\\bLTAI[A-Za-z0-9]{12,20}',
    captureGroup: 0,
  },

  {
    id: 'jwt-token',
    type: 'JWT',
    name: 'JWT Token',
    description: '匹配 JSON Web Token',
    enabled: true,
    builtin: true,
    priority: 90,
    method: 'regex',
    pattern: '\\beyJ[A-Za-z0-9_-]{8,}\\.[A-Za-z0-9_-]{8,}\\.[A-Za-z0-9_-]{8,}',
    captureGroup: 0,
  },

  // ============= 连接字符串 (Priority 85) =============

  {
    id: 'connection-string',
    type: 'CONNSTR',
    name: '数据库连接字符串',
    description: '匹配 MySQL/PostgreSQL/MongoDB/Redis 连接字符串',
    enabled: true,
    builtin: true,
    priority: 85,
    method: 'regex',
    pattern: '\\b(?:mysql|postgresql|postgres|mongodb|redis|mssql):\\/\\/[^\\s<>"\']+',
    captureGroup: 0,
  },

  // ============= 中国 PII (Priority 75-80) =============

  {
    id: 'china-phone',
    type: 'PHONE',
    name: '中国手机号',
    description: '匹配中国大陆手机号码（1 开头 11 位）',
    enabled: true,
    builtin: true,
    priority: 80,
    method: 'regex',
    pattern: '\\b1[3-9]\\d{9}\\b',
    captureGroup: 0,
  },

  {
    id: 'china-id-card',
    type: 'IDENTITY',
    name: '中国身份证号',
    description: '匹配中国大陆身份证号码（18 位带校验）',
    enabled: true,
    builtin: true,
    priority: 75,
    method: 'regex',
    pattern: '\\b\\d{17}[\\dXx]\\b',
    captureGroup: 0,
    // TODO: 添加身份证校验位验证
  },

  {
    id: 'bank-card',
    type: 'BANK_CARD',
    name: '银行卡号',
    description: '匹配 13-19 位银行卡号（含 Luhn 校验）',
    enabled: true,
    builtin: true,
    priority: 75,
    method: 'regex',
    pattern: '\\b\\d{13,19}\\b',
    captureGroup: 0,
    // TODO: 添加 Luhn 校验
  },

  // ============= 邮箱 (Priority 70, 默认禁用) =============

  {
    id: 'email',
    type: 'EMAIL',
    name: '邮箱地址',
    description: '匹配电子邮件地址（默认禁用，误报率高）',
    enabled: false,
    builtin: true,
    priority: 70,
    method: 'regex',
    pattern: '\\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\\.[A-Z|a-z]{2,}\\b',
    captureGroup: 0,
  },

  // ============= IP 地址 (Priority 65) =============

  {
    id: 'ipv4-private',
    type: 'IPPRIVATE',
    name: '内网 IPv4 地址',
    description: '匹配私有 IPv4 地址（10.x, 172.16-31.x, 192.168.x）',
    enabled: true,
    builtin: true,
    priority: 65,
    method: 'regex',
    pattern: '\\b(?:10|172\\.(?:1[6-9]|2\\d|3[01])|192\\.168)\\.\\d{1,3}\\.\\d{1,3}\\b',
    captureGroup: 0,
    allowList: ['127.0.0.1', '0.0.0.0', '255.255.255.255'],
  },

  // ============= 高熵检测 (Priority 50) =============

  {
    id: 'high-entropy',
    type: 'SECRET',
    name: '高熵字符串',
    description: '基于交叉熵的高熵字符串检测（随机密钥/Token）',
    enabled: true,
    builtin: true,
    priority: 50,
    method: 'entropy',
    minLength: 9,
    entropyThreshold: 5.2, // length-aware threshold
  },
];

/**
 * 获取所有启用的规则（按优先级降序）
 */
export function getEnabledRules(rules: Rule[]): Rule[] {
  return rules
    .filter(r => r.enabled)
    .sort((a, b) => b.priority - a.priority);
}

/**
 * 根据 ID 查找规则
 */
export function findRuleById(rules: Rule[], id: string): Rule | undefined {
  return rules.find(r => r.id === id);
}

/**
 * 根据类型查找规则
 */
export function findRulesByType(rules: Rule[], type: string): Rule[] {
  return rules.filter(r => r.type === type);
}
