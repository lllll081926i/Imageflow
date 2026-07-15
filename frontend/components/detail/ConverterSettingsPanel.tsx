import React, { memo } from 'react';
import { Switch, StyledSlider, CustomSelect, SegmentedControl } from '../Controls';

type ConverterSettingsProps = {
    format: string;
    setFormat: (v: string) => void;
    quality: number;
    setQuality: (v: number) => void;
    compressLevel: number;
    setCompressLevel: (v: number) => void;
    icoSizes: number[];
    setIcoSizes: (v: number[]) => void;
    resizeMode: string;
    setResizeMode: (v: string) => void;
    scalePercent: number;
    setScalePercent: (v: number) => void;
    fixedWidth: number;
    setFixedWidth: (v: number) => void;
    fixedHeight: number;
    setFixedHeight: (v: number) => void;
    longEdge: number;
    setLongEdge: (v: number) => void;
    keepMetadata: boolean;
    setKeepMetadata: (v: boolean) => void;
    maintainAR: boolean;
    setMaintainAR: (v: boolean) => void;
    overwriteSource: boolean;
    setOverwriteSource: (v: boolean) => void;
};

export const ConverterSettings = memo(({
    format,
    setFormat,
    quality,
    setQuality,
    compressLevel,
    setCompressLevel,
    icoSizes,
    setIcoSizes,
    resizeMode,
    setResizeMode,
    scalePercent,
    setScalePercent,
    fixedWidth,
    setFixedWidth,
    fixedHeight,
    setFixedHeight,
    longEdge,
    setLongEdge,
    keepMetadata,
    setKeepMetadata,
    maintainAR,
    setMaintainAR,
    overwriteSource,
    setOverwriteSource,
}: ConverterSettingsProps) => {

    const toggleIcoSize = (size: number) => {
        if (icoSizes.includes(size)) {
            setIcoSizes(icoSizes.filter(s => s !== size));
        } else {
            setIcoSizes([...icoSizes, size].sort((a, b) => a - b));
        }
    };

    return (
        <div className="space-y-4">
            {/* Top Row: Format & Quality/Compression */}
            <div className="grid grid-cols-[1fr_2fr] gap-4">
                <CustomSelect 
                    label="目标格式" 
                    options={['JPG', 'PNG', 'WEBP', 'AVIF', 'TIFF', 'ICO', 'BMP']} 
                    value={format}
                    onChange={setFormat}
                />
                
                {['JPG', 'WEBP', 'AVIF'].includes(format) && (
                    <div className="space-y-2">
                        <label className="text-sm font-medium text-gray-700 dark:text-gray-300">输出质量</label>
                        <div className="h-10 flex items-center">
                            <StyledSlider value={quality} onChange={setQuality} unit="%" className="w-full" />
                        </div>
                    </div>
                )}

                {format === 'PNG' && (
                    <div className="space-y-2">
                        <label className="text-sm font-medium text-gray-700 dark:text-gray-300">压缩级别 (0-9)</label>
                        <div className="h-10 flex items-center">
                            <StyledSlider value={compressLevel} min={0} max={9} onChange={setCompressLevel} className="w-full" />
                        </div>
                    </div>
                )}
                
                {format === 'ICO' && (
                     <div className="space-y-2">
                        <label className="text-sm font-medium text-gray-700 dark:text-gray-300">包含尺寸</label>
                        <div className="flex flex-wrap gap-2 pt-1">
                            {[16, 32, 48, 64, 128, 256].map(s => (
                                <button 
                                    key={s}
                                    onClick={() => toggleIcoSize(s)}
                                    className={`px-2 py-1 text-xs rounded-md border transition-colors ${
                                        icoSizes.includes(s) 
                                        ? 'bg-[#007AFF] border-[#007AFF] text-white' 
                                        : 'bg-gray-50 dark:bg-white/5 border-gray-200 dark:border-white/10 text-gray-600 dark:text-gray-300 hover:border-[#007AFF]/50'
                                    }`}
                                >
                                    {s}px
                                </button>
                            ))}
                        </div>
                    </div>
                )}

                {!['JPG', 'WEBP', 'AVIF', 'PNG', 'ICO'].includes(format) && (
                    <div className="flex items-center justify-center h-full pt-6">
                        <span className="text-xs text-gray-400">无额外设置</span>
                    </div>
                )}
            </div>

            {/* Middle Row: Resize Mode Buttons */}
            <div className="space-y-2">
                <label className="text-sm font-medium text-gray-700 dark:text-gray-300">尺寸调整</label>
                <SegmentedControl 
                    options={['原图', '比例', '固定', '长边']}
                    value={resizeMode === '原图尺寸' ? '原图' : resizeMode === '按比例' ? '比例' : resizeMode === '固定宽高' ? '固定' : '长边'}
                    onChange={(v) => setResizeMode(v === '原图' ? '原图尺寸' : v === '比例' ? '按比例' : v === '固定' ? '固定宽高' : '最长边')}
                />
            </div>

            {resizeMode === '按比例' && (
                 <StyledSlider label="缩放比例" value={scalePercent} min={1} max={200} unit="%" onChange={setScalePercent} />
            )}

            {resizeMode === '固定宽高' && (
                <div className="flex flex-col gap-3 animate-enter">
                    <div className="flex gap-3">
                        <div className="flex-1 space-y-1">
                            <label className="text-xs text-gray-500">宽度 (px)</label>
                            <input type="number" value={fixedWidth || ''} onChange={e => setFixedWidth(Number(e.target.value || 0))} className="w-full px-3 py-2.5 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm focus:border-[#007AFF] focus:ring-1 focus:ring-[#007AFF] outline-none dark:text-white" placeholder="自动" />
                        </div>
                        <div className="flex-1 space-y-1">
                            <label className="text-xs text-gray-500">高度 (px)</label>
                            <input type="number" value={fixedHeight || ''} onChange={e => setFixedHeight(Number(e.target.value || 0))} className="w-full px-3 py-2.5 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm focus:border-[#007AFF] focus:ring-1 focus:ring-[#007AFF] outline-none dark:text-white" placeholder="自动" />
                        </div>
                    </div>
                    <div className="flex items-center justify-between">
                         <label className="text-sm font-medium text-gray-700 dark:text-gray-300">
                             保持纵横比
                             <span className="block text-xs font-normal text-gray-500 mt-0.5">
                                 若关闭则强制拉伸至指定宽高
                             </span>
                         </label>
                         <Switch checked={maintainAR} onChange={setMaintainAR} />
                    </div>
                </div>
            )}

            {resizeMode === '最长边' && (
                <div className="flex gap-3 animate-enter">
                    <div className="flex-1 space-y-1">
                        <label className="text-xs text-gray-500">最长边 (px)</label>
                        <input type="number" value={longEdge || ''} onChange={e => setLongEdge(Number(e.target.value || 0))} className="w-full px-3 py-2.5 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm focus:border-[#007AFF] focus:ring-1 focus:ring-[#007AFF] outline-none dark:text-white" placeholder="2048" />
                    </div>
                    <div className="flex-1" />
                </div>
            )}

            <div className="pt-2 border-t border-gray-100 dark:border-white/5 space-y-3">
                <Switch label="保留元数据 (EXIF)" checked={keepMetadata} onChange={setKeepMetadata} />
            </div>

            <div className="pt-2 border-t border-gray-100 dark:border-white/5 space-y-3">
                <Switch label="直接覆盖源文件" checked={overwriteSource} onChange={setOverwriteSource} />
            </div>
        </div>
    );
});
