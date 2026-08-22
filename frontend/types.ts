export interface Feature {
    id: FeatureId;
    title: string;
    desc: string;
    iconName: string;
    color: string;
    bg: string;
    darkBg: string;
}

export type Theme = 'light' | 'dark' | 'system';

export type FeatureId =
    | 'converter'
    | 'compressor'
    | 'resize'
    | 'grid'
    | 'split'
    | 'stitch'
    | 'watermark'
    | 'palette'
    | 'pdf'
    | 'gif'
    | 'richtext'
    | 'settings';

export type ViewState = FeatureId;
