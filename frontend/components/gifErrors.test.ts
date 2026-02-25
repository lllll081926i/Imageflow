import { describe, expect, it } from 'vitest';
import { resolveGifErrorMessage } from './gifErrors';

describe('resolveGifErrorMessage', () => {
    it('returns mapped message by code', () => {
        expect(resolveGifErrorMessage('GIF_MEMORY_LIMIT', 'x')).toBe('GIF 体积过大，超出安全处理上限');
    });

    it('falls back to backend error text', () => {
        expect(resolveGifErrorMessage('', 'custom backend error')).toBe('custom backend error');
    });

    it('falls back to generic text', () => {
        expect(resolveGifErrorMessage('UNKNOWN_CODE', '')).toBe('GIF 处理失败');
    });
});
