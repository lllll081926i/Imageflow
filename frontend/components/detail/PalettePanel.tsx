import React, { memo } from 'react';
import { StyledSlider, Switch } from '../Controls';

export type PaletteData = {
    theme_color: string;
    colors: string[];
} | null;

type PalettePanelProps = {
    data: PaletteData;
    loading: boolean;
    swatchHeight: number;
    setSwatchHeight: (v: number) => void;
    includeText: boolean;
    setIncludeText: (v: boolean) => void;
    onExport: (kind: 'palette' | 'combined') => void;
    busy: boolean;
};

const PaletteSwatches: React.FC<{ theme: string; colors: string[] }> = ({ theme, colors }) => (
    <div className="space-y-2">
        <div className="flex items-center gap-3">
            <div
                className="w-12 h-12 rounded-xl border border-gray-200 dark:border-white/10 shadow-sm shrink-0"
                style={{ background: theme }}
                title={theme}
            />
            <div className="min-w-0">
                <div className="text-[11px] text-gray-400">主题色</div>
                <div className="text-sm font-mono text-gray-700 dark:text-gray-200">{theme.toUpperCase()}</div>
            </div>
        </div>
        <div className="flex rounded-xl overflow-hidden h-10 border border-gray-200 dark:border-white/10">
            {colors.map((hex) => (
                <div key={hex} className="flex-1 min-w-0" style={{ background: hex }} title={hex.toUpperCase()} />
            ))}
        </div>
        <div className="grid grid-cols-5 gap-1.5 font-mono text-[10px] text-gray-500 dark:text-gray-400">
            {colors.map((hex) => (
                <span key={hex} className="truncate text-center" title={hex.toUpperCase()}>{hex.slice(1).toUpperCase()}</span>
            ))}
        </div>
    </div>
);

export const PalettePanel = memo(({
    data,
    loading,
    swatchHeight,
    setSwatchHeight,
    includeText,
    setIncludeText,
    onExport,
    busy,
}: PalettePanelProps) => {
    return (
        <div className="space-y-6">
            {loading && <div className="text-xs text-gray-400">提取中...</div>}
            {!loading && !data && <div className="text-xs text-gray-400">拖入图片</div>}
            {!loading && data && (
                <PaletteSwatches theme={data.theme_color} colors={data.colors} />
            )}

            <StyledSlider
                label="色板高度"
                value={swatchHeight}
                min={24}
                max={400}
                unit="px"
                onChange={setSwatchHeight}
            />
            <Switch
                checked={includeText}
                onChange={setIncludeText}
                label="标注色号"
            />

            <button
                type="button"
                disabled={!data || busy}
                onClick={() => onExport('palette')}
                className="spring-tap-sm w-full py-2.5 rounded-xl bg-gradient-to-r from-[#007AFF] to-[#0055FF] hover:to-[#0044DD] text-xs font-semibold text-white shadow-md shadow-blue-500/25 transition-all duration-300 disabled:opacity-50 disabled:cursor-not-allowed"
            >
                {busy ? '导出中...' : '导出色板'}
            </button>
            <button
                type="button"
                disabled={!data || busy}
                onClick={() => onExport('combined')}
                className="spring-tap-sm w-full py-2.5 rounded-xl border border-[#007AFF] text-xs font-semibold text-[#007AFF] hover:bg-[#007AFF]/10 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
                拼接在图片下方导出
            </button>
        </div>
    );
});
