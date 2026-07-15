import { describe, expect, it } from 'vitest';
import {
    isCancellationError,
    normalizeBatchResults,
    summarizeBatchProgress,
    type BatchItemResult,
} from './batchHelpers';

describe('normalizeBatchResults', () => {
    const chunk = [{ input_path: 'a.png' }, { input_path: 'b.png' }, { input_path: 'c.png' }];

    it('keeps per-item failures from list responses', () => {
        const outcome = normalizeBatchResults(
            [
                { success: true, input_path: 'a.png' },
                { success: false, error: 'boom', input_path: 'b.png' },
                { success: true, input_path: 'c.png' },
            ],
            chunk,
        );
        expect(outcome.failed).toBe(1);
        expect(outcome.success).toBe(2);
        expect(outcome.cancelled).toBe(false);
        expect(outcome.settled).toBe(3);
        expect(outcome.warnings).toBe(0);
    });

    it('turns error envelope dict into failures for every chunk item', () => {
        const outcome = normalizeBatchResults(
            { success: false, error: 'worker crashed' },
            chunk,
        );
        expect(outcome.failed).toBe(3);
        expect(outcome.success).toBe(0);
        expect(outcome.results).toHaveLength(3);
        expect(outcome.results[0].error).toBe('worker crashed');
        expect(outcome.results[1].input_path).toBe('b.png');
        expect(outcome.results[2].input_path).toBe('c.png');
        expect(outcome.results.every((item) => item.success === false)).toBe(true);
    });

    it('treats null/undefined/string responses as full-chunk failures', () => {
        for (const response of [null, undefined, 'bad', 42]) {
            const outcome = normalizeBatchResults(response, chunk, '批处理失败');
            expect(outcome.failed).toBe(3);
            expect(outcome.success).toBe(0);
            expect(outcome.settled).toBe(3);
            expect(outcome.results[0].error).toBe('批处理失败');
        }
    });

    it('detects cancellation without counting cancelled items as failed', () => {
        const outcome = normalizeBatchResults(
            [
                { success: true },
                { success: false, error: '[PY_CANCELLED] operation cancelled' },
                { success: false, error: 'disk full' },
            ],
            chunk,
        );
        expect(outcome.cancelled).toBe(true);
        expect(outcome.failed).toBe(1);
        expect(outcome.success).toBe(1);
        expect(outcome.settled).toBe(3);
    });

    it('supports alternate cancellation message forms', () => {
        const outcome = normalizeBatchResults(
            [
                { success: false, error: 'operation canceled by user' },
                { success: true },
            ],
            chunk.slice(0, 2),
        );
        expect(outcome.cancelled).toBe(true);
        expect(outcome.failed).toBe(0);
        expect(outcome.success).toBe(1);
    });

    it('pads short arrays to chunk length as failures', () => {
        const outcome = normalizeBatchResults([{ success: true }], chunk);
        expect(outcome.settled).toBe(3);
        expect(outcome.failed).toBe(2);
        expect(outcome.success).toBe(1);
        expect(outcome.results[1].input_path).toBe('b.png');
        expect(outcome.results[2].input_path).toBe('c.png');
    });

    it('truncates oversized arrays to chunk length', () => {
        const outcome = normalizeBatchResults(
            [
                { success: true },
                { success: true },
                { success: false, error: 'x' },
                { success: false, error: 'extra should be dropped' },
            ],
            chunk,
        );
        expect(outcome.settled).toBe(3);
        expect(outcome.failed).toBe(1);
        expect(outcome.success).toBe(2);
        expect(outcome.results).toHaveLength(3);
    });

    it('counts warnings independently of success', () => {
        const outcome = normalizeBatchResults(
            [
                { success: true, warning: 'soft notice' },
                { success: false, error: 'hard fail', warning: 'also warned' },
                { success: true },
            ],
            chunk,
        );
        expect(outcome.warnings).toBe(2);
        expect(outcome.success).toBe(2);
        expect(outcome.failed).toBe(1);
    });

    it('maps sparse non-object list entries to fallback failures', () => {
        const outcome = normalizeBatchResults(
            [null, 'x', { success: true }] as unknown as BatchItemResult[],
            chunk,
            'normalize fallback',
        );
        expect(outcome.results[0].success).toBe(false);
        expect(outcome.results[0].error).toBe('normalize fallback');
        expect(outcome.results[1].input_path).toBe('b.png');
        expect(outcome.results[2].success).toBe(true);
        expect(outcome.failed).toBe(2);
        expect(outcome.success).toBe(1);
    });

    it('simulates multi-chunk cancel accounting used by DetailView', () => {
        // chunk1 fully succeeds, chunk2 mixed cancel, later chunks not processed.
        const chunk1 = [{ input_path: '1.png' }, { input_path: '2.png' }];
        const chunk2 = [{ input_path: '3.png' }, { input_path: '4.png' }];

        let completed = 0;
        let failed = 0;
        let cancelled = false;

        const first = normalizeBatchResults(
            [{ success: true }, { success: true }],
            chunk1,
        );
        completed += first.settled;
        failed += first.failed;

        const second = normalizeBatchResults(
            [
                { success: true },
                { success: false, error: '[PY_CANCELLED] operation cancelled' },
            ],
            chunk2,
        );
        completed += second.settled;
        failed += second.failed;
        cancelled = second.cancelled;

        expect(cancelled).toBe(true);
        expect(completed).toBe(4);
        expect(failed).toBe(0);
        // success count in UI = completed - failed
        expect(completed - failed).toBe(4);
        // But true successful ops before cancel marker = first.success + second.success
        expect(first.success + second.success).toBe(3);
    });

    it('uses custom fallback error text for envelopes without error field', () => {
        const outcome = normalizeBatchResults({ success: false }, chunk, '自定义失败');
        expect(outcome.results.every((item) => item.error === '自定义失败')).toBe(true);
    });
});

describe('isCancellationError', () => {
    it('matches py cancel markers and casing variants', () => {
        expect(isCancellationError('[PY_CANCELLED] operation cancelled')).toBe(true);
        expect(isCancellationError('operation cancelled')).toBe(true);
        expect(isCancellationError('operation canceled')).toBe(true);
        expect(isCancellationError(new Error('operation cancelled'))).toBe(true);
        expect(isCancellationError({ message: 'operation cancelled' } as any)).toBe(false);
        expect(isCancellationError('other')).toBe(false);
        expect(isCancellationError('')).toBe(false);
        expect(isCancellationError(null)).toBe(false);
        expect(isCancellationError(undefined)).toBe(false);
    });
});

describe('summarizeBatchProgress', () => {
    it('formats cancelled and completed messages with failure extras', () => {
        expect(summarizeBatchProgress(3, 2, 1, true, '转换', '（失败 1）')).toContain('已停止');
        expect(summarizeBatchProgress(3, 2, 1, true, '转换', '（失败 1）')).toContain('成功 1/3');
        expect(summarizeBatchProgress(3, 3, 0, false, '转换')).toContain('完成');
        expect(summarizeBatchProgress(5, 5, 2, false, '压缩', '（失败 2）')).toBe(
            '压缩完成：成功 3/5 项（失败 2）',
        );
    });

    it('never reports negative success counts', () => {
        expect(summarizeBatchProgress(2, 1, 5, false, '水印')).toContain('成功 0/2');
    });
});
