/**
 * Shared helpers for DetailView batch processing.
 * Kept pure/stateless so handlers can be unit-tested and DetailView stays thinner.
 */
import type { models } from '../../types/backend-models';
import {
    isCancellationError,
    normalizeBatchResults,
    summarizeBatchProgress,
} from '../batchHelpers';
import type { DroppedFile } from './detailTypes';

export type AppBindingsLike = {
    Convert?: (req: models.ConvertRequest) => Promise<any>;
    ConvertBatch?: (reqs: models.ConvertRequest[]) => Promise<any>;
    Compress?: (req: models.CompressRequest) => Promise<any>;
    CompressBatch?: (reqs: models.CompressRequest[]) => Promise<any>;
};

export type BatchRunnerContext = {
    app: AppBindingsLike;
    files: DroppedFile[];
    total: number;
    cancelRequestedRef: { current: boolean };
    setCancelRequested: (v: boolean) => void;
    setProgressThrottled: (v: number) => void;
    flushProgress: () => void;
    setLastMessage: (v: string) => void;
    reportTaskFailure: (taskName: string, filePath: string, reason: unknown, fallback?: string) => void;
    reportBatchTaskFailure: (taskName: string, files: DroppedFile[], reason: unknown, fallback?: string) => void;
    getBatchChunkSize: (itemCount: number, requestsPerItem?: number) => number;
    normalizePath: (p: string) => string;
};

export async function runConvertBatch(options: {
    ctx: BatchRunnerContext;
    format: string;
    quality: number;
    compressLevel: number;
    icoSizeGroups: number[][];
    overwriteSource: boolean;
    resizeMode: string;
    scalePercent: number;
    fixedWidth: number;
    fixedHeight: number;
    longEdge: number;
    maintainAR: boolean;
    keepMetadata: boolean;
    outputDir: string;
    outputTemplate: string;
    outputPrefix: string;
    preserveStructure: boolean;
    batchTime: Date;
    buildOutputRelPath: (file: DroppedFile, opts: any) => string;
    resolveUniquePath: (candidate: string) => Promise<string>;
    resolveConverterOverwritePath: (inputPath: string, format: string) => string;
    joinPath: (base: string, rel: string) => string;
    basename: (p: string) => string;
    stripExtension: (p: string) => string;
    onDone?: (summary: RunDoneSummary) => void;
}) {
    const {
        ctx, format, quality, compressLevel, icoSizeGroups, overwriteSource,
        resizeMode, scalePercent, fixedWidth, fixedHeight, longEdge, maintainAR, keepMetadata,
        outputDir, outputTemplate, outputPrefix, preserveStructure, batchTime,
        buildOutputRelPath, resolveUniquePath, resolveConverterOverwritePath, joinPath, basename, stripExtension, onDone,
    } = options;
    const isIcoFormat = format === 'ico';
    const totalTasks = isIcoFormat ? ctx.files.length * icoSizeGroups.length : ctx.total;
    let completed = 0;
    let failed = 0;
    let cancelledItems = 0;
    let skipped = 0;
    let seq = 1;
    const chunkSize = ctx.getBatchChunkSize(ctx.files.length, icoSizeGroups.length);

    for (let i = 0; i < ctx.files.length; i += chunkSize) {
        if (ctx.cancelRequestedRef.current) break;
        const group = ctx.files.slice(i, i + chunkSize);
        const chunk: models.ConvertRequest[] = [];
        for (const f of group) {
            if (ctx.cancelRequestedRef.current) break;
            const input_path = ctx.normalizePath(f.input_path);
            for (const icoSizeGroup of icoSizeGroups) {
                if (ctx.cancelRequestedRef.current) break;
                const canOverwrite = overwriteSource && !isIcoFormat;
                let output_path = input_path;
                if (isIcoFormat) {
                    const suffix = icoSizeGroup.length === 1 ? `_ico${icoSizeGroup[0]}` : '';
                    if (overwriteSource) {
                        const sourceFile = basename(input_path);
                        const sourceBase = stripExtension(sourceFile).replace(/_ico\d+(?:-\d+)*$/i, '');
                        const sourceDirPath = input_path.replace(/\\/g, '/');
                        const idx = sourceDirPath.lastIndexOf('/');
                        const sourceDir = idx === -1 ? '' : sourceDirPath.slice(0, idx);
                        const outName = `${sourceBase}${suffix}`;
                        const candidate = sourceDir ? `${sourceDir}/${outName}.${format}` : `${outName}.${format}`;
                        output_path = await resolveUniquePath(candidate);
                    } else {
                        const rel = buildOutputRelPath(f, {
                            ext: format,
                            suffix,
                            seq,
                            op: 'converter',
                            template: outputTemplate,
                            prefix: outputPrefix,
                            preserveStructure,
                            date: batchTime,
                        });
                        output_path = await resolveUniquePath(joinPath(outputDir, rel));
                    }
                } else if (!canOverwrite) {
                    const rel = buildOutputRelPath(f, {
                        ext: format,
                        seq,
                        op: 'converter',
                        template: outputTemplate,
                        prefix: outputPrefix,
                        preserveStructure,
                        date: batchTime,
                    });
                    output_path = await resolveUniquePath(joinPath(outputDir, rel));
                } else {
                    const overwritePath = resolveConverterOverwritePath(input_path, format);
                    if (ctx.normalizePath(overwritePath) !== input_path) {
                        output_path = await resolveUniquePath(overwritePath);
                    }
                }
                if (!output_path) {
                    skipped += 1;
                    completed += 1;
                    seq += 1;
                    continue;
                }
                chunk.push({
                    input_path,
                    output_path,
                    format,
                    quality,
                    compress_level: compressLevel,
                    ico_sizes: isIcoFormat ? icoSizeGroup : [],
                    icoSizes: isIcoFormat ? icoSizeGroup : [],
                    width: resizeMode === 'fixed' ? fixedWidth : 0,
                    height: resizeMode === 'fixed' ? fixedHeight : 0,
                    maintain_ar: maintainAR,
                    resize_mode: resizeMode,
                    scale_percent: resizeMode === 'percent' ? scalePercent : 0,
                    long_edge: resizeMode === 'long_edge' ? longEdge : 0,
                    keep_metadata: keepMetadata,
                } as models.ConvertRequest);
                seq += 1;
            }
        }
        if (ctx.cancelRequestedRef.current) break;
        if (chunk.length === 0) continue;
        try {
            const res = chunk.length === 1
                ? [await ctx.app.Convert!(chunk[0])]
                : await ctx.app.ConvertBatch!(chunk);
            const outcome = normalizeBatchResults(res, chunk, '转换失败');
            completed += outcome.settled;
            failed += outcome.failed;
            outcome.results.forEach((item, idx) => {
                if (!item?.success && !isCancellationError(item?.error)) {
                    ctx.reportTaskFailure('格式转换', chunk[idx]?.input_path || item?.input_path || '', item?.error, '转换失败');
                }
            });
            if (outcome.cancelled) {
                cancelledItems += outcome.results.filter((item) => isCancellationError(item?.error)).length;
                ctx.cancelRequestedRef.current = true;
                ctx.setCancelRequested(true);
                ctx.setProgressThrottled((completed / totalTasks) * 100);
                break;
            }
        } catch (err) {
            if (isCancellationError(err) || ctx.cancelRequestedRef.current) {
                ctx.cancelRequestedRef.current = true;
                ctx.setCancelRequested(true);
                break;
            }
            console.error(err);
            failed += chunk.length;
            completed += chunk.length;
            ctx.reportBatchTaskFailure('格式转换', group, err, '转换失败');
        }
        ctx.setProgressThrottled((completed / totalTasks) * 100);
    }
    const cancelled = ctx.cancelRequestedRef.current;
    onDone?.({ total: ctx.total, failed, skipped });
    const extraParts = [
        failed > 0 ? `失败 ${failed}` : '',
        skipped > 0 ? `跳过 ${skipped}` : '',
    ].filter(Boolean);
    const extra = extraParts.length > 0 ? `（${extraParts.join('，')}）` : '';
    ctx.flushProgress();
    if (cancelled || failed > 0 || skipped > 0) {
        ctx.setLastMessage(summarizeBatchProgress(totalTasks, completed, failed, cancelled, '转换', extra, cancelledItems));
    }
}

export type CompressBatchStats = {
    succeeded: number;
    failed: number;
    warnings: number;
    skipped: number;
    totalOriginalBytes: number;
    totalCompressedBytes: number;
};

export type RunDoneSummary = {
    total: number;
    failed: number;
    skipped: number;
};

export async function runCompressBatch(options: {
    ctx: BatchRunnerContext;
    level: number;
    engine: string;
    targetSizeKB: number;
    resizeMode: string;
    resizePercent: number;
    resizeWidth: number;
    resizeHeight: number;
    maintainAspect: boolean;
    overwriteSource: boolean;
    outputDir: string;
    outputTemplate: string;
    outputPrefix: string;
    preserveStructure: boolean;
    batchTime: Date;
    buildOutputRelPath: (file: DroppedFile, opts: any) => string;
    resolveUniquePath: (candidate: string) => Promise<string>;
    joinPath: (base: string, rel: string) => string;
    onStats?: (stats: CompressBatchStats) => void;
    onDone?: (summary: RunDoneSummary) => void;
}) {
    const {
        ctx, level, engine, targetSizeKB, resizeMode, resizePercent, resizeWidth, resizeHeight, maintainAspect, overwriteSource,
        outputDir, outputTemplate, outputPrefix, preserveStructure, batchTime,
        buildOutputRelPath, resolveUniquePath, joinPath, onStats, onDone,
    } = options;
    let completed = 0;
    let failed = 0;
    let cancelledItems = 0;
    let warnings = 0;
    let skipped = 0;
    let seq = 1;
    let totalOriginalBytes = 0;
    let totalCompressedBytes = 0;
    const chunkSize = ctx.getBatchChunkSize(ctx.files.length);

    for (let i = 0; i < ctx.files.length; i += chunkSize) {
        if (ctx.cancelRequestedRef.current) break;
        const group = ctx.files.slice(i, i + chunkSize);
        const chunk: models.CompressRequest[] = [];
        for (const f of group) {
            if (ctx.cancelRequestedRef.current) break;
            const input_path = ctx.normalizePath(f.input_path);
            let output_path = input_path;
            if (!overwriteSource) {
                const rel = buildOutputRelPath(f, {
                    seq,
                    op: 'compressor',
                    template: outputTemplate,
                    prefix: outputPrefix,
                    preserveStructure,
                    date: batchTime,
                });
                output_path = await resolveUniquePath(joinPath(outputDir, rel));
                if (!output_path) {
                    skipped += 1;
                    completed += 1;
                    seq += 1;
                    continue;
                }
            }
            chunk.push({
                input_path,
                output_path,
                level,
                engine,
                target_size_kb: targetSizeKB,
                strip_metadata: true,
                resize_mode: resizeMode,
                resize_percent: resizePercent,
                resize_width: resizeWidth,
                resize_height: resizeHeight,
                maintain_aspect: maintainAspect,
            } as models.CompressRequest);
            seq += 1;
        }
        if (ctx.cancelRequestedRef.current) break;
        if (chunk.length === 0) continue;
        try {
            const res = chunk.length === 1
                ? [await ctx.app.Compress!(chunk[0])]
                : await ctx.app.CompressBatch!(chunk);
            const outcome = normalizeBatchResults(res, chunk, '压缩失败');
            completed += outcome.settled;
            failed += outcome.failed;
            warnings += outcome.warnings;
            outcome.results.forEach((item, idx) => {
                if (item?.success) {
                    const originalSize = Number(item?.original_size);
                    const compressedSize = Number(item?.compressed_size);
                    if (Number.isFinite(originalSize) && originalSize > 0) totalOriginalBytes += originalSize;
                    if (Number.isFinite(compressedSize) && compressedSize >= 0) totalCompressedBytes += compressedSize;
                } else if (!item?.success && !isCancellationError(item?.error)) {
                    ctx.reportTaskFailure('图片压缩', chunk[idx]?.input_path || item?.input_path || '', item?.error, '压缩失败');
                }
            });
            if (outcome.cancelled) {
                cancelledItems += outcome.results.filter((item) => isCancellationError(item?.error)).length;
                ctx.cancelRequestedRef.current = true;
                ctx.setCancelRequested(true);
                ctx.setProgressThrottled((completed / ctx.total) * 100);
                break;
            }
        } catch (err) {
            if (isCancellationError(err) || ctx.cancelRequestedRef.current) {
                ctx.cancelRequestedRef.current = true;
                ctx.setCancelRequested(true);
                break;
            }
            console.error(err);
            failed += chunk.length;
            completed += chunk.length;
            ctx.reportBatchTaskFailure('图片压缩', group, err, '压缩失败');
        }
        ctx.setProgressThrottled((completed / ctx.total) * 100);
    }
    const cancelled = ctx.cancelRequestedRef.current;
    onStats?.({
        succeeded: Math.max(0, completed - failed - skipped),
        failed,
        warnings,
        skipped,
        totalOriginalBytes,
        totalCompressedBytes,
    });
    onDone?.({ total: ctx.total, failed, skipped });
    const extraParts = [
        failed > 0 ? `失败 ${failed}` : '',
        skipped > 0 ? `跳过 ${skipped}` : '',
        warnings > 0 ? `提示 ${warnings}` : '',
    ].filter(Boolean);
    const extra = extraParts.length > 0 ? `（${extraParts.join('，')}）` : '';
    ctx.flushProgress();
    if (cancelled || failed > 0 || warnings > 0 || skipped > 0) {
        ctx.setLastMessage(summarizeBatchProgress(ctx.total, completed, failed, cancelled, '压缩', extra, cancelledItems));
    }
}
