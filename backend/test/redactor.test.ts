import { describe, it, expect } from 'vitest';
import { redactText, redactObject } from '../src/engine/redactor.js';
import type { Rule } from '../src/types.js';

describe('redactor', () => {
  const rules: Rule[] = [
    {
      id: 'phone',
      type: 'PHONE',
      name: 'Phone',
      enabled: true,
      builtin: true,
      priority: 80,
      method: 'regex',
      pattern: '1[3-9]\\d{9}',
    },
    {
      id: 'email',
      type: 'EMAIL',
      name: 'Email',
      enabled: true,
      builtin: true,
      priority: 70,
      method: 'regex',
      pattern: '[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}',
    },
  ];

  describe('redactText', () => {
    it('should redact sensitive information', () => {
      const text = 'Contact: 13812345678, email: user@example.com';
      const result = redactText(text, rules, { creator: 'test-user' });

      expect(result.redactedText).toContain('{{PHONE_');
      expect(result.redactedText).toContain('{{EMAIL_');
      expect(result.redactedText).not.toContain('13812345678');
      expect(result.redactedText).not.toContain('user@example.com');
      expect(result.matchCount).toBe(2);
      expect(result.redactedCount).toBe(2);
      expect(result.mappings).toHaveLength(2);
    });

    it('should reuse placeholders for same plaintext', () => {
      const text = 'Phone: 13812345678, again: 13812345678';
      const result = redactText(text, rules, { creator: 'test-user' });

      // Should generate only 1 mapping for repeated plaintext
      expect(result.mappings).toHaveLength(1);
      expect(result.redactedCount).toBe(2); // But redact both occurrences

      // Extract placeholders
      const placeholders = result.redactedText.match(/\{\{PHONE_[A-Z0-9]+\}\}/g);
      expect(placeholders).toHaveLength(2);
      expect(placeholders![0]).toBe(placeholders![1]); // Same placeholder
    });

    it('should handle overlapping matches by priority', () => {
      const highPriorityRule: Rule = {
        id: 'high',
        type: 'HIGH',
        name: 'High Priority',
        enabled: true,
        builtin: false,
        priority: 100,
        method: 'regex',
        pattern: 'secret123',
      };

      const lowPriorityRule: Rule = {
        id: 'low',
        type: 'LOW',
        name: 'Low Priority',
        enabled: true,
        builtin: false,
        priority: 50,
        method: 'regex',
        pattern: 'secret',
      };

      const text = 'Password: secret123';
      const result = redactText(text, [highPriorityRule, lowPriorityRule], {
        creator: 'test-user',
      });

      // High priority rule should win
      expect(result.redactedText).toContain('{{HIGH_');
      expect(result.redactedText).not.toContain('{{LOW_');
      expect(result.redactedCount).toBe(1);
    });

    it('should support dry run mode', () => {
      const text = 'Phone: 13812345678';
      const result = redactText(text, rules, {
        creator: 'test-user',
        dryRun: true,
      });

      expect(result.redactedText).toContain('{{PHONE_');
      expect(result.mappings).toHaveLength(0); // No mappings in dry run
    });
  });

  describe('redactObject', () => {
    it('should redact specified fields in object', () => {
      const obj = {
        user: {
          name: 'John',
          phone: '13812345678',
          email: 'john@example.com',
        },
        message: 'Contact me at 18900001111',
      };

      const result = redactObject(
        obj,
        ['user.phone', 'user.email', 'message'],
        rules,
        { creator: 'test-user' },
      );

      expect(result.redactedObj.user.name).toBe('John'); // Unchanged
      expect(result.redactedObj.user.phone).toContain('{{PHONE_');
      expect(result.redactedObj.user.email).toContain('{{EMAIL_');
      expect(result.redactedObj.message).toContain('{{PHONE_');
      expect(result.mappings.length).toBeGreaterThan(0);
    });

    it('should handle missing fields gracefully', () => {
      const obj = { a: 'test' };

      const result = redactObject(
        obj,
        ['nonexistent.field'],
        rules,
        { creator: 'test-user' },
      );

      expect(result.redactedObj).toEqual({ a: 'test' });
      expect(result.mappings).toHaveLength(0);
    });
  });
});
