/**
 * 网关入口文件
 */

import { loadConfig, validateConfig } from './config.js';
import { initLogger, getLogger } from './logger.js';

async function main() {
  try {
    // 加载配置
    const config = loadConfig();
    validateConfig(config);

    // 初始化日志
    initLogger(config);
    const logger = getLogger();

    logger.info({ config: { ...config, masterKey: '[REDACTED]' } }, 'Configuration loaded');

    // TODO: 初始化存储
    // TODO: 初始化脱敏引擎
    // TODO: 启动代理服务器
    // TODO: 启动管理服务器

    logger.info('Redact Gateway is starting...');
    logger.info(`Proxy port: ${config.proxyPort}`);
    logger.info(`Management port: ${config.managementPort}`);

    // 优雅关闭
    const shutdown = async (signal: string) => {
      logger.info(`Received ${signal}, shutting down gracefully...`);
      // TODO: 关闭服务器
      // TODO: 关闭数据库连接
      process.exit(0);
    };

    process.on('SIGTERM', () => shutdown('SIGTERM'));
    process.on('SIGINT', () => shutdown('SIGINT'));
  } catch (error) {
    console.error('Failed to start Redact Gateway:', error);
    process.exit(1);
  }
}

main();
