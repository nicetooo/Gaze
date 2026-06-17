import { describe, it, expect } from 'vitest';
import en from './en.json';
import zh from './zh.json';
import ja from './ja.json';
import ko from './ko.json';
import zhTW from './zh-TW.json';

// Regression guard: all locales must share the exact same key set, so no
// language silently falls back to English (e.g. the 70-key drift fixed in
// ja/ko/zh-TW for the plugins page and MCP plugin tools).
const LOCALES: Record<string, unknown> = { en, zh, ja, ko, 'zh-TW': zhTW };

function flattenKeys(obj: unknown, prefix = ''): string[] {
  if (obj === null || typeof obj !== 'object' || Array.isArray(obj)) {
    return [prefix];
  }
  return Object.entries(obj as Record<string, unknown>).flatMap(([key, value]) =>
    flattenKeys(value, prefix ? `${prefix}.${key}` : key),
  );
}

describe('locale key consistency', () => {
  const enKeys = new Set(flattenKeys(en));

  it.each(Object.keys(LOCALES).filter((l) => l !== 'en'))(
    '%s has the same key set as en',
    (locale) => {
      const keys = new Set(flattenKeys(LOCALES[locale]));
      const missing = [...enKeys].filter((k) => !keys.has(k));
      const extra = [...keys].filter((k) => !enKeys.has(k));
      expect(missing, `keys missing in ${locale}`).toEqual([]);
      expect(extra, `extra keys in ${locale} not present in en`).toEqual([]);
    },
  );

  it('no locale has empty string values for plugin keys', () => {
    for (const [locale, data] of Object.entries(LOCALES)) {
      const plugins = (data as Record<string, Record<string, string>>).plugins;
      expect(plugins, `plugins section in ${locale}`).toBeDefined();
      for (const [key, value] of Object.entries(plugins)) {
        expect(value.trim(), `plugins.${key} in ${locale}`).not.toBe('');
      }
    }
  });
});
