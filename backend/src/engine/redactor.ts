/**
 * 脱敏引擎核心模块
 *
 * 负责：
 * 1. 扫描文本中的敏感信息
 * 2. 生成占位符并替换
 * 3. 处理重叠匹配（优先级）
 * 4. 生成映射表
 */

import type { Rule, Mapping } from '../types.js';
import { matchAll, type Match } from './matcher.js';
import { generatePlaceholder } from './placeholder.js';
import { extractCreator } from './creator.js';
import { logger } from '../logger.js';

export interface RedactResult {
  redactedText: string;           // 脱敏后的文本
  mappings: Mapping[];            // 新生成的映射表
  matchCount: number;             // 总匹配数
  redactedCount: number;          // 实际替换数（去重后）
  ruleHits: Record<string, number>; // 各规则命中次数
}

export interface RedactOptions {
  creator?: string;               // 创建者身份（默认从 headers 提取）
  ttl?: number;                   // 映射有效期（秒），默认 7 天
  existingMappings?: Map<string, string>; // 已有映射（用于复用占位符）
  dryRun?: boolean;               // 试运行模式（不生成映射）
}

/**
 * 解决重叠匹配冲突
 *
 * 策略：优先级高的规则优先，相同优先级取最长匹配
 *
 * @param matches - 所有匹配结果
 * @returns 去重后的匹配结果
 */
function resolveOverlaps(matches: Match[]): Match[] {
  if (matches.length === 0) {
    return [];
  }

  // 按优先级降序、起始位置升序排序
  const sorted = [...matches].sort((a, b) => {
    if (a.priority !== b.priority) {
      return b.priority - a.priority; // 高优先级在前
    }
    if (a.start !== b.start) {
      return a.start - b.start;
    }
    return b.end - a.end; // 相同起点取最长
  });

  const resolved: Match[] = [];
  const occupied = new Set<number>();

  for (const match of sorted) {
    // 检查是否与已选匹配重叠
    let hasOverlap = false;

    for (let i = match.start; i < match.end; i++) {
      if (occupied.has(i)) {
        hasOverlap = true;
        break;
      }
    }

    if (!hasOverlap) {
      resolved.push(match);

      // 标记已占用位置
      for (let i = match.start; i < match.end; i++) {
        occupied.add(i);
      }
    }
  }

  // 按位置重新排序（便于后续替换）
  resolved.sort((a, b) => a.start - b.start);

  return resolved;
}

/**
 * 扫描文本并应用脱敏规则
 *
 * @param text - 原始文本
 * @param rules - 规则集
 * @param options - 选项
 * @returns 脱敏结果
 */
export function redactText(
  text: string,
  rules: Rule[],
  options: RedactOptions = {},
): RedactResult {
  const startTime = Date.now();

  // 1. 匹配所有规则
  const allMatches = matchAll(text, rules);

  logger.debug({
    totalMatches: allMatches.length,
    rules: rules.filter(r => r.enabled).map(r => r.id),
  }, 'Matched all rules');

  // 2. 解决重叠冲突
  const resolvedMatches = resolveOverlaps(allMatches);

  logger.debug({
    beforeResolve: allMatches.length,
    afterResolve: resolvedMatches.length,
  }, 'Resolved overlapping matches');

  // 3. 生成映射表（复用已有占位符）
  const existingMappings = options.existingMappings || new Map();
  const newMappings: Mapping[] = [];
  const creator = options.creator || 'anonymous';
  const ttl = options.ttl || 7 * 24 * 60 * 60; // 默认 7 天
  const now = Date.now();

  const replacements = new Map<Match, string>();

  for (const match of resolvedMatches) {
    const plaintext = match.text;
    const key = `${creator}:${plaintext}`;

    // 尝试复用已有占位符
    let placeholder = existingMappings.get(key);

    if (!placeholder) {
      // 生成新占位符
      placeholder = generatePlaceholder(match.ruleType);

      if (!options.dryRun) {
        newMappings.push({
          placeholder,
          plaintext,
          ruleType: match.ruleType,
          creator,
          createdAt: now,
          expiresAt: now + ttl * 1000,
        });

        // 缓存到 existingMappings（同一文本内复用）
        existingMappings.set(key, placeholder);
      }
    }

    replacements.set(match, placeholder);
  }

  // 4. 执行替换（从后往前，避免索引失效）
  let redactedText = text;
  const reversedMatches = [...resolvedMatches].reverse();

  for (const match of reversedMatches) {
    const placeholder = replacements.get(match)!;
    redactedText =
      redactedText.slice(0, match.start) +
      placeholder +
      redactedText.slice(match.end);
  }

  // 5. 统计规则命中情况
  const ruleHits: Record<string, number> = {};

  for (const match of resolvedMatches) {
    ruleHits[match.ruleId] = (ruleHits[match.ruleId] || 0) + 1;
  }

  const duration = Date.now() - startTime;

  logger.info({
    matchCount: allMatches.length,
    redactedCount: resolvedMatches.length,
    newMappings: newMappings.length,
    ruleHits,
    duration,
  }, 'Redaction completed');

  return {
    redactedText,
    mappings: newMappings,
    matchCount: allMatches.length,
    redactedCount: resolvedMatches.length,
    ruleHits,
  };
}

/**
 * 便捷函数：从 HTTP headers 提取 creator 并执行脱敏
 *
 * @param text - 原始文本
 * @param rules - 规则集
 * @param headers - HTTP 请求头
 * @param options - 额外选项
 * @returns 脱敏结果
 */
export function redactWithHeaders(
  text: string,
  rules: Rule[],
  headers: Record<string, string | undefined>,
  options: Omit<RedactOptions, 'creator'> = {},
): RedactResult {
  const creator = extractCreator(headers);

  return redactText(text, rules, {
    ...options,
    creator,
  });
}

/**
 * 批量脱敏（用于 JSON 对象的多个字段）
 *
 * @param obj - 待脱敏对象
 * @param fields - 需要脱敏的字段路径（如 ['messages.0.content', 'prompt']）
 * @param rules - 规则集
 * @param options - 选项
 * @returns 脱敏后的对象和映射表
 */
export function redactObject(
  obj: any,
  fields: string[],
  rules: Rule[],
  options: RedactOptions = {},
): { redactedObj: any; mappings: Mapping[] } {
  const cloned = JSON.parse(JSON.stringify(obj));
  const allMappings: Mapping[] = [];
  const existingMappings = options.existingMappings || new Map();

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

    // 脱敏目标字段
    if (parent && lastKey && typeof parent[lastKey] === 'string') {
      const result = redactText(parent[lastKey], rules, {
        ...options,
        existingMappings,
      });

      parent[lastKey] = result.redactedText;
      allMappings.push(...result.mappings);
    }
  }

  return {
    redactedObj: cloned,
    mappings: allMappings,
  };
}
