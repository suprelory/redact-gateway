/**
 * 内存存储实现（用于开发和测试）
 *
 * 生产环境应使用 SQLite 持久化存储
 */

import type { Mapping } from '../types.js';
import { generateIndex } from './crypto.js';
import { logger } from '../logger.js';

export interface MappingStorage {
  createMapping(mapping: Mapping): void;
  getMapping(placeholder: string): Mapping | null;
  findByPlaintext(plaintext: string, creator: string): Mapping | null;
  deleteExpired(): number;
  close(): void;
}

/**
 * 创建内存映射表存储
 *
 * @param masterKey - 主密钥（用于生成索引）
 * @returns MappingStorage 实例
 */
export function createMemoryMappingStorage(
  masterKey: string,
): MappingStorage {
  // placeholder -> Mapping
  const mappings = new Map<string, Mapping>();

  // index_hash -> placeholder (用于快速查找)
  const indexes = new Map<string, string>();

  logger.info('Memory mapping storage initialized');

  return {
    createMapping(mapping: Mapping): void {
      try {
        // 生成索引
        const indexHash = generateIndex(
          mapping.plaintext,
          mapping.creator,
          masterKey,
        );

        // 存储映射
        mappings.set(mapping.placeholder, mapping);
        indexes.set(indexHash, mapping.placeholder);

        logger.debug({
          placeholder: mapping.placeholder,
          ruleType: mapping.ruleType,
          creator: mapping.creator,
        }, 'Mapping created (memory)');
      } catch (err) {
        logger.error({ err, placeholder: mapping.placeholder }, 'Failed to create mapping');
        throw err;
      }
    },

    getMapping(placeholder: string): Mapping | null {
      return mappings.get(placeholder) || null;
    },

    findByPlaintext(plaintext: string, creator: string): Mapping | null {
      try {
        const indexHash = generateIndex(plaintext, creator, masterKey);
        const placeholder = indexes.get(indexHash);

        if (!placeholder) {
          return null;
        }

        return mappings.get(placeholder) || null;
      } catch (err) {
        logger.error({ err }, 'Failed to find mapping by plaintext');
        return null;
      }
    },

    deleteExpired(): number {
      const now = Date.now();
      let deleted = 0;

      for (const [placeholder, mapping] of mappings.entries()) {
        if (mapping.expiresAt < now) {
          mappings.delete(placeholder);

          // 删除索引
          const indexHash = generateIndex(
            mapping.plaintext,
            mapping.creator,
            masterKey,
          );
          indexes.delete(indexHash);

          deleted++;
        }
      }

      logger.info({ deleted }, 'Expired mappings deleted (memory)');

      return deleted;
    },

    close(): void {
      mappings.clear();
      indexes.clear();
      logger.info('Memory mapping storage closed');
    },
  };
}

/**
 * 定期清理过期映射（后台任务）
 *
 * @param storage - MappingStorage 实例
 * @param intervalMs - 清理间隔（毫秒）
 * @returns 定时器 ID（用于取消）
 */
export function startExpirationCleaner(
  storage: MappingStorage,
  intervalMs = 60 * 60 * 1000, // 默认每小时清理一次
): NodeJS.Timeout {
  const timer = setInterval(() => {
    try {
      storage.deleteExpired();
    } catch (err) {
      logger.error({ err }, 'Expiration cleaner failed');
    }
  }, intervalMs);

  logger.info({ intervalMs }, 'Expiration cleaner started');

  return timer;
}
