/**
 * Creator 身份识别
 */

import { createHash } from 'crypto';

/**
 * 从请求头中提取并计算 Creator 哈希
 * 优先级: x-api-key > Authorization > api-key > anonymous
 */
export function extractCreator(headers: Record<string, string | undefined>): string {
  // 规范化 header 键名（小写）
  const normalized = Object.keys(headers).reduce((acc, key) => {
    acc[key.toLowerCase()] = headers[key];
    return acc;
  }, {} as Record<string, string | undefined>);

  const apiKey =
    normalized['x-api-key'] ||
    normalized['authorization'] ||
    normalized['api-key'];

  if (!apiKey) {
    return 'anonymous';
  }

  // 规范化凭据：移除 Bearer 前缀，去除空白
  const credential = apiKey.replace(/^Bearer\s+/i, '').trim();

  // 返回 SHA-256 哈希
  return createHash('sha256')
    .update(credential)
    .digest('hex');
}

/**
 * 获取 Creator 哈希的前缀（用于日志显示）
 */
export function getCreatorPrefix(creator: string, length: number = 8): string {
  return creator.slice(0, length);
}

/**
 * 验证 Creator 是否匹配
 */
export function verifyCreator(expected: string, actual: string): boolean {
  return expected === actual;
}
