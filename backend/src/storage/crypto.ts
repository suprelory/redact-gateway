/**
 * 加密工具模块
 *
 * 提供：
 * - HKDF 密钥派生
 * - AES-256-GCM 加密/解密
 * - HMAC-SHA256 索引生成
 */

import { randomBytes, createCipheriv, createDecipheriv, createHmac, pbkdf2Sync } from 'node:crypto';

const ALGORITHM = 'aes-256-gcm';
const KEY_LENGTH = 32; // 256 bits
const IV_LENGTH = 12;  // 96 bits (recommended for GCM)
const AUTH_TAG_LENGTH = 16; // 128 bits
const SALT_LENGTH = 32;

/**
 * 使用 PBKDF2 派生密钥
 *
 * 将主密钥派生为不同用途的子密钥：
 * - encryption: 用于 AES-256-GCM 加密
 * - indexing: 用于 HMAC-SHA256 索引生成
 *
 * @param masterKey - 主密钥（至少 32 字节）
 * @param purpose - 用途标识
 * @param salt - 盐值（可选，用于多租户隔离）
 * @returns 派生密钥（32 字节）
 */
export function deriveKey(
  masterKey: string,
  purpose: 'encryption' | 'indexing',
  salt?: Buffer,
): Buffer {
  const info = `redact-gateway.${purpose}`;
  const actualSalt = salt || Buffer.from(info, 'utf8');

  // 使用 PBKDF2 派生密钥（100,000 次迭代）
  return pbkdf2Sync(
    masterKey,
    actualSalt,
    100000,
    KEY_LENGTH,
    'sha256',
  );
}

/**
 * 使用 AES-256-GCM 加密
 *
 * 输出格式：salt(32) || iv(12) || ciphertext || authTag(16)
 *
 * @param plaintext - 明文
 * @param masterKey - 主密钥
 * @returns Base64 编码的密文
 */
export function encrypt(plaintext: string, masterKey: string): string {
  // 生成随机盐和 IV
  const salt = randomBytes(SALT_LENGTH);
  const iv = randomBytes(IV_LENGTH);

  // 派生加密密钥
  const key = deriveKey(masterKey, 'encryption', salt);

  // 加密
  const cipher = createCipheriv(ALGORITHM, key, iv);
  const ciphertext = Buffer.concat([
    cipher.update(plaintext, 'utf8'),
    cipher.final(),
  ]);

  // 获取认证标签
  const authTag = cipher.getAuthTag();

  // 拼接：salt || iv || ciphertext || authTag
  const result = Buffer.concat([salt, iv, ciphertext, authTag]);

  return result.toString('base64');
}

/**
 * 使用 AES-256-GCM 解密
 *
 * @param ciphertext - Base64 编码的密文
 * @param masterKey - 主密钥
 * @returns 明文
 * @throws 如果解密失败（认证标签不匹配）
 */
export function decrypt(ciphertext: string, masterKey: string): string {
  const data = Buffer.from(ciphertext, 'base64');

  // 解析各部分
  const salt = data.subarray(0, SALT_LENGTH);
  const iv = data.subarray(SALT_LENGTH, SALT_LENGTH + IV_LENGTH);
  const authTag = data.subarray(data.length - AUTH_TAG_LENGTH);
  const encrypted = data.subarray(
    SALT_LENGTH + IV_LENGTH,
    data.length - AUTH_TAG_LENGTH,
  );

  // 派生解密密钥
  const key = deriveKey(masterKey, 'encryption', salt);

  // 解密
  const decipher = createDecipheriv(ALGORITHM, key, iv);
  decipher.setAuthTag(authTag);

  const plaintext = Buffer.concat([
    decipher.update(encrypted),
    decipher.final(),
  ]);

  return plaintext.toString('utf8');
}

/**
 * 生成 HMAC-SHA256 索引
 *
 * 用于：
 * - 数据库索引（不泄露明文）
 * - 查找已有映射（相同明文+creator -> 相同索引 -> 复用占位符）
 *
 * @param plaintext - 明文
 * @param creator - 创建者身份
 * @param masterKey - 主密钥
 * @returns 索引值（十六进制字符串）
 */
export function generateIndex(
  plaintext: string,
  creator: string,
  masterKey: string,
): string {
  const indexKey = deriveKey(masterKey, 'indexing');

  // HMAC-SHA256(key, plaintext || creator)
  const hmac = createHmac('sha256', indexKey);
  hmac.update(plaintext);
  hmac.update('\x00'); // 分隔符
  hmac.update(creator);

  return hmac.digest('hex');
}

/**
 * 安全随机字符串生成
 *
 * @param length - 字节长度
 * @returns 十六进制字符串
 */
export function generateRandomHex(length: number): string {
  return randomBytes(length).toString('hex');
}

/**
 * 安全随机字符串生成（Base64）
 *
 * @param length - 字节长度
 * @returns Base64 字符串
 */
export function generateRandomBase64(length: number): string {
  return randomBytes(length).toString('base64url');
}

/**
 * 验证主密钥强度
 *
 * @param masterKey - 主密钥
 * @returns 是否符合安全要求
 */
export function validateMasterKey(masterKey: string): {
  valid: boolean;
  reason?: string;
} {
  if (masterKey.length < 32) {
    return {
      valid: false,
      reason: 'Master key must be at least 32 characters',
    };
  }

  // 检查熵（防止弱密钥如 "aaaaaaaa..."）
  const uniqueChars = new Set(masterKey).size;

  if (uniqueChars < 10) {
    return {
      valid: false,
      reason: 'Master key has insufficient entropy (too few unique characters)',
    };
  }

  return { valid: true };
}
