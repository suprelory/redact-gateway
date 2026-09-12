/**
 * 还原引擎核心模块
 *
 * 负责：
 * 1. 将占位符还原为原始明文
 * 2. 流式还原（SSE 场景）
 * 3. 跨块占位符拼接
 * 4. Creator 授权验证
 */

import type { Mapping } from '../types.js';
import {
  PLACEHOLDER_PATTERN_GLOBAL,
  extractPlaceholders,
  MAX_PLACEHOLDER_LENGTH,
} from './placeholder.js';
import { logger } from '../logger.js';

export interface RestoreResult {
  restoredText: string;           // 还原后的文本
  restoreCount: number;           // 还原的占位符数量
  skippedCount: number;           // 跳过的占位符数量（未授权或不存在）
}

/**
 * 还原文本中的占位符
 *
 * @param text - 包含占位符的文本
 * @param mappings - 映射表（placeholder -> plaintext）
 * @param creator - 当前请求的 creator（用于权限验证）
 * @returns 还原结果
 */
export function restoreText(
  text: string,
  mappings: Map<string, Mapping>,
  creator: string,
): RestoreResult {
  let restoreCount = 0;
  let skippedCount = 0;

  const restoredText = text.replace(
    PLACEHOLDER_PATTERN_GLOBAL,
    (placeholder) => {
      const mapping = mappings.get(placeholder);

      // 映射不存在
      if (!mapping) {
        logger.warn({ placeholder }, 'Mapping not found for placeholder');
        skippedCount++;
        return placeholder;
      }

      // Creator 权限验证
      if (mapping.creator !== creator && mapping.creator !== 'anonymous') {
        logger.warn({
          placeholder,
          expectedCreator: mapping.creator,
          actualCreator: creator,
        }, 'Creator mismatch, skipping restore');
        skippedCount++;
        return placeholder;
      }

      // 检查过期时间
      if (Date.now() > mapping.expiresAt) {
        logger.warn({
          placeholder,
          expiresAt: new Date(mapping.expiresAt).toISOString(),
        }, 'Mapping expired, skipping restore');
        skippedCount++;
        return placeholder;
      }

      restoreCount++;
      return mapping.plaintext;
    },
  );

  logger.debug({
    restoreCount,
    skippedCount,
  }, 'Text restoration completed');

  return {
    restoredText,
    restoreCount,
    skippedCount,
  };
}

/**
 * 流式还原器（用于 SSE 场景）
 *
 * 使用滑动窗口处理跨块切分的占位符：
 *
 * 输入块 1: "Key is {{PHONE_01ARZ"
 * 输入块 2: "3NDEKTSV4RRFFQ69G5FAV}}"
 *
 * 正确输出: "Key is 13812345678"
 */
export class SlidingWindowRestorer {
  private buffer: string = '';
  private mappings: Map<string, Mapping>;
  private creator: string;
  private totalRestored = 0;
  private totalSkipped = 0;

  constructor(mappings: Map<string, Mapping>, creator: string) {
    this.mappings = mappings;
    this.creator = creator;
  }

  /**
   * 处理一个数据块
   *
   * @param chunk - 输入数据块
   * @returns 可以安全输出的部分（已还原）
   */
  process(chunk: string): string {
    // 1. 将新数据追加到缓冲区
    this.buffer += chunk;

    // 2. 查找完整的占位符并还原
    let output = '';
    let lastSafeIndex = 0;

    // 使用正则查找所有完整占位符
    const regex = new RegExp(PLACEHOLDER_PATTERN_GLOBAL.source, 'g');
    let match: RegExpExecArray | null;

    while ((match = regex.exec(this.buffer)) !== null) {
      const placeholder = match[0];
      const matchStart = match.index;
      const matchEnd = matchStart + placeholder.length;

      // 输出占位符之前的内容
      output += this.buffer.slice(lastSafeIndex, matchStart);

      // 尝试还原占位符
      const mapping = this.mappings.get(placeholder);

      if (
        mapping &&
        mapping.creator === this.creator &&
        Date.now() <= mapping.expiresAt
      ) {
        output += mapping.plaintext;
        this.totalRestored++;
      } else {
        output += placeholder;
        this.totalSkipped++;
      }

      lastSafeIndex = matchEnd;
    }

    // 3. 检查缓冲区尾部是否有不完整的占位符
    const remainingBuffer = this.buffer.slice(lastSafeIndex);
    const potentialPlaceholderStart = remainingBuffer.lastIndexOf('{{');

    if (potentialPlaceholderStart !== -1) {
      // 发现潜在的不完整占位符
      const potentialPlaceholder = remainingBuffer.slice(
        potentialPlaceholderStart,
      );

      // 如果长度已超过最大占位符长度，说明不是占位符
      if (potentialPlaceholder.length >= MAX_PLACEHOLDER_LENGTH) {
        output += remainingBuffer;
        this.buffer = '';
      } else {
        // 保留在缓冲区，等待下一块
        output += remainingBuffer.slice(0, potentialPlaceholderStart);
        this.buffer = potentialPlaceholder;
      }
    } else {
      // 没有不完整占位符，全部输出
      output += remainingBuffer;
      this.buffer = '';
    }

    return output;
  }

  /**
   * 刷新缓冲区（流结束时调用）
   *
   * @returns 缓冲区中剩余的内容
   */
  flush(): string {
    const remaining = this.buffer;
    this.buffer = '';

    logger.debug({
      totalRestored: this.totalRestored,
      totalSkipped: this.totalSkipped,
    }, 'Stream restoration completed');

    return remaining;
  }

  /**
   * 获取统计信息
   */
  getStats() {
    return {
      totalRestored: this.totalRestored,
      totalSkipped: this.totalSkipped,
      bufferSize: this.buffer.length,
    };
  }
}

/**
 * 便捷函数：从映射数组创建 Map
 *
 * @param mappings - Mapping 数组
 * @returns Map<placeholder, Mapping>
 */
export function createMappingMap(mappings: Mapping[]): Map<string, Mapping> {
  const map = new Map<string, Mapping>();

  for (const mapping of mappings) {
    map.set(mapping.placeholder, mapping);
  }

  return map;
}

/**
 * 便捷函数：批量还原多个字段
 *
 * @param obj - 包含占位符的对象
 * @param fields - 需要还原的字段路径
 * @param mappings - 映射表
 * @param creator - 当前 creator
 * @returns 还原后的对象
 */
export function restoreObject(
  obj: any,
  fields: string[],
  mappings: Map<string, Mapping>,
  creator: string,
): any {
  const cloned = JSON.parse(JSON.stringify(obj));

  for (const fieldPath of fields) {
    const keys = fieldPath.split('.');
    let current: any = cloned;
    let parent: any = null;
    let lastKey: string = '';

    // 遍历路径
    for (let i = 0; i < keys.length; i++) {
      const key = keys[i];

      if (i === keys.length - 1) {
        parent = current;
        lastKey = key;
      } else {
        if (current[key] === undefined) {
          break;
        }
        current = current[key];
      }
    }

    // 还原目标字段
    if (parent && lastKey && typeof parent[lastKey] === 'string') {
      const result = restoreText(parent[lastKey], mappings, creator);
      parent[lastKey] = result.restoredText;
    }
  }

  return cloned;
}
