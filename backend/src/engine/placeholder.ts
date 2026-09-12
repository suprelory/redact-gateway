/**
 * 占位符生成和验证
 */

import { ulid } from 'ulid';

const TOKEN_PREFIX = '{{';
const TOKEN_SUFFIX = '}}';
const TOKEN_SEPARATOR = '_';

/**
 * 生成占位符
 * 格式: {{TYPE_ULID}}
 * 例如: {{PHONE_01ARZ3NDEKTSV4RRFFQ69G5FAV}}
 */
export function generatePlaceholder(ruleType: string): string {
  const id = ulid();
  return `${TOKEN_PREFIX}${ruleType}${TOKEN_SEPARATOR}${id}${TOKEN_SUFFIX}`;
}

/**
 * 验证占位符格式
 */
export function isValidPlaceholder(text: string): boolean {
  return PLACEHOLDER_PATTERN.test(text);
}

/**
 * 提取占位符中的规则类型
 */
export function extractRuleType(placeholder: string): string | null {
  const match = placeholder.match(PLACEHOLDER_PATTERN);
  if (!match) return null;
  return match[1];
}

/**
 * 从文本中提取所有占位符
 */
export function extractPlaceholders(text: string): string[] {
  const matches = text.match(PLACEHOLDER_PATTERN_GLOBAL);
  return matches || [];
}

/**
 * 占位符正则模式（单个匹配）
 * 匹配: {{TYPE_01ARZ3NDEKTSV4RRFFQ69G5FAV}}
 */
export const PLACEHOLDER_PATTERN = /\{\{([A-Z_]+)_([0-9A-HJKMNP-TV-Z]{26})\}\}/;

/**
 * 占位符正则模式（全局匹配）
 */
export const PLACEHOLDER_PATTERN_GLOBAL = /\{\{[A-Z_]+_[0-9A-HJKMNP-TV-Z]{26}\}\}/g;

/**
 * 占位符最大长度
 * {{TYPE_01ARZ3NDEKTSV4RRFFQ69G5FAV}} = 2 + 最长类型名 + 1 + 26 + 2
 * 假设类型名最长 30 字符，总长度约 61 字符
 * 为安全起见，设置为 80
 */
export const MAX_PLACEHOLDER_LENGTH = 80;
