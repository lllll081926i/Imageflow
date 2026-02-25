import type { models } from '../wailsjs/go/models';

export type GIFSplitRequest = models.GIFSplitRequest & {
    width?: number;
    height?: number;
    maintain_aspect?: boolean;
};

export type GIFSplitResult = models.GIFSplitResult & {
    error_code?: string;
    error_detail?: string;
};

export type AppBindings = {
    AddWatermark: (req: models.WatermarkRequest) => Promise<models.WatermarkResult>;
    AddWatermarkBatch: (reqs: models.WatermarkRequest[]) => Promise<models.WatermarkResult[]>;
    Adjust: (req: models.AdjustRequest) => Promise<models.AdjustResult>;
    AdjustBatch: (reqs: models.AdjustRequest[]) => Promise<models.AdjustResult[]>;
    ApplyFilter: (req: models.FilterRequest) => Promise<models.FilterResult>;
    ApplyFilterBatch: (reqs: models.FilterRequest[]) => Promise<models.FilterResult[]>;
    CancelProcessing: () => Promise<boolean>;
    Compress: (req: models.CompressRequest) => Promise<models.CompressResult>;
    CompressBatch: (reqs: models.CompressRequest[]) => Promise<models.CompressResult[]>;
    Convert: (req: models.ConvertRequest) => Promise<models.ConvertResult>;
    ConvertBatch: (reqs: models.ConvertRequest[]) => Promise<models.ConvertResult[]>;
    EditMetadata: (req: models.MetadataEditRequest) => Promise<models.MetadataEditResult>;
    ExpandDroppedPaths: (paths: string[]) => Promise<models.ExpandDroppedPathsResult>;
    GeneratePDF: (req: models.PDFRequest) => Promise<models.PDFResult>;
    GetImagePreview: (req: models.PreviewRequest) => Promise<models.PreviewResult>;
    GetInfo: (req: models.InfoRequest) => Promise<models.InfoResult>;
    GetSettings: () => Promise<models.AppSettings>;
    ListSystemFonts: () => Promise<string[]>;
    Ping: () => Promise<string>;
    ResolveOutputPath: (req: models.ResolveOutputPathRequest) => Promise<models.ResolveOutputPathResult>;
    SaveSettings: (req: models.AppSettings) => Promise<models.AppSettings>;
    SelectInputDirectory: () => Promise<string>;
    SelectInputFiles: () => Promise<string[]>;
    SelectOutputDirectory: () => Promise<string>;
    SplitGIF: (req: GIFSplitRequest) => Promise<GIFSplitResult>;
    StripMetadata: (req: models.MetadataStripRequest) => Promise<models.MetadataStripResult>;
};

export function getAppBindings(): Partial<AppBindings> | null {
    const app = window.go?.main?.App;
    if (!app) return null;
    return app as Partial<AppBindings>;
}
