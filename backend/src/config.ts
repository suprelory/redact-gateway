/**
 * 配置管理模块
 */

import { config as loadEnv } from 'dotenv';
import { existsSync, mkdirSync, readFileSync } from 'fs';
import { resolve } from 'path';
import type { GatewayConfig } from './types.js';

loadEnv();

const DEFAULT_MASTER_KEY_LENGTH = 32;
const DEFAULT_PROXY_PORT = 18787;
const DEFAULT_MANAGEMENT_PORT = 18788;
const DEFAULT_MAX_BODY_BYTES = 32 * 1024 * 1024; // 32 MiB
const DEFAULT_MAX_REDACTIONS = 16384;
const DEFAULT_MAPPING_TTL = 7 * 24 * 60 * 60; // 7 days

/**
 * 从环境变量或密钥文件加载主密钥
 */
function loadMasterKey(dataDir: string): string {
  // 优先使用环境变量
  if (process.env.MASTER_KEY) {
    const key = process.env.MASTER_KEY;
    if (key.length < DEFAULT_MASTER_KEY_LENGTH) {
      throw new Error(`MASTER_KEY must be at least ${DEFAULT_MASTER_KEY_LENGTH} characters`);
    }
    return key;
  }

  // 尝试从文件读取
  const keyFile = resolve(dataDir, 'master.key');
  if (existsSync(keyFile)) {
    const key = readFileSync(keyFile, 'utf-8').trim();
    if (key.length < DEFAULT_MASTER_KEY_LENGTH) {
      throw new Error(`Master key in ${keyFile} is too short`);
    }
    return key;
  }

  throw new Error(
    'Master key not found. Set MASTER_KEY environment variable or create a master.key file in the data directory.'
  );
}

/**
 * 加载网关配置
 */
export function loadConfig(): GatewayConfig {
  const dataDir = process.env.DATA_DIR || resolve(process.cwd(), 'data');

  // 确保数据目录存在
  if (!existsSync(dataDir)) {
    mkdirSync(dataDir, { recursive: true });
  }

  const masterKey = loadMasterKey(dataDir);

  const allowedHostsEnv = process.env.ALLOWED_HOSTS || '';
  const allowedHosts = allowedHostsEnv
    ? allowedHostsEnv.split(',').map(h => h.trim()).filter(Boolean)
    : [];

  return {
    proxyPort: parseInt(process.env.PROXY_PORT || String(DEFAULT_PROXY_PORT), 10),
    managementPort: parseInt(process.env.MANAGEMENT_PORT || String(DEFAULT_MANAGEMENT_PORT), 10),
    dataDir,
    logLevel: process.env.LOG_LEVEL || 'info',
    corsOrigin: process.env.CORS_ORIGIN || '*',
    maxBodyBytes: parseInt(process.env.MAX_BODY_BYTES || String(DEFAULT_MAX_BODY_BYTES), 10),
    maxRedactions: parseInt(process.env.MAX_REDACTIONS || String(DEFAULT_MAX_REDACTIONS), 10),
    mappingTTL: parseInt(process.env.MAPPING_TTL || String(DEFAULT_MAPPING_TTL), 10),
    allowedHosts,
    masterKey,
  };
}

/**
 * 验证配置有效性
 */
export function validateConfig(config: GatewayConfig): void {
  if (config.proxyPort === config.managementPort) {
    throw new Error('PROXY_PORT and MANAGEMENT_PORT must be different');
  }

  if (config.proxyPort < 1 || config.proxyPort > 65535) {
    throw new Error('PROXY_PORT must be between 1 and 65535');
  }

  if (config.managementPort < 1 || config.managementPort > 65535) {
    throw new Error('MANAGEMENT_PORT must be between 1 and 65535');
  }

  if (config.maxBodyBytes < 1024 || config.maxBodyBytes > 128 * 1024 * 1024) {
    throw new Error('MAX_BODY_BYTES must be between 1KB and 128MB');
  }

  if (config.maxRedactions < 1 || config.maxRedactions > 1000000) {
    throw new Error('MAX_REDACTIONS must be between 1 and 1,000,000');
  }

  if (config.mappingTTL < 60) {
    throw new Error('MAPPING_TTL must be at least 60 seconds');
  }
}
