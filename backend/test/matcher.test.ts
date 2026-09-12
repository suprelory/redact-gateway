import { describe, it, expect } from 'vitest';
import { matchRegex, matchDictionary, matchEntropy, luhnCheck, validateChineseID } from '../src/engine/matcher.js';
import type { Rule } from '../src/types.js';

describe('matcher', () => {
  describe('matchRegex', () => {
    it('should match phone numbers', () => {
      const rule: Rule = {
        id: 'phone',
        type: 'PHONE',
        name: 'Phone Number',
        enabled: true,
        builtin: true,
        priority: 80,
        method: 'regex',
        pattern: '1[3-9]\\d{9}',
      };

      const text = 'Call me at 13812345678 or 18900001111';
      const matches = matchRegex(text, rule);

      expect(matches).toHaveLength(2);
      expect(matches[0].text).toBe('13812345678');
      expect(matches[1].text).toBe('18900001111');
    });

    it('should use capture group when specified', () => {
      const rule: Rule = {
        id: 'email',
        type: 'EMAIL',
        name: 'Email',
        enabled: true,
        builtin: true,
        priority: 70,
        method: 'regex',
        pattern: '([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,})',
        captureGroup: 1,
      };

      const text = 'Contact: user@example.com';
      const matches = matchRegex(text, rule);

      expect(matches).toHaveLength(1);
      expect(matches[0].text).toBe('user@example.com');
    });
  });

  describe('matchDictionary', () => {
    it('should match keywords with word boundaries', () => {
      const rule: Rule = {
        id: 'company',
        type: 'COMPANY',
        name: 'Company Names',
        enabled: true,
        builtin: false,
        priority: 60,
        method: 'dict',
        dictionary: ['Microsoft', 'Google', 'Apple'],
      };

      const text = 'Microsoft and Google are tech giants. Pineapple is fruit.';
      const matches = matchDictionary(text, rule);

      expect(matches).toHaveLength(2);
      expect(matches[0].text).toBe('Microsoft');
      expect(matches[1].text).toBe('Google');
      // "Pineapple" should NOT match "Apple" (word boundary check)
    });

    it('should be case-insensitive', () => {
      const rule: Rule = {
        id: 'keyword',
        type: 'KEYWORD',
        name: 'Keywords',
        enabled: true,
        builtin: false,
        priority: 50,
        method: 'dict',
        dictionary: ['secret'],
      };

      const text = 'This is SECRET information';
      const matches = matchDictionary(text, rule);

      expect(matches).toHaveLength(1);
      expect(matches[0].text).toBe('SECRET');
    });
  });

  describe('luhnCheck', () => {
    it('should validate correct card numbers', () => {
      expect(luhnCheck('4532015112830366')).toBe(true); // Visa
      expect(luhnCheck('5425233430109903')).toBe(true); // Mastercard
      expect(luhnCheck('6011000991300009')).toBe(true); // Discover
    });

    it('should reject invalid card numbers', () => {
      expect(luhnCheck('4532015112830367')).toBe(false); // Wrong checksum
      expect(luhnCheck('1234567890123456')).toBe(false); // Invalid
      expect(luhnCheck('123')).toBe(false); // Too short
    });
  });

  describe('validateChineseID', () => {
    it('should validate 18-digit ID cards', () => {
      expect(validateChineseID('11010519491231002X')).toBe(true);
      expect(validateChineseID('440524188001010014')).toBe(true);
    });

    it('should accept 15-digit old format', () => {
      expect(validateChineseID('110105491231002')).toBe(true);
    });

    it('should reject invalid ID cards', () => {
      expect(validateChineseID('110105194912310021')).toBe(false); // Wrong checksum
      expect(validateChineseID('12345678901234567')).toBe(false); // Invalid format
      expect(validateChineseID('123')).toBe(false); // Too short
    });
  });
});
