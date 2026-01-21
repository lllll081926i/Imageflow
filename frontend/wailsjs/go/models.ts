export namespace models {
	
	export class AdjustRequest {
	    input_path: string;
	    output_path: string;
	    rotate: number;
	    flip_h: boolean;
	    flip_v: boolean;
	    brightness: number;
	    contrast: number;
	    saturation: number;
	    hue: number;
	
	    static createFrom(source: any = {}) {
	        return new AdjustRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.input_path = source["input_path"];
	        this.output_path = source["output_path"];
	        this.rotate = source["rotate"];
	        this.flip_h = source["flip_h"];
	        this.flip_v = source["flip_v"];
	        this.brightness = source["brightness"];
	        this.contrast = source["contrast"];
	        this.saturation = source["saturation"];
	        this.hue = source["hue"];
	    }
	}
	export class AdjustResult {
	    success: boolean;
	    input_path: string;
	    output_path: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new AdjustResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.input_path = source["input_path"];
	        this.output_path = source["output_path"];
	        this.error = source["error"];
	    }
	}
	export class AppSettings {
	    max_concurrency: number;
	
	    static createFrom(source: any = {}) {
	        return new AppSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.max_concurrency = source["max_concurrency"];
	    }
	}
	export class CompressRequest {
	    input_path: string;
	    output_path: string;
	    level: number;
	    engine?: string;
	    target_size_kb?: number;
	    strip_metadata?: boolean;
	
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
	export class FilterRequest {
	    input_path: string;
	    output_path: string;
	    filter_type: string;
	    intensity: number;
	
	    static createFrom(source: any = {}) {
	        return new FilterRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.input_path = source["input_path"];
	        this.output_path = source["output_path"];
	        this.filter_type = source["filter_type"];
	        this.intensity = source["intensity"];
	    }
	}
	export class FilterResult {
	    success: boolean;
	    input_path: string;
	    output_path: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new FilterResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.input_path = source["input_path"];
	        this.output_path = source["output_path"];
	        this.error = source["error"];
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
	        this.warning = source["warning"];
	        this.error = source["error"];
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
	        this.histogram = source["histogram"];
	        this.error = source["error"];
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
	export class PreviewRequest {
	    input_path: string;
	
	    static createFrom(source: any = {}) {
	        return new PreviewRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.input_path = source["input_path"];
	    }
	}
	export class PreviewResult {
	    success: boolean;
	    data_url?: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new PreviewResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.data_url = source["data_url"];
	        this.error = source["error"];
	    }
	}
	export class WatermarkRequest {
	    input_path: string;
	    output_path: string;
	    watermark_type: string;
	    text?: string;
	    image_path?: string;
	    position: string;
	    opacity: number;
	    scale: number;
	    font_size: number;
	    font_color: string;
	    rotation: number;
	
	    static createFrom(source: any = {}) {
	        return new WatermarkRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.input_path = source["input_path"];
	        this.output_path = source["output_path"];
	        this.watermark_type = source["watermark_type"];
	        this.text = source["text"];
	        this.image_path = source["image_path"];
	        this.position = source["position"];
	        this.opacity = source["opacity"];
	        this.scale = source["scale"];
	        this.font_size = source["font_size"];
	        this.font_color = source["font_color"];
	        this.rotation = source["rotation"];
	    }
	}
	export class WatermarkResult {
	    success: boolean;
	    input_path: string;
	    output_path: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new WatermarkResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.input_path = source["input_path"];
	        this.output_path = source["output_path"];
	        this.error = source["error"];
	    }
	}

}

