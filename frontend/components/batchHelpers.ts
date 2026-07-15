export type BatchItemResult = {
    success?: boolean;
    error?: unknown;
    warning?: unknown;
    input_path?: string;
    [key: string]: unknown;
};

export type NormalizedBatchOutcome = {
    results: BatchItemResult[];
    failed: number;
    warnings: number;
    cancelled: boolean;
    settled: number;
    success: number;
};

export function isCancellationError(error: unknown): boolean {
    if (error == null) {
        return false;
    }
    if (typeof error === 'string') {
        return error.includes('[PY_CANCELLED]') || /operation cancel(?:led|ed)/i.test(error);
    }
    if (error instanceof Error) {
        return isCancellationError(error.message);
    }
    try {
        return isCancellationError(String(error));
    } catch {
        return false;
    }
}

/**
 * Normalize *Batch API responses.
 * Backend must return list[dict]; older/error envelopes become per-item failures.
 */
export function normalizeBatchResults(
    response: unknown,
    chunk: Array<{ input_path?: string }>,
    fallbackError = '批处理失败',
): NormalizedBatchOutcome {
    const size = Math.max(0, chunk.length);
    let results: BatchItemResult[] = [];

    if (Array.isArray(response)) {
        results = response.map((item, idx) => {
            if (item && typeof item === 'object') {
                return item as BatchItemResult;
            }
            return {
                success: false,
                error: fallbackError,
                input_path: chunk[idx]?.input_path || '',
            };
        });
        while (results.length < size) {
            results.push({
                success: false,
                error: fallbackError,
                input_path: chunk[results.length]?.input_path || '',
            });
        }
        if (results.length > size) {
            results = results.slice(0, size);
        }
    } else if (response && typeof response === 'object') {
        const envelope = response as BatchItemResult;
        const error = envelope.error ?? fallbackError;
        results = chunk.map((item) => ({
            success: false,
            error,
            input_path: item.input_path || '',
        }));
    } else {
        results = chunk.map((item) => ({
            success: false,
            error: fallbackError,
            input_path: item.input_path || '',
        }));
    }

    let failed = 0;
    let warnings = 0;
    let cancelled = false;
    let success = 0;

    for (const item of results) {
        if (item?.warning) {
            warnings += 1;
        }
        if (item?.success) {
            success += 1;
            continue;
        }
        if (isCancellationError(item?.error)) {
            cancelled = true;
            continue;
        }
        failed += 1;
    }

    return {
        results,
        failed,
        warnings,
        cancelled,
        settled: results.length,
        success,
    };
}

export function summarizeBatchProgress(total: number, settled: number, failed: number, cancelled: boolean, label: string, extra = ''): string {
    const safeTotal = Math.max(0, total);
    const success = Math.max(0, settled - failed);
    const body = `${label}${cancelled ? '已停止' : '完成'}：成功 ${success}/${safeTotal} 项${extra}`;
    return body;
}
