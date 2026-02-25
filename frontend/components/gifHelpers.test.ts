import { describe, expect, it } from 'vitest';
import { buildGifProcessSuffix, resolveGifAction } from './gifHelpers';

describe('resolveGifAction', () => {
    it('maps 倒放 to reverse', () => {
        expect(resolveGifAction('倒放')).toBe('reverse');
    });

    it('maps 修改帧率 to change_speed', () => {
        expect(resolveGifAction('修改帧率')).toBe('change_speed');
    });

    it('maps 压缩 to compress', () => {
        expect(resolveGifAction('压缩')).toBe('compress');
    });

    it('maps 缩放 to resize', () => {
        expect(resolveGifAction('缩放')).toBe('resize');
    });

    it('falls back to change_speed for unknown mode', () => {
        expect(resolveGifAction('未知模式')).toBe('change_speed');
    });
});

describe('buildGifProcessSuffix', () => {
    it('returns reverse suffix', () => {
        expect(buildGifProcessSuffix('reverse', 100, 90)).toBe('_reverse');
    });

    it('returns speed suffix', () => {
        expect(buildGifProcessSuffix('change_speed', 125, 90)).toBe('_speed_125');
    });

    it('returns compress suffix', () => {
        expect(buildGifProcessSuffix('compress', 100, 77)).toBe('_compress_q77');
    });

    it('returns resize suffix', () => {
        expect(buildGifProcessSuffix('resize', 100, 77, 320, 180)).toBe('_resize_320x180');
    });
});
