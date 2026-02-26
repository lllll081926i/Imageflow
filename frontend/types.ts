export interface Feature {
    id: FeatureId;
    title: string;
    desc: string;
    iconName: string;
    color: string;
    bg: string;
    darkBg: string;
}

export type Theme = 'light' | 'dark';

export type FeatureId =
    | 'converter'
    | 'compressor'
    | 'pdf'
    | 'gif'
    | 'info'
    | 'watermark'
    | 'adjust'
    | 'filter'
    | 'settings';

export type ViewState = 'dashboard' | FeatureId;
