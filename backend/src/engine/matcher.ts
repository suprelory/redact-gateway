/**
 * 规则匹配器模块
 *
 * 提供三种匹配方法：
 * - regex: 正则表达式匹配
 * - dict: 字典精确匹配
 * - entropy: 高熵检测匹配
 */

import type { Rule } from '../types.js';
import {
  isHighEntropy,
  findHighEntropySegments,
} from './entropy.js';

export interface Match {
  text: string;       // 匹配到的文本
  start: number;      // 起始位置
  end: number;        // 结束位置
  ruleId: string;     // 命中的规则 ID
  ruleType: string;   // 规则类型
  priority: number;   // 规则优先级
}

/**
 * 正则表达式匹配
 *
 * @param text - 待匹配文本
 * @param rule - 规则对象
 * @returns 匹配结果数组
 */
export function matchRegex(text: string, rule: Rule): Match[] {
  if (!rule.pattern) {
    return [];
  }

  const matches: Match[] = [];
  const regex = new RegExp(rule.pattern, 'g');
  let match: RegExpExecArray | null;

  while ((match = regex.exec(text)) !== null) {
    // 使用 captureGroup 指定的捕获组，默认为 0（整个匹配）
    const groupIndex = rule.captureGroup ?? 0;
    const matchedText = match[groupIndex];

    if (!matchedText) {
      continue;
    }

    // 计算实际位置（如果使用捕获组，需要调整 start）
    const fullMatchStart = match.index;
    const captureStart = fullMatchStart + match[0].indexOf(matchedText);

    matches.push({
      text: matchedText,
      start: captureStart,
      end: captureStart + matchedText.length,
      ruleId: rule.id,
      ruleType: rule.type,
      priority: rule.priority,
    });
  }

  return matches;
}

/**
 * 字典精确匹配
 *
 * 使用 Aho-Corasick 风格的多模式匹配（简化版）
 *
 * @param text - 待匹配文本
 * @param rule - 规则对象
 * @returns 匹配结果数组
 */
export function matchDictionary(text: string, rule: Rule): Match[] {
  if (!rule.dictionary || rule.dictionary.length === 0) {
    return [];
  }

  const matches: Match[] = [];
  const lowerText = text.toLowerCase();

  for (const keyword of rule.dictionary) {
    const lowerKeyword = keyword.toLowerCase();
    let startIndex = 0;

    while (true) {
      const index = lowerText.indexOf(lowerKeyword, startIndex);

      if (index === -1) {
        break;
      }

      // 检查是否为单词边界（避免部分匹配）
      const beforeChar = index > 0 ? text[index - 1] : ' ';
      const afterChar = index + keyword.length < text.length
        ? text[index + keyword.length]
        : ' ';

      const isWordBoundary =
        !/[a-zA-Z0-9_]/.test(beforeChar) &&
        !/[a-zA-Z0-9_]/.test(afterChar);

      if (isWordBoundary) {
        matches.push({
          text: text.slice(index, index + keyword.length),
          start: index,
          end: index + keyword.length,
          ruleId: rule.id,
          ruleType: rule.type,
          priority: rule.priority,
        });
      }

      startIndex = index + 1;
    }
  }

  return matches;
}

/**
 * 高熵检测匹配
 *
 * 使用交叉熵检测随机字符串（API Key、Token 等）
 *
 * @param text - 待匹配文本
 * @param rule - 规则对象
 * @returns 匹配结果数组
 */
export function matchEntropy(text: string, rule: Rule): Match[] {
  const threshold = rule.entropyThreshold;
  const segments = findHighEntropySegments(text, 16, 128);

  return segments
    .filter(seg => {
      // 应用自定义阈值
      if (threshold !== undefined) {
        return seg.entropy > threshold;
      }
      return true;
    })
    .map(seg => ({
      text: seg.text,
      start: seg.start,
      end: seg.end,
      ruleId: rule.id,
      ruleType: rule.type,
      priority: rule.priority,
    }));
}

/**
 * Luhn 算法校验（用于银行卡号）
 *
 * @param cardNumber - 卡号字符串
 * @returns 是否通过校验
 */
export function luhnCheck(cardNumber: string): boolean {
  const digits = cardNumber.replace(/\D/g, '');

  if (digits.length < 13 || digits.length > 19) {
    return false;
  }

  let sum = 0;
  let isEven = false;

  for (let i = digits.length - 1; i >= 0; i--) {
    let digit = parseInt(digits[i], 10);

    if (isEven) {
      digit *= 2;
      if (digit > 9) {
        digit -= 9;
      }
    }

    sum += digit;
    isEven = !isEven;
  }

  return sum % 10 === 0;
}

/**
 * 中国居民身份证号校验
 *
 * 支持 15 位（旧版）和 18 位（新版）
 *
 * @param idCard - 身份证号
 * @returns 是否通过校验
 */
export function validateChineseID(idCard: string): boolean {
  const id = idCard.trim();

  if (id.length === 15) {
    return /^\d{15}$/.test(id);
  }

  if (id.length !== 18) {
    return false;
  }

  if (!/^\d{17}[\dXx]$/.test(id)) {
    return false;
  }

  // 校验位算法
  const weights = [7, 9, 10, 5, 8, 4, 2, 1, 6, 3, 7, 9, 10, 5, 8, 4, 2];
  const checkCodes = ['1', '0', 'X', '9', '8', '7', '6', '5', '4', '3', '2'];

  let sum = 0;
  for (let i = 0; i < 17; i++) {
    sum += parseInt(id[i], 10) * weights[i];
  }

  const expectedCheck = checkCodes[sum % 11];
  const actualCheck = id[17].toUpperCase();

  return expectedCheck === actualCheck;
}

/**
 * 统一匹配入口
 *
 * 根据规则的 method 字段自动选择匹配器
 *
 * @param text - 待匹配文本
 * @param rule - 规则对象
 * @returns 匹配结果数组
 */
export function match(text: string, rule: Rule): Match[] {
  if (!rule.enabled) {
    return [];
  }

  let matches: Match[] = [];

  switch (rule.method) {
    case 'regex':
      matches = matchRegex(text, rule);
      break;
    case 'dict':
      matches = matchDictionary(text, rule);
      break;
    case 'entropy':
      matches = matchEntropy(text, rule);
      break;
    default:
      return [];
  }

  // 应用白名单过滤
  if (rule.allowList && rule.allowList.length > 0) {
    const allowSet = new Set(
      rule.allowList.map(item => item.toLowerCase())
    );

    matches = matches.filter(
      m => !allowSet.has(m.text.toLowerCase())
    );
  }

  // 后处理：特定规则的额外校验
  if (rule.type === 'BANK_CARD') {
    matches = matches.filter(m => luhnCheck(m.text));
  }

  if (rule.type === 'CHINESE_ID') {
    matches = matches.filter(m => validateChineseID(m.text));
  }

  return matches;
}

/**
 * 批量匹配多个规则
 *
 * @param text - 待匹配文本
 * @param rules - 规则数组
 * @returns 所有匹配结果（已按位置排序）
 */
export function matchAll(text: string, rules: Rule[]): Match[] {
  const allMatches: Match[] = [];

  for (const rule of rules) {
    if (rule.enabled) {
      const ruleMatches = match(text, rule);
      allMatches.push(...ruleMatches);
    }
  }

  // 按起始位置排序
  allMatches.sort((a, b) => a.start - b.start);

  return allMatches;
}
