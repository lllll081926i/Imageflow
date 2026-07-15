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
    AddWatermark?: (req: models.WatermarkRequest) => Promise<any>;
    AddWatermarkBatch?: (reqs: models.WatermarkRequest[]) => Promise<any>;
    Adjust?: (req: models.AdjustRequest) => Promise<any>;
    AdjustBatch?: (reqs: models.AdjustRequest[]) => Promise<any>;
    ApplyFilter?: (req: models.FilterRequest) => Promise<any>;
    ApplyFilterBatch?: (reqs: models.FilterRequest[]) => Promise<any>;
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

export async function runGenericBatch(options: {
    ctx: BatchRunnerContext;
    taskName: string;
    label: string;
    fallbackError: string;
    buildChunk: (group: DroppedFile[], seqStart: number) => Promise<{ chunk: any[]; nextSeq: number }>;
    runSingle: (item: any) => Promise<any>;
    runBatch?: (items: any[]) => Promise<any>;
    canBatch?: boolean;
}) {
    const {
        ctx,
        taskName,
        label,
        fallbackError,
        buildChunk,
        runSingle,
        runBatch,
        canBatch = true,
    } = options;
    const { files, total } = ctx;
    let completed = 0;
    let failed = 0;
    let seq = 1;
    const chunkSize = canBatch && runBatch ? ctx.getBatchChunkSize(files.length) : 1;

    for (let i = 0; i < files.length; i += chunkSize) {
        if (ctx.cancelRequestedRef.current) break;
        const group = files.slice(i, i + chunkSize);
        const built = await buildChunk(group, seq);
        const chunk = built.chunk;
        seq = built.nextSeq;
        if (ctx.cancelRequestedRef.current || chunk.length === 0) break;

        try {
            const res = (chunk.length > 1 && canBatch && runBatch)
                ? await runBatch(chunk)
                : [await runSingle(chunk[0])];
            const outcome = normalizeBatchResults(res, chunk, fallbackError);
            completed += outcome.settled;
            failed += outcome.failed;
            outcome.results.forEach((item, idx) => {
                if (!item?.success && !isCancellationError(item?.error)) {
                    ctx.reportTaskFailure(
                        taskName,
                        chunk[idx]?.input_path || item?.input_path || '',
                        item?.error,
                        fallbackError,
                    );
                }
            });
            if (outcome.cancelled) {
                ctx.cancelRequestedRef.current = true;
                ctx.setCancelRequested(true);
                ctx.setProgressThrottled((completed / total) * 100);
                break;
            }
        } catch (err) {
            if (isCancellationError(err) || ctx.cancelRequestedRef.current) {
                ctx.cancelRequestedRef.current = true;
                ctx.setCancelRequested(true);
                break;
            }
            console.error(`Failed to process ${label} batch:`, err);
            failed += chunk.length;
            completed += chunk.length;
            ctx.reportBatchTaskFailure(taskName, group, err, fallbackError);
        }
        ctx.setProgressThrottled((completed / total) * 100);
    }

    const cancelled = ctx.cancelRequestedRef.current;
    const extra = failed > 0 ? `（失败 ${failed}）` : '';
    ctx.flushProgress();
    ctx.setLastMessage(summarizeBatchProgress(total, completed, failed, cancelled, label, extra));
}


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
}) {
    const {
        ctx, format, quality, compressLevel, icoSizeGroups, overwriteSource,
        resizeMode, scalePercent, fixedWidth, fixedHeight, longEdge, maintainAR, keepMetadata,
        outputDir, outputTemplate, outputPrefix, preserveStructure, batchTime,
        buildOutputRelPath, resolveUniquePath, resolveConverterOverwritePath, joinPath, basename, stripExtension,
    } = options;
    const isIcoFormat = format === 'ico';
    const totalTasks = isIcoFormat ? ctx.files.length * icoSizeGroups.length : ctx.total;
    let completed = 0;
    let failed = 0;
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
        if (ctx.cancelRequestedRef.current || chunk.length === 0) break;
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
    const extra = failed > 0 ? `（失败 ${failed}）` : '';
    ctx.flushProgress();
    ctx.setLastMessage(summarizeBatchProgress(totalTasks, completed, failed, cancelled, '转换', extra));
}

export async function runCompressBatch(options: {
    ctx: BatchRunnerContext;
    level: number;
    engine: string;
    targetSizeKB: number;
    overwriteSource: boolean;
    outputDir: string;
    outputTemplate: string;
    outputPrefix: string;
    preserveStructure: boolean;
    batchTime: Date;
    buildOutputRelPath: (file: DroppedFile, opts: any) => string;
    resolveUniquePath: (candidate: string) => Promise<string>;
    joinPath: (base: string, rel: string) => string;
}) {
    const {
        ctx, level, engine, targetSizeKB, overwriteSource,
        outputDir, outputTemplate, outputPrefix, preserveStructure, batchTime,
        buildOutputRelPath, resolveUniquePath, joinPath,
    } = options;
    let completed = 0;
    let failed = 0;
    let warnings = 0;
    let seq = 1;
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
            }
            chunk.push({
                input_path,
                output_path,
                level,
                engine,
                target_size_kb: targetSizeKB,
                strip_metadata: true,
            } as models.CompressRequest);
            seq += 1;
        }
        if (ctx.cancelRequestedRef.current || chunk.length === 0) break;
        try {
            const res = chunk.length === 1
                ? [await ctx.app.Compress!(chunk[0])]
                : await ctx.app.CompressBatch!(chunk);
            const outcome = normalizeBatchResults(res, chunk, '压缩失败');
            completed += outcome.settled;
            failed += outcome.failed;
            warnings += outcome.warnings;
            outcome.results.forEach((item, idx) => {
                if (!item?.success && !isCancellationError(item?.error)) {
                    ctx.reportTaskFailure('图片压缩', chunk[idx]?.input_path || item?.input_path || '', item?.error, '压缩失败');
                }
            });
            if (outcome.cancelled) {
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
    const extraParts = [
        failed > 0 ? `失败 ${failed}` : '',
        warnings > 0 ? `提示 ${warnings}` : '',
    ].filter(Boolean);
    const extra = extraParts.length > 0 ? `（${extraParts.join('，')}）` : '';
    ctx.flushProgress();
    ctx.setLastMessage(summarizeBatchProgress(ctx.total, completed, failed, cancelled, '压缩', extra));
}
