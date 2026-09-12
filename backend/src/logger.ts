/**
 * 日志模块 - 使用 Pino
 */

import pino from 'pino';
import type { GatewayConfig } from './types.js';

let logger: pino.Logger;

/**
 * 初始化日志系统
 */
export function initLogger(config: GatewayConfig): void {
  const isDev = process.env.NODE_ENV !== 'production';

  logger = pino({
    level: config.logLevel,
    transport: isDev
      ? {
          target: 'pino-pretty',
          options: {
            colorize: true,
            translateTime: 'HH:MM:ss.l',
            ignore: 'pid,hostname',
          },
        }
      : undefined,
    redact: {
      paths: [
        'req.headers.authorization',
        'req.headers["x-api-key"]',
        'req.headers["api-key"]',
        'req.body',
        'res.body',
        'plaintext',
        'mappings',
      ],
      censor: '[REDACTED]',
    },
  });
}

/**
 * 获取日志实例
 */
export function getLogger(): pino.Logger {
  if (!logger) {
    throw new Error('Logger not initialized. Call initLogger() first.');
  }
  return logger;
}
