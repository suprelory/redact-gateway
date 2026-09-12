/**
 * 熵检测模块
 *
 * 使用交叉熵（Cross-Entropy）检测高熵字符串（如 API Key、Token）
 * 基于英文字母二元组频率分布
 */

/**
 * 英文字母二元组频率表（基于大规模语料统计）
 *
 * 来源：Google Books Ngrams 和 Brown Corpus
 * 已归一化为概率分布（总和为 1）
 */
const BIGRAM_FREQ: Record<string, number> = {
  th: 0.0356, he: 0.0307, in: 0.0243, er: 0.0205, an: 0.0199,
  re: 0.0185, on: 0.0176, at: 0.0149, en: 0.0145, nd: 0.0135,
  ti: 0.0134, es: 0.0134, or: 0.0128, te: 0.0120, of: 0.0117,
  ed: 0.0117, is: 0.0113, it: 0.0112, al: 0.0109, ar: 0.0107,
  st: 0.0105, to: 0.0104, nt: 0.0104, ng: 0.0095, se: 0.0093,
  ha: 0.0093, as: 0.0087, ou: 0.0087, io: 0.0083, le: 0.0083,
  ve: 0.0083, co: 0.0079, me: 0.0079, de: 0.0076, hi: 0.0076,
  ri: 0.0073, ro: 0.0073, ic: 0.0070, ne: 0.0069, ea: 0.0069,
  ra: 0.0069, ce: 0.0065, li: 0.0062, ch: 0.0060, ll: 0.0058,
  be: 0.0058, ma: 0.0057, si: 0.0055, om: 0.0055, ur: 0.0054,
};

const DEFAULT_BIGRAM_PROB = 0.0001; // 未知二元组的默认概率

/**
 * 计算字符串的交叉熵（Cross-Entropy）
 *
 * 交叉熵衡量字符串与自然语言的偏离程度：
 * - 自然英文单词：低交叉熵（< 3.5）
 * - 随机字符串：高交叉熵（> 4.5）
 *
 * 公式：H(p, q) = -Σ p(x) log₂ q(x)
 * 其中 p 是实际分布，q 是语料库分布
 *
 * @param text - 待检测文本
 * @returns 交叉熵值（bits per bigram）
 */
export function calculateCrossEntropy(text: string): number {
  if (text.length < 2) {
    return 0;
  }

  const normalized = text.toLowerCase().replace(/[^a-z]/g, '');

  if (normalized.length < 2) {
    return 0;
  }

  let entropy = 0;
  let bigramCount = 0;

  for (let i = 0; i < normalized.length - 1; i++) {
    const bigram = normalized.slice(i, i + 2);
    const prob = BIGRAM_FREQ[bigram] || DEFAULT_BIGRAM_PROB;

    // 使用 log2 计算交叉熵
    entropy -= Math.log2(prob);
    bigramCount++;
  }

  return bigramCount > 0 ? entropy / bigramCount : 0;
}

/**
 * 计算 Shannon 熵（字符级熵）
 *
 * 用于辅助检测：
 * - 高 Shannon 熵 + 高交叉熵 = 高置信度随机字符串
 * - 低 Shannon 熵 = 重复字符（aaaaaa），不是 API Key
 *
 * @param text - 待检测文本
 * @returns Shannon 熵值（bits per character）
 */
export function calculateShannonEntropy(text: string): number {
  if (text.length === 0) {
    return 0;
  }

  const freq = new Map<string, number>();

  for (const char of text) {
    freq.set(char, (freq.get(char) || 0) + 1);
  }

  let entropy = 0;
  const len = text.length;

  for (const count of freq.values()) {
    const p = count / len;
    entropy -= p * Math.log2(p);
  }

  return entropy;
}

/**
 * 长度感知阈值函数
 *
 * 短字符串更容易出现"假阳性"，需要更高的阈值
 *
 * @param length - 字符串长度
 * @returns 交叉熵阈值
 */
export function getCrossEntropyThreshold(length: number): number {
  if (length < 8) {
    return 5.5; // 极短字符串：严格阈值
  } else if (length < 16) {
    return 5.0; // 短字符串
  } else if (length < 32) {
    return 4.5; // 中等长度
  } else {
    return 4.2; // 长字符串：宽松阈值（更可能是 Key）
  }
}

/**
 * 判断字符串是否为高熵字符串
 *
 * 综合判断条件：
 * 1. 交叉熵超过长度感知阈值
 * 2. Shannon 熵 > 3.5（排除 "aaaaaaa" 类重复）
 * 3. 长度 >= 8（排除短随机片段）
 * 4. 包含字母数字混合（排除纯数字/纯字母）
 *
 * @param text - 待检测文本
 * @param customThreshold - 自定义阈值（可选）
 * @returns 是否为高熵字符串
 */
export function isHighEntropy(
  text: string,
  customThreshold?: number,
): boolean {
  if (text.length < 8) {
    return false;
  }

  // 检查字符类型多样性
  const hasLetter = /[a-zA-Z]/.test(text);
  const hasDigit = /\d/.test(text);

  if (!hasLetter || !hasDigit) {
    return false; // 必须包含字母和数字
  }

  // 计算 Shannon 熵（排除低熵重复字符串）
  const shannon = calculateShannonEntropy(text);
  if (shannon < 3.5) {
    return false;
  }

  // 计算交叉熵
  const crossEntropy = calculateCrossEntropy(text);
  const threshold = customThreshold ?? getCrossEntropyThreshold(text.length);

  return crossEntropy > threshold;
}

/**
 * 批量检测文本中的高熵片段
 *
 * 使用滑动窗口扫描文本，找出所有高熵子串
 *
 * @param text - 待扫描文本
 * @param minLength - 最小片段长度
 * @param maxLength - 最大片段长度
 * @returns 高熵片段数组 [{ text, start, end, entropy }]
 */
export function findHighEntropySegments(
  text: string,
  minLength = 16,
  maxLength = 128,
): Array<{ text: string; start: number; end: number; entropy: number }> {
  const segments: Array<{
    text: string;
    start: number;
    end: number;
    entropy: number;
  }> = [];

  // 按空白符分割（避免跨单词检测）
  const tokens = text.split(/\s+/);
  let currentIndex = 0;

  for (const token of tokens) {
    const tokenStart = text.indexOf(token, currentIndex);

    if (token.length >= minLength && token.length <= maxLength) {
      if (isHighEntropy(token)) {
        const entropy = calculateCrossEntropy(token);
        segments.push({
          text: token,
          start: tokenStart,
          end: tokenStart + token.length,
          entropy,
        });
      }
    }

    currentIndex = tokenStart + token.length;
  }

  return segments;
}
