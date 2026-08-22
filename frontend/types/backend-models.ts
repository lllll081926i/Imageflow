export namespace models {
	export class AppSettings {
	    max_concurrency: number;
	    output_prefix: string;
	    output_template: string;
	    preserve_folder_structure: boolean;
	    conflict_strategy: string;
	    default_output_dir: string;
	    recent_input_dirs: string[];
	    recent_output_dirs: string[];
	    auto_check_updates?: boolean;
	    open_output_folder_on_complete?: boolean;
	    remember_window_state?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AppSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.max_concurrency = source["max_concurrency"];
	        this.output_prefix = source["output_prefix"];
	        this.output_template = source["output_template"];
	        this.preserve_folder_structure = source["preserve_folder_structure"];
	        this.conflict_strategy = source["conflict_strategy"];
	        this.default_output_dir = source["default_output_dir"];
	        this.recent_input_dirs = source["recent_input_dirs"];
	        this.recent_output_dirs = source["recent_output_dirs"];
	        this.auto_check_updates = source["auto_check_updates"];
	        this.open_output_folder_on_complete = source["open_output_folder_on_complete"];
	        this.remember_window_state = source["remember_window_state"];
	    }
	}
	export class CompressRequest {
	    input_path: string;
	    output_path: string;
	    level: number;
	    engine?: string;
	    target_size_kb?: number;
	    strip_metadata?: boolean;
	    resize_mode?: string;
	    resize_percent?: number;
	    resize_width?: number;
	    resize_height?: number;
	    maintain_aspect?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CompressRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.input_path = source["input_path"];
	        this.output_path = source["output_path"];
	        this.level = source["level"];
	        this.engine = source["engine"];
	        this.target_size_kb = source["target_size_kb"];
	        this.strip_metadata = source["strip_metadata"];
	        this.resize_mode = source["resize_mode"];
	        this.resize_percent = source["resize_percent"];
	        this.resize_width = source["resize_width"];
	        this.resize_height = source["resize_height"];
	        this.maintain_aspect = source["maintain_aspect"];
	    }
	}
	export class CompressResult {
	    success: boolean;
	    input_path: string;
	    output_path: string;
	    original_size: number;
	    compressed_size: number;
	    compression_rate: number;
	    compression_level: number;
	    warning?: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new CompressResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.input_path = source["input_path"];
	        this.output_path = source["output_path"];
	        this.original_size = source["original_size"];
	        this.compressed_size = source["compressed_size"];
	        this.compression_rate = source["compression_rate"];
	        this.compression_level = source["compression_level"];
	        this.warning = source["warning"];
	        this.error = source["error"];
	    }
	}
	export class ConvertRequest {
	    input_path: string;
	    output_path: string;
	    format: string;
	    quality: number;
	    width: number;
	    height: number;
	    maintain_ar: boolean;
	    resize_mode: string;
	    scale_percent: number;
	    long_edge: number;
	    keep_metadata: boolean;
	    compress_level: number;
	    ico_sizes: number[];
	    icoSizes?: number[];
	
	    static createFrom(source: any = {}) {
	        return new ConvertRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.input_path = source["input_path"];
	        this.output_path = source["output_path"];
	        this.format = source["format"];
	        this.quality = source["quality"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.maintain_ar = source["maintain_ar"];
	        this.resize_mode = source["resize_mode"];
	        this.scale_percent = source["scale_percent"];
	        this.long_edge = source["long_edge"];
	        this.keep_metadata = source["keep_metadata"];
	        this.compress_level = source["compress_level"];
	        this.ico_sizes = source["ico_sizes"];
	        this.icoSizes = source["icoSizes"];
	    }
	}
	export class ConvertResult {
	    success: boolean;
	    input_path: string;
	    output_path: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ConvertResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.input_path = source["input_path"];
	        this.output_path = source["output_path"];
	        this.error = source["error"];
	    }
	}
	export class DroppedFile {
	    input_path: string;
	    source_root: string;
	    relative_path: string;
	    is_from_dir_drop: boolean;
	    size: number;
	    mod_time: number;
	
	    static createFrom(source: any = {}) {
	        return new DroppedFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.input_path = source["input_path"];
	        this.source_root = source["source_root"];
	        this.relative_path = source["relative_path"];
	        this.is_from_dir_drop = source["is_from_dir_drop"];
	        this.size = source["size"];
	        this.mod_time = source["mod_time"];
	    }
	}
	export class ExpandDroppedPathsResult {
	    files: DroppedFile[];
	    has_directory: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ExpandDroppedPathsResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.files = this.convertValues(source["files"], DroppedFile);
	        this.has_directory = source["has_directory"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class GIFSplitRequest {
	    action?: string;
	    input_path?: string;
	    input_paths?: string[];
	    output_dir?: string;
	    output_path?: string;
	    output_format?: string;
	    frame_range?: string;
	    start_frame?: number;
	    end_frame?: number;
	    format?: string;
	    speed_factor?: number;
	    fps?: number;
	    quality?: number;
	    width?: number;
	    height?: number;
	    maintain_aspect: boolean;
	    loop?: number;
	
	    static createFrom(source: any = {}) {
	        return new GIFSplitRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.action = source["action"];
	        this.input_path = source["input_path"];
	        this.input_paths = source["input_paths"];
	        this.output_dir = source["output_dir"];
	        this.output_path = source["output_path"];
	        this.output_format = source["output_format"];
	        this.frame_range = source["frame_range"];
	        this.start_frame = source["start_frame"];
	        this.end_frame = source["end_frame"];
	        this.format = source["format"];
	        this.speed_factor = source["speed_factor"];
	        this.fps = source["fps"];
	        this.quality = source["quality"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.maintain_aspect = source["maintain_aspect"];
	        this.loop = source["loop"];
	    }
	}
	export class GIFSplitResult {
	    success: boolean;
	    input_path?: string;
	    input_paths?: string[];
	    output_dir?: string;
	    output_path?: string;
	    frame_count?: number;
	    export_count?: number;
	    frame_paths?: string[];
	    speed_factor?: number;
	    fps?: number;
	    quality?: number;
	    width?: number;
	    height?: number;
	    original_width?: number;
	    original_height?: number;
	    error_code?: string;
	    error_detail?: string;
	    warning?: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new GIFSplitResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.input_path = source["input_path"];
	        this.input_paths = source["input_paths"];
	        this.output_dir = source["output_dir"];
	        this.output_path = source["output_path"];
	        this.frame_count = source["frame_count"];
	        this.export_count = source["export_count"];
	        this.frame_paths = source["frame_paths"];
	        this.speed_factor = source["speed_factor"];
	        this.fps = source["fps"];
	        this.quality = source["quality"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.original_width = source["original_width"];
	        this.original_height = source["original_height"];
	        this.error_code = source["error_code"];
	        this.error_detail = source["error_detail"];
	        this.warning = source["warning"];
	        this.error = source["error"];
	    }
	}
	export class InfoBasic {
	    path?: string;
	    file_name?: string;
	    extension?: string;
	    format?: string;
	    mime_type?: string;
	    mode?: string;
	    width?: number;
	    height?: number;
	    bit_depth?: number;
	    file_size?: number;
	    modified?: number;
	    orientation?: string;
	    has_alpha?: boolean;
	    is_animated?: boolean;
	    frame_count?: number;
	    duration_ms?: number;
	    loop_count?: number;
	    dpi_x?: number;
	    dpi_y?: number;
	
	    static createFrom(source: any = {}) {
	        return new InfoBasic(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.file_name = source["file_name"];
	        this.extension = source["extension"];
	        this.format = source["format"];
	        this.mime_type = source["mime_type"];
	        this.mode = source["mode"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.bit_depth = source["bit_depth"];
	        this.file_size = source["file_size"];
	        this.modified = source["modified"];
	        this.orientation = source["orientation"];
	        this.has_alpha = source["has_alpha"];
	        this.is_animated = source["is_animated"];
	        this.frame_count = source["frame_count"];
	        this.duration_ms = source["duration_ms"];
	        this.loop_count = source["loop_count"];
	        this.dpi_x = source["dpi_x"];
	        this.dpi_y = source["dpi_y"];
	    }
	}
	export class InfoField {
	    key: string;
	    label: string;
	    value: string;
	    group?: string;
	    source?: string;
	    editable?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new InfoField(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.label = source["label"];
	        this.value = source["value"];
	        this.group = source["group"];
	        this.source = source["source"];
	        this.editable = source["editable"];
	    }
	}
	export class InfoRequest {
	    input_path: string;
	
	    static createFrom(source: any = {}) {
	        return new InfoRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.input_path = source["input_path"];
	    }
	}
	export class InfoWarning {
	    code?: string;
	    message?: string;
	
	    static createFrom(source: any = {}) {
	        return new InfoWarning(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.message = source["message"];
	    }
	}
	export class InfoResult {
	    success: boolean;
	    input_path: string;
	    file_name?: string;
	    format: string;
	    mode: string;
	    width: number;
	    height: number;
	    bit_depth?: number;
	    file_size: number;
	    modified?: number;
	    exif?: Record<string, string>;
	    metadata?: Record<string, any>;
	    basic?: InfoBasic;
	    format_details?: Record<string, string>;
	    fields?: InfoField[];
	    warnings?: InfoWarning[];
	    histogram?: Record<string, Array<number>>;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new InfoResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.input_path = source["input_path"];
	        this.file_name = source["file_name"];
	        this.format = source["format"];
	        this.mode = source["mode"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.bit_depth = source["bit_depth"];
	        this.file_size = source["file_size"];
	        this.modified = source["modified"];
	        this.exif = source["exif"];
	        this.metadata = source["metadata"];
	        this.basic = this.convertValues(source["basic"], InfoBasic);
	        this.format_details = source["format_details"];
	        this.fields = this.convertValues(source["fields"], InfoField);
	        this.warnings = this.convertValues(source["warnings"], InfoWarning);
	        this.histogram = source["histogram"];
	        this.error = source["error"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class MetadataEditRequest {
	    input_path: string;
	    output_path: string;
	    exif_data: Record<string, any>;
	    overwrite: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MetadataEditRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.input_path = source["input_path"];
	        this.output_path = source["output_path"];
	        this.exif_data = source["exif_data"];
	        this.overwrite = source["overwrite"];
	    }
	}
	export class MetadataEditResult {
	    success: boolean;
	    input_path: string;
	    output_path: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new MetadataEditResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.input_path = source["input_path"];
	        this.output_path = source["output_path"];
	        this.error = source["error"];
	    }
	}
	export class MetadataStripRequest {
	    input_path: string;
	    output_path: string;
	    overwrite: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MetadataStripRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.input_path = source["input_path"];
	        this.output_path = source["output_path"];
	        this.overwrite = source["overwrite"];
	    }
	}
	export class MetadataStripResult {
	    success: boolean;
	    input_path: string;
	    output_path: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new MetadataStripResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.input_path = source["input_path"];
	        this.output_path = source["output_path"];
	        this.error = source["error"];
	    }
	}
 	export class PDFRequest {
 	    image_paths: string[];
 	    output_path: string;
 	    page_size: string;
 	    layout: string;
 	    margin: number;
 	    compression_level: number;
 	    fit_mode?: string;
 	    title: string;
 	    author: string;
 	    user_password?: string;
 	
 	    static createFrom(source: any = {}) {
 	        return new PDFRequest(source);
 	    }
 	
 	    constructor(source: any = {}) {
 	        if ('string' === typeof source) source = JSON.parse(source);
 	        this.image_paths = source["image_paths"];
 	        this.output_path = source["output_path"];
 	        this.page_size = source["page_size"];
 	        this.layout = source["layout"];
 	        this.margin = source["margin"];
 	        this.compression_level = source["compression_level"];
 	        this.fit_mode = source["fit_mode"];
 	        this.title = source["title"];
 	        this.author = source["author"];
 	        this.user_password = source["user_password"];
 	    }
 	}
	export class PDFResult {
	    success: boolean;
	    output_path: string;
	    page_count: number;
	    file_size: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new PDFResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.output_path = source["output_path"];
	        this.page_count = source["page_count"];
	        this.file_size = source["file_size"];
	        this.error = source["error"];
	    }
	}
	export class RecentPathsUpdateRequest {
	    input_dir: string;
	    output_dir: string;
	
	    static createFrom(source: any = {}) {
	        return new RecentPathsUpdateRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.input_dir = source["input_dir"];
	        this.output_dir = source["output_dir"];
	    }
	}
	export class ResolveOutputPathRequest {
	    base_path: string;
	    strategy: string;
	    reserved?: string[];
	
	    static createFrom(source: any = {}) {
	        return new ResolveOutputPathRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.base_path = source["base_path"];
	        this.strategy = source["strategy"];
	        this.reserved = source["reserved"];
	    }
	}
 	export class ResolveOutputPathResult {
 	    success: boolean;
 	    output_path?: string;
 	    exists?: boolean;
 	    error?: string;
 	
 	    static createFrom(source: any = {}) {
 	        return new ResolveOutputPathResult(source);
 	    }
 	
 	    constructor(source: any = {}) {
 	        if ('string' === typeof source) source = JSON.parse(source);
 	        this.success = source["success"];
 	        this.output_path = source["output_path"];
 	        this.exists = source["exists"];
 	        this.error = source["error"];
 	    }
 	}
 }
