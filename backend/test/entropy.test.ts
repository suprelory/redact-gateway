import { describe, it, expect } from 'vitest';
import { calculateCrossEntropy, calculateShannonEntropy, isHighEntropy } from '../src/engine/entropy.js';

describe('entropy', () => {
  describe('calculateCrossEntropy', () => {
    it('should return low entropy for natural English', () => {
      const entropy = calculateCrossEntropy('hello world');
      expect(entropy).toBeLessThan(4.0);
    });

    it('should return high entropy for random strings', () => {
      const entropy = calculateCrossEntropy('xk7Hs9Qp2Lm4Rn8Wv');
      expect(entropy).toBeGreaterThan(4.5);
    });

    it('should handle API key format', () => {
      const entropy = calculateCrossEntropy('sk_test_4eC39HqLyjWDarjtT1zdp7dc');
      expect(entropy).toBeGreaterThan(4.5);
    });
  });

  describe('calculateShannonEntropy', () => {
    it('should return 0 for empty string', () => {
      expect(calculateShannonEntropy('')).toBe(0);
    });

    it('should return low entropy for repeated characters', () => {
      const entropy = calculateShannonEntropy('aaaaaaaaaa');
      expect(entropy).toBeLessThan(1.0);
    });

    it('should return high entropy for diverse characters', () => {
      const entropy = calculateShannonEntropy('Abc123!@#XyZ');
      expect(entropy).toBeGreaterThan(3.0);
    });
  });

  describe('isHighEntropy', () => {
    it('should detect API keys', () => {
      expect(isHighEntropy('sk_test_4eC39HqLyjWDarjtT1zdp7dc')).toBe(true);
      expect(isHighEntropy('ghp_5xG8mN2kP9qR3vT7wY1zD4hJ6lF0sA8bC')).toBe(true);
    });

    it('should detect JWT tokens', () => {
      const jwt = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c';
      expect(isHighEntropy(jwt)).toBe(true);
    });

    it('should reject natural language', () => {
      expect(isHighEntropy('hello world this is a test')).toBe(false);
      expect(isHighEntropy('the quick brown fox jumps over')).toBe(false);
    });

    it('should reject short strings', () => {
      expect(isHighEntropy('abc123')).toBe(false); // Too short
    });

    it('should reject pure numbers', () => {
      expect(isHighEntropy('123456789012345678')).toBe(false);
    });

    it('should reject pure letters', () => {
      expect(isHighEntropy('abcdefghijklmnop')).toBe(false);
    });

    it('should reject repeated patterns', () => {
      expect(isHighEntropy('aaaaaaaa11111111')).toBe(false); // Low Shannon entropy
    });
  });
});
