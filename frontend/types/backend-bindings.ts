import type { models } from './backend-models';

export type AppBindings = {
    ProbeAnimatedPaths?: (arg1: Array<string>) => Promise<Array<{
        input_path: string;
        frame_count: number;
        is_animated: boolean;
        format?: string;
        error?: string;
    }>>;
    CancelProcessing: () => Promise<boolean> | boolean;
    Compress: (arg1: models.CompressRequest) => Promise<models.CompressResult>;
    CompressBatch: (arg1: Array<models.CompressRequest>) => Promise<Array<models.CompressResult>>;
    Convert: (arg1: models.ConvertRequest) => Promise<models.ConvertResult>;
    ConvertBatch: (arg1: Array<models.ConvertRequest>) => Promise<Array<models.ConvertResult>>;
    ExpandDroppedPaths: (arg1: Array<string>) => Promise<models.ExpandDroppedPathsResult>;
    GeneratePDF: (arg1: models.PDFRequest) => Promise<models.PDFResult>;
    GetSettings: () => Promise<models.AppSettings>;
    GetAppVersion?: () => Promise<{ success?: boolean; version?: string }>;
    GetThumbnail?: (arg1?: { path: string; max_edge?: number }) => Promise<{
        success: boolean;
        data_url?: string;
        width?: number;
        height?: number;
        error?: string;
    }>;
    OpenPath?: (arg1?: { path?: string; dir?: string }) => Promise<{ success?: boolean; path?: string; error?: string }>;
    CheckForUpdates?: (arg1?: { timeout_seconds?: number; force?: boolean }) => Promise<{
        success?: boolean;
        current_version?: string;
        latest_version?: string;
        update_available?: boolean;
        release_url?: string;
        release_notes?: string;
        checked?: boolean;
        error?: string;
        distribution_kind?: 'installed' | 'portable' | 'development';
        platform?: string;
        update_action?: 'install' | 'open_release';
        installer_available?: boolean;
        proxy_used?: boolean;
        cached?: boolean;
        asset_name?: string;
        asset_size?: number;
    }>;
    DownloadAndInstallUpdate?: (arg1?: { expected_version?: string; timeout_seconds?: number }) => Promise<{
        success?: boolean;
        action?: 'installer_started' | 'release_page' | 'none';
        version?: string;
        release_url?: string;
        proxy_used?: boolean;
        portable?: boolean;
        error?: string;
        message?: string;
    }>;
    OpenReleasePage?: (arg1?: { url?: string }) => Promise<{
        success?: boolean;
        action?: 'release_page';
        release_url?: string;
        error?: string;
    }>;
    Ping: () => Promise<string> | string;
    ResolveOutputPath: (arg1: models.ResolveOutputPathRequest) => Promise<models.ResolveOutputPathResult>;
    ResolveOutputPaths?: (arg1: { items: Array<string>; reserved?: Array<string>; strategy?: string }) => Promise<{
        success: boolean;
        paths?: Array<string>;
        exists?: Array<boolean>;
        error?: string;
    }>;
    SaveSettings: (arg1: models.AppSettings) => Promise<models.AppSettings>;
    SelectInputDirectory: () => Promise<string>;
    SelectInputFiles: (options?: unknown) => Promise<Array<string>>;
    SelectOutputDirectory: () => Promise<string>;
    SplitGIF: (arg1: models.GIFSplitRequest) => Promise<models.GIFSplitResult>;
    SplitImage?: (arg1?: {
        input_path: string;
        output_dir: string;
        rows?: number;
        cols?: number;
        mode?: 'square' | 'full';
        output_format?: string;
        quality?: number;
    }) => Promise<{
        success: boolean;
        rows?: number; cols?: number; tile_count?: number;
        tile_width?: number; tile_height?: number; uniform_tiles?: boolean;
        output_paths?: Array<string>;
        error_code?: string; error?: string;
    }>;
    AddWatermark?: (arg1?: {
        action?: 'apply' | 'preview';
        input_path: string;
        output_path?: string;
        text: string;
        font_name?: string;
        size_mode?: 'px' | 'percent';
        font_size?: number;
        color?: string;
        opacity?: number;
        rotation?: number;
        position?: string;
        margin?: number;
        tiled?: boolean;
        tile_gap_x?: number;
        tile_gap_y?: number;
        stroke_width?: number;
        stroke_color?: string;
        shadow?: boolean;
        output_format?: string;
        quality?: number;
    }) => Promise<{
        success: boolean;
        data_url?: string;
        width?: number; height?: number;
        output_path?: string;
        error_code?: string; error?: string;
    }>;
    ResizeImage?: (arg1?: {
        input_path: string;
        output_path: string;
        mode?: 'percent' | 'long_edge' | 'short_edge';
        value?: number;
        output_format?: string;
        quality?: number;
    }) => Promise<{
        success: boolean;
        original_width?: number; original_height?: number;
        width?: number; height?: number;
        error_code?: string; error?: string;
    }>;
    PaletteTool?: (arg1?: {
        action?: 'extract' | 'export_palette' | 'export_combined';
        input_path: string;
        output_path?: string;
        swatch_height?: number;
        include_hex_text?: boolean;
        output_format?: string;
    }) => Promise<{
        success: boolean;
        theme_color?: string;
        colors?: Array<string>;
        output_path?: string;
        error_code?: string; error?: string;
    }>;
    StitchImages?: (arg1?: {
        input_paths: Array<string>;
        output_path: string;
        direction?: 'vertical' | 'horizontal';
        target_width?: number;
        target_height?: number;
        gap?: number;
        background?: string;
        border_mode?: boolean;
        border?: number;
        output_format?: string;
    }) => Promise<{
        success: boolean;
        width?: number; height?: number; image_count?: number;
        error_code?: string; error?: string;
    }>;
    RenderRichText?: (arg1?: {
        blocks: Array<Record<string, any>>;
        width?: number; padding?: number;
        background?: string; text_color?: string; heading_color?: string; accent_color?: string;
        font_size?: number; line_height?: number; font_name?: string;
        preview?: boolean;
        output_path?: string;
    }) => Promise<{
        success: boolean;
        width?: number; height?: number;
        data_url?: string;
        missing_images?: Array<string>;
        output_path?: string;
        error_code?: string; error?: string;
    }>;
    CutImageGrid?: (arg1?: {
        input_path: string;
        output_dir: string;
        grid_size?: number;
        output_format?: string;
        quality?: number;
    }) => Promise<{
        success: boolean;
        input_path?: string;
        output_dir?: string;
        grid_size?: number;
        tile_count?: number;
        tile_width?: number;
        tile_height?: number;
        output_format?: string;
        output_paths?: Array<string>;
        error_code?: string;
        error?: string;
    }>;
    UpdateRecentPaths: (arg1: models.RecentPathsUpdateRequest) => Promise<models.AppSettings>;
    OpenFileDialog?: (options?: unknown) => Promise<string | string[] | null | undefined>;
    OpenDirectoryDialog?: (options?: unknown) => Promise<string | null | undefined>;
    Quit?: () => void | Promise<void>;
    WindowMinimise?: () => void | Promise<void>;
    WindowToggleMaximise?: () => void | Promise<void>;
    ResolveFilePaths?: (files: File[]) => Promise<unknown> | unknown;
    CanResolveFilePaths?: () => boolean;
};
