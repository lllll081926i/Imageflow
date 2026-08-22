export type DroppedFile = {
    input_path: string;
    source_root: string;
    relative_path: string;
    is_from_dir_drop: boolean;
};

export type ExpandDroppedPathsResult = {
    files: DroppedFile[];
    has_directory: boolean;
};

export type OutputSettings = {
    output_prefix: string;
    output_template: string;
    preserve_folder_structure: boolean;
    conflict_strategy: string;
};

export const defaultOutputSettings: OutputSettings = {
    output_prefix: 'IF',
    output_template: '{prefix}{basename}',
    preserve_folder_structure: true,
    conflict_strategy: 'rename',
};

export function getBatchChunkSize(itemCount: number, requestsPerItem = 1, maxConcurrency?: number) {
    const safeRequestsPerItem = Math.max(1, Math.round(Number(requestsPerItem) || 1));
    const concurrency = Math.max(1, Math.min(32, Math.round(Number(maxConcurrency) || 0)));
    // Keep the pipeline full (~2 rounds of work in flight) while bounding cancel latency.
    const base = concurrency > 0 ? Math.min(64, Math.max(concurrency * 2, 8)) : 64;
    return Math.max(1, Math.min(itemCount, Math.floor(base / safeRequestsPerItem)));
}
