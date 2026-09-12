import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { restoreText, SlidingWindowRestorer, createMappingMap } from '../src/engine/restorer.js';
import type { Mapping } from '../src/types.js';

describe('restorer', () => {
  const now = Date.now();
  const creator = 'test-user';

  const mappings: Mapping[] = [
    {
      placeholder: '{{PHONE_01ARZ3NDEKTSV4RRFFQ69G5FAV}}',
      plaintext: '13812345678',
      ruleType: 'PHONE',
      creator,
      createdAt: now,
      expiresAt: now + 7 * 24 * 60 * 60 * 1000,
    },
    {
      placeholder: '{{EMAIL_01ARZ3NDEKTSV4RRFFQ69G5XYZ}}',
      plaintext: 'user@example.com',
      ruleType: 'EMAIL',
      creator,
      createdAt: now,
      expiresAt: now + 7 * 24 * 60 * 60 * 1000,
    },
  ];

  const mappingMap = createMappingMap(mappings);

  describe('restoreText', () => {
    it('should restore placeholders', () => {
      const text = 'Contact: {{PHONE_01ARZ3NDEKTSV4RRFFQ69G5FAV}}, email: {{EMAIL_01ARZ3NDEKTSV4RRFFQ69G5XYZ}}';
      const result = restoreText(text, mappingMap, creator);

      expect(result.restoredText).toBe('Contact: 13812345678, email: user@example.com');
      expect(result.restoreCount).toBe(2);
      expect(result.skippedCount).toBe(0);
    });

    it('should skip unknown placeholders', () => {
      const text = 'Unknown: {{PHONE_UNKNOWN123456789012345678}}';
      const result = restoreText(text, mappingMap, creator);

      expect(result.restoredText).toBe(text); // Unchanged
      expect(result.restoreCount).toBe(0);
      expect(result.skippedCount).toBe(1);
    });

    it('should enforce creator isolation', () => {
      const text = 'Phone: {{PHONE_01ARZ3NDEKTSV4RRFFQ69G5FAV}}';
      const result = restoreText(text, mappingMap, 'different-user');

      expect(result.restoredText).toBe(text); // Not restored
      expect(result.restoreCount).toBe(0);
      expect(result.skippedCount).toBe(1);
    });

    it('should skip expired mappings', () => {
      const expiredMapping: Mapping = {
        placeholder: '{{PHONE_EXPIRED12345678901234567}}',
        plaintext: '13800001111',
        ruleType: 'PHONE',
        creator,
        createdAt: now - 8 * 24 * 60 * 60 * 1000,
        expiresAt: now - 1000, // Expired
      };

      const map = createMappingMap([expiredMapping]);
      const text = 'Phone: {{PHONE_EXPIRED12345678901234567}}';
      const result = restoreText(text, map, creator);

      expect(result.restoredText).toBe(text); // Not restored
      expect(result.skippedCount).toBe(1);
    });
  });

  describe('SlidingWindowRestorer', () => {
    it('should handle complete placeholders', () => {
      const restorer = new SlidingWindowRestorer(mappingMap, creator);

      const chunk1 = 'Contact: {{PHONE_01ARZ3NDEKTSV4RRFFQ69G5FAV}}';
      const output1 = restorer.process(chunk1);

      expect(output1).toBe('Contact: 13812345678');

      const remaining = restorer.flush();
      expect(remaining).toBe('');
    });

    it('should handle cross-chunk placeholders', () => {
      const restorer = new SlidingWindowRestorer(mappingMap, creator);

      const chunk1 = 'Phone: {{PHONE_01ARZ3NDEKTSV';
      const output1 = restorer.process(chunk1);
      expect(output1).toBe('Phone: '); // Placeholder incomplete, buffered

      const chunk2 = '4RRFFQ69G5FAV}} end';
      const output2 = restorer.process(chunk2);
      expect(output2).toBe('13812345678 end'); // Restored

      const remaining = restorer.flush();
      expect(remaining).toBe('');
    });

    it('should handle split at placeholder boundary', () => {
      const restorer = new SlidingWindowRestorer(mappingMap, creator);

      const chunk1 = 'Text {{';
      const output1 = restorer.process(chunk1);
      expect(output1).toBe('Text '); // "{{" buffered

      const chunk2 = 'PHONE_01ARZ3NDEKTSV4RRFFQ69G5FAV}}';
      const output2 = restorer.process(chunk2);
      expect(output2).toBe('13812345678'); // Restored

      restorer.flush();
    });

    it('should flush incomplete data at end', () => {
      const restorer = new SlidingWindowRestorer(mappingMap, creator);

      const chunk = 'Incomplete {{PHONE_';
      restorer.process(chunk);

      const remaining = restorer.flush();
      expect(remaining).toBe('{{PHONE_'); // Not a valid placeholder, flushed as-is
    });

    it('should get statistics', () => {
      const restorer = new SlidingWindowRestorer(mappingMap, creator);

      restorer.process('Phone: {{PHONE_01ARZ3NDEKTSV4RRFFQ69G5FAV}}');
      restorer.process('Unknown: {{UNKNOWN_01ARZ3NDEKTSV4RRFFQ69G5XYZ}}');

      const stats = restorer.getStats();
      expect(stats.totalRestored).toBe(1);
      expect(stats.totalSkipped).toBe(1);
    });
  });
});
