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
	export class CompressRequest {
	    input_path: string;
	    output_path: string;
	    mode: string;
	    quality: number;
	
	    static createFrom(source: any = {}) {
	        return new CompressRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.input_path = source["input_path"];
	        this.output_path = source["output_path"];
	        this.mode = source["mode"];
	        this.quality = source["quality"];
	    }
	}
	export class CompressResult {
	    success: boolean;
	    input_path: string;
	    output_path: string;
	    original_size: number;
	    compressed_size: number;
	    compression_rate: number;
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
	
	    static createFrom(source: any = {}) {
	        return new DroppedFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.input_path = source["input_path"];
	        this.source_root = source["source_root"];
	        this.relative_path = source["relative_path"];
	        this.is_from_dir_drop = source["is_from_dir_drop"];
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
	    input_path: string;
	    output_dir: string;
	    start_frame: number;
	    end_frame: number;
	    format: string;
	
	    static createFrom(source: any = {}) {
	        return new GIFSplitRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.input_path = source["input_path"];
	        this.output_dir = source["output_dir"];
	        this.start_frame = source["start_frame"];
	        this.end_frame = source["end_frame"];
	        this.format = source["format"];
	    }
	}
	export class GIFSplitResult {
	    success: boolean;
	    input_path: string;
	    output_dir: string;
	    frame_count: number;
	    frame_paths: string[];
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new GIFSplitResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.input_path = source["input_path"];
	        this.output_dir = source["output_dir"];
	        this.frame_count = source["frame_count"];
	        this.frame_paths = source["frame_paths"];
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
	    format: string;
	    mode: string;
	    width: number;
	    height: number;
	    file_size: number;
	    exif?: Record<string, string>;
	    histogram?: Record<string, Array<number>>;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new InfoResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.input_path = source["input_path"];
	        this.format = source["format"];
	        this.mode = source["mode"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.file_size = source["file_size"];
	        this.exif = source["exif"];
	        this.histogram = source["histogram"];
	        this.error = source["error"];
	    }
	}
	export class PDFRequest {
	    image_paths: string[];
	    output_path: string;
	    page_size: string;
	    layout: string;
	    margin: number;
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

