/**
 * 映射表存储模块（SQLite 版本）
 *
 * 注意：需要安装 better-sqlite3 依赖
 * 开发环境可使用 memory.ts 的内存版本
 *
 * 使用 SQLite + 加密存储占位符 <-> 明文映射
 *
 * 表结构:
 * - mappings: 主表（加密存储）
 * - indexes: 索引表（HMAC-SHA256）
 */

import Database from 'better-sqlite3';
import { resolve } from 'node:path';
import { mkdirSync, existsSync } from 'node:fs';
import type { Mapping } from '../types.js';
import { encrypt, decrypt, generateIndex } from './crypto.js';
import { logger } from '../logger.js';

export interface MappingStorage {
  createMapping(mapping: Mapping): void;
  getMapping(placeholder: string): Mapping | null;
  findByPlaintext(plaintext: string, creator: string): Mapping | null;
  deleteExpired(): number;
  close(): void;
}

/**
 * 创建 SQLite 映射表存储
 *
 * @param dbPath - 数据库文件路径
 * @param masterKey - 主密钥（用于加密）
 * @returns MappingStorage 实例
 */
export function createMappingStorage(
  dbPath: string,
  masterKey: string,
): MappingStorage {
  // 确保目录存在
  const dbDir = resolve(dbPath, '..');
  if (!existsSync(dbDir)) {
    mkdirSync(dbDir, { recursive: true });
  }

  const db = new Database(dbPath);

  // 启用 WAL 模式（提升并发性能）
  db.pragma('journal_mode = WAL');

  // 初始化表结构
  initSchema(db);

  logger.info({ dbPath }, 'Mapping storage initialized');

  // 预编译 SQL 语句
  const insertStmt = db.prepare(`
    INSERT INTO mappings (placeholder, encrypted_plaintext, rule_type, creator, created_at, expires_at)
    VALUES (?, ?, ?, ?, ?, ?)
  `);

  const insertIndexStmt = db.prepare(`
    INSERT OR IGNORE INTO indexes (index_hash, placeholder)
    VALUES (?, ?)
  `);

  const selectByPlaceholderStmt = db.prepare(`
    SELECT * FROM mappings WHERE placeholder = ?
  `);

  const selectByIndexStmt = db.prepare(`
    SELECT m.* FROM mappings m
    JOIN indexes i ON i.placeholder = m.placeholder
    WHERE i.index_hash = ?
    LIMIT 1
  `);

  const deleteExpiredStmt = db.prepare(`
    DELETE FROM mappings WHERE expires_at < ?
  `);

  return {
    createMapping(mapping: Mapping): void {
      try {
        // 加密明文
        const encryptedPlaintext = encrypt(mapping.plaintext, masterKey);

        // 生成索引
        const indexHash = generateIndex(
          mapping.plaintext,
          mapping.creator,
          masterKey,
        );

        // 事务插入
        const transaction = db.transaction(() => {
          insertStmt.run(
            mapping.placeholder,
            encryptedPlaintext,
            mapping.ruleType,
            mapping.creator,
            mapping.createdAt,
            mapping.expiresAt,
          );

          insertIndexStmt.run(indexHash, mapping.placeholder);
        });

        transaction();

        logger.debug({
          placeholder: mapping.placeholder,
          ruleType: mapping.ruleType,
          creator: mapping.creator,
        }, 'Mapping created');
      } catch (err) {
        logger.error({ err, placeholder: mapping.placeholder }, 'Failed to create mapping');
        throw err;
      }
    },

    getMapping(placeholder: string): Mapping | null {
      try {
        const row = selectByPlaceholderStmt.get(placeholder) as any;

        if (!row) {
          return null;
        }

        // 解密明文
        const plaintext = decrypt(row.encrypted_plaintext, masterKey);

        return {
          placeholder: row.placeholder,
          plaintext,
          ruleType: row.rule_type,
          creator: row.creator,
          createdAt: row.created_at,
          expiresAt: row.expires_at,
        };
      } catch (err) {
        logger.error({ err, placeholder }, 'Failed to get mapping');
        return null;
      }
    },

    findByPlaintext(plaintext: string, creator: string): Mapping | null {
      try {
        // 生成索引
        const indexHash = generateIndex(plaintext, creator, masterKey);

        const row = selectByIndexStmt.get(indexHash) as any;

        if (!row) {
          return null;
        }

        // 解密明文
        const decryptedPlaintext = decrypt(row.encrypted_plaintext, masterKey);

        return {
          placeholder: row.placeholder,
          plaintext: decryptedPlaintext,
          ruleType: row.rule_type,
          creator: row.creator,
          createdAt: row.created_at,
          expiresAt: row.expires_at,
        };
      } catch (err) {
        logger.error({ err }, 'Failed to find mapping by plaintext');
        return null;
      }
    },

    deleteExpired(): number {
      try {
        const now = Date.now();
        const result = deleteExpiredStmt.run(now);

        logger.info({ deleted: result.changes }, 'Expired mappings deleted');

        return result.changes;
      } catch (err) {
        logger.error({ err }, 'Failed to delete expired mappings');
        return 0;
      }
    },

    close(): void {
      db.close();
      logger.info('Mapping storage closed');
    },
  };
}

/**
 * 初始化数据库表结构
 */
function initSchema(db: Database.Database): void {
  db.exec(`
    CREATE TABLE IF NOT EXISTS mappings (
      placeholder TEXT PRIMARY KEY,
      encrypted_plaintext TEXT NOT NULL,
      rule_type TEXT NOT NULL,
      creator TEXT NOT NULL,
      created_at INTEGER NOT NULL,
      expires_at INTEGER NOT NULL
    );

    CREATE INDEX IF NOT EXISTS idx_mappings_creator ON mappings(creator);
    CREATE INDEX IF NOT EXISTS idx_mappings_expires_at ON mappings(expires_at);

    CREATE TABLE IF NOT EXISTS indexes (
      index_hash TEXT PRIMARY KEY,
      placeholder TEXT NOT NULL,
      FOREIGN KEY (placeholder) REFERENCES mappings(placeholder) ON DELETE CASCADE
    );

    CREATE INDEX IF NOT EXISTS idx_indexes_placeholder ON indexes(placeholder);
  `);
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
