import React, { memo } from 'react';
import { Switch, StyledSlider, CustomSelect, SegmentedControl } from './Controls';

export type GifSettingsPanelProps = {
    mode: string;
    setMode: (v: string) => void;
    exportFormat: string;
    setExportFormat: (v: string) => void;
    convertFormat: string;
    setConvertFormat: (v: string) => void;
    speedPercent: number;
    setSpeedPercent: (v: number) => void;
    compressQuality: number;
    setCompressQuality: (v: number) => void;
    sourceType: 'gif' | 'images' | 'mixed' | 'empty';
    buildFps: number;
    setBuildFps: (v: number) => void;
    resizeWidth: number;
    resizeHeight: number;
    onResizeWidthChange: (v: number) => void;
    onResizeHeightChange: (v: number) => void;
    resizeMaintainAR: boolean;
    setResizeMaintainAR: (v: boolean) => void;
    originalWidth: number;
    originalHeight: number;
};

const GifSettingsPanel = memo(({
    mode,
    setMode,
    exportFormat,
    setExportFormat,
    convertFormat,
    setConvertFormat,
    speedPercent,
    setSpeedPercent,
    compressQuality,
    setCompressQuality,
    sourceType,
    buildFps,
    setBuildFps,
    resizeWidth,
    resizeHeight,
    onResizeWidthChange,
    onResizeHeightChange,
    resizeMaintainAR,
    setResizeMaintainAR,
    originalWidth,
    originalHeight,
}: GifSettingsPanelProps) => {
    if (sourceType === 'images') {
        return (
            <div className="space-y-6">
                <div className="space-y-2">
                    <label className="text-sm font-medium text-gray-700 dark:text-gray-300">合成 GIF</label>
                    <div className="text-xs text-gray-500 dark:text-gray-400">
                        所有输入都会按 PNG 流程处理，非 PNG 会先转换后再合成 GIF。
                    </div>
                </div>
                <div className="space-y-3 animate-enter">
                    <StyledSlider
                        label="合成帧率 (FPS)"
                        value={buildFps}
                        min={1}
                        max={60}
                        onChange={setBuildFps}
                    />
                </div>
                <div className="p-3 rounded-xl bg-purple-50 dark:bg-purple-500/10 text-xs text-purple-700 dark:text-purple-400 mt-2 border border-purple-100 dark:border-purple-500/10">
                    当前为图片序列模式，将输出合成 GIF。
                </div>
            </div>
        );
    }

    return (
        <div className="space-y-6">
            <div className="space-y-2">
                <label className="text-sm font-medium text-gray-700 dark:text-gray-300">处理模式</label>
                <SegmentedControl
                    options={['导出', '互转', '倒放', '修改帧率', '压缩', '缩放']}
                    value={mode}
                    onChange={setMode}
                />
            </div>

            {mode === '导出' && (
                <div className="space-y-3 animate-enter">
                    <CustomSelect
                        label="导出帧格式"
                        options={['PNG']}
                        value={exportFormat}
                        onChange={setExportFormat}
                    />
                    <div className="text-xs text-gray-500 dark:text-gray-400">
                        GIF 输入导出帧，图片输入合成 GIF（统一按 PNG 流程处理）。
                    </div>
                </div>
            )}

            {mode === '互转' && (
                <div className="space-y-3 animate-enter">
                    <CustomSelect
                        label="目标动图格式"
                        options={['GIF', 'APNG', 'WEBP']}
                        value={convertFormat}
                        onChange={setConvertFormat}
                    />
                    <div className="text-xs text-gray-500 dark:text-gray-400">
                        支持 GIF / APNG / Animated WebP 互转，尽量保留透明背景与帧时序。
                    </div>
                </div>
            )}

            {mode === '修改帧率' && (
                <div className="animate-enter space-y-3">
                    <StyledSlider
                        label="帧率倍数 (10%-200%)"
                        value={speedPercent}
                        min={10}
                        max={200}
                        onChange={setSpeedPercent}
                    />
                    <div className="text-xs text-gray-500 dark:text-gray-400">
                        10% 表示 10 倍慢，200% 表示 2 倍快。
                    </div>
                </div>
            )}

            {mode === '压缩' && (
                <div className="animate-enter space-y-3">
                    <label className="text-sm font-medium text-gray-700 dark:text-gray-300">保留质量</label>
                    <div className="flex items-center gap-3">
                        <input
                            type="range"
                            min={1}
                            max={100}
                            step={1}
                            value={compressQuality}
                            onChange={(e) => setCompressQuality(Number(e.target.value))}
                            className="w-full accent-[#007AFF]"
                        />
                        <div className="w-16 shrink-0 text-center font-mono text-sm text-[#007AFF] bg-[#007AFF]/10 border border-[#007AFF]/20 rounded-lg py-1.5">
                            {compressQuality}%
                        </div>
                    </div>
                    <div className="text-xs text-gray-500 dark:text-gray-400">
                        质量越高细节保留越多，文件体积通常也会更大。
                    </div>
                </div>
            )}

            {mode === '缩放' && (
                <div className="animate-enter space-y-3">
                    <div className="text-xs text-gray-500 dark:text-gray-400">
                        原始尺寸：{originalWidth > 0 && originalHeight > 0 ? `${originalWidth} x ${originalHeight} px` : '未读取到 GIF 尺寸'}
                    </div>
                    <div className="grid grid-cols-2 gap-3">
                        <div className="space-y-1">
                            <label className="text-xs text-gray-500">宽度 (px)</label>
                            <input
                                type="number"
                                min={1}
                                value={resizeWidth || ''}
                                onChange={(e) => onResizeWidthChange(Number(e.target.value || 0))}
                                className="w-full px-3 py-2.5 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm focus:border-[#007AFF] focus:ring-1 focus:ring-[#007AFF] outline-none dark:text-white"
                                placeholder="自动"
                            />
                        </div>
                        <div className="space-y-1">
                            <label className="text-xs text-gray-500">高度 (px)</label>
                            <input
                                type="number"
                                min={1}
                                value={resizeHeight || ''}
                                onChange={(e) => onResizeHeightChange(Number(e.target.value || 0))}
                                className="w-full px-3 py-2.5 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm focus:border-[#007AFF] focus:ring-1 focus:ring-[#007AFF] outline-none dark:text-white"
                                placeholder="自动"
                            />
                        </div>
                    </div>
                    <Switch label="保持纵横比" checked={resizeMaintainAR} onChange={setResizeMaintainAR} />
                    <div className="text-xs text-gray-500 dark:text-gray-400">
                        只填一个边会自动按原比例计算另一边；同时填写宽高时会按比例适配到目标范围内。
                    </div>
                </div>
            )}

            {sourceType === 'mixed' && (
                <div className="text-xs text-red-500">
                    请只选择 GIF 或图片序列，混合输入会导致操作失败。
                </div>
            )}
        </div>
    );
});

GifSettingsPanel.displayName = 'GifSettingsPanel';

export default GifSettingsPanel;
