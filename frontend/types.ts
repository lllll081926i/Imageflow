export interface Feature {
    id: string;
    title: string;
    desc: string;
    iconName: string;
    color: string;
    bg: string;
    darkBg: string;
}

export type Theme = 'light' | 'dark';

export type ViewState = 'dashboard' | string;