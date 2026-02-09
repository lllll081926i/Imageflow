import React, { useCallback, useMemo, useState, useEffect, useRef, memo } from 'react';
import { createPortal } from 'react-dom';
import Icon from './Icon';
import { FEATURES } from '../constants';
import { ViewState } from '../types';
import { Switch, StyledSlider, CustomSelect, SegmentedControl, PositionGrid, FileDropZone, ProgressBar } from './Controls';

interface DetailViewProps {
    id: ViewState;
    onBack: () => void;
    isActive?: boolean;
    onTaskFailure?: (payload: { taskName: string; imageName: string; reason: string }) => void;
}

type DroppedFile = {
    input_path: string;
    source_root: string;
    relative_path: string;
    is_from_dir_drop: boolean;
};

type ExpandDroppedPathsResult = {
    files: DroppedFile[];
    has_directory: boolean;
};

type OutputSettings = {
    output_prefix: string;
    output_template: string;
    preserve_folder_structure: boolean;
    conflict_strategy: string;
};

const defaultOutputSettings: OutputSettings = {
    output_prefix: 'IF',
    output_template: '{prefix}{basename}',
    preserve_folder_structure: true,
    conflict_strategy: 'rename',
};

// --- Feature Settings Panels (Memoized) ---

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

const ConverterSettings = memo(({
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
                            {[16, 32, 48, 64, 128, 256, 512, 1024].map(s => (
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

type CompressorSettingsProps = {
    mode: string;
    setMode: (v: string) => void;
    targetSize: boolean;
    setTargetSize: (v: boolean) => void;
    targetSizeKB: number;
    setTargetSizeKB: (v: number) => void;
    engine: string;
    setEngine: (v: string) => void;
    overwriteSource: boolean;
    setOverwriteSource: (v: boolean) => void;
};

const CompressorSettings = memo(({
    mode,
    setMode,
    targetSize,
    setTargetSize,
    targetSizeKB,
    setTargetSizeKB,
    engine,
    setEngine,
    overwriteSource,
    setOverwriteSource,
}: CompressorSettingsProps) => {
    
    return (
        <div className="space-y-6">
            <div className="space-y-4">
                 <label className="text-sm font-medium text-gray-700 dark:text-gray-300">压缩策略</label>
                 <SegmentedControl 
                    options={['无损', '轻度', '标准', '强力', '极限']}
                    value={mode}
                    onChange={setMode}
                 />
            </div>

            <CustomSelect label="压缩引擎" options={['自动 (推荐)', 'MozJPEG (JPEG)', 'PNGQuant (PNG)', 'OxiPNG (PNG 无损)', 'Pillow (兼容)']} value={engine} onChange={setEngine} />

            <div className="space-y-4 pt-2">
                <Switch label="指定目标大小限制 (KB)" checked={targetSize} onChange={setTargetSize} />
                {targetSize && (
                     <div className="animate-enter">
                        <input type="number" value={targetSizeKB} onChange={e => setTargetSizeKB(Number(e.target.value))} placeholder="例如: 500" className="w-full px-3 py-2.5 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm focus:outline-none focus:ring-2 focus:ring-[#007AFF]/50 focus:border-[#007AFF] dark:text-white transition-all" />
                     </div>
                )}
            </div>

            <div className="pt-2 border-t border-gray-100 dark:border-white/5 space-y-3">
                <Switch label="直接覆盖源文件" checked={overwriteSource} onChange={setOverwriteSource} />
            </div>
        </div>
    );
});

type WatermarkSettingsProps = {
    type: string;
    setType: (v: string) => void;
    text: string;
    setText: (v: string) => void;
    imagePath: string;
    setImagePath: (v: string) => void;
    position: string;
    setPosition: (v: string) => void;
    opacity: number;
    setOpacity: (v: number) => void;
    rotate: number;
    setRotate: (v: number) => void;
    size: number;
    setSize: (v: number) => void;
    tiled: boolean;
    setTiled: (v: boolean) => void;
    blendMode: string;
    setBlendMode: (v: string) => void;
    shadow: boolean;
    setShadow: (v: boolean) => void;
    margin: { x: number; y: number };
    setMargin: (v: { x: number; y: number }) => void;
    font: string;
    setFont: (v: string) => void;
    color: string;
    setColor: (v: string) => void;
    useSystemFonts: boolean;
    setUseSystemFonts: (v: boolean) => void;
    isSystemFontsLoading: boolean;
    fontOptions: Array<string | { label: string; value: string }>;
    systemFontsCount: number;
};

const WatermarkSettings = memo(({
    type,
    setType,
    text,
    setText,
    imagePath,
    setImagePath,
    position,
    setPosition,
    opacity,
    setOpacity,
    rotate,
    setRotate,
    size,
    setSize,
    tiled,
    setTiled,
    blendMode,
    setBlendMode,
    shadow,
    setShadow,
    margin,
    setMargin,
    font,
    setFont,
    color,
    setColor,
    useSystemFonts,
    setUseSystemFonts,
    isSystemFontsLoading,
    fontOptions,
    systemFontsCount,
}: WatermarkSettingsProps) => {
    const [colorInput, setColorInput] = useState(color.toUpperCase());
    const [isColorPickerOpen, setIsColorPickerOpen] = useState(false);
    const [colorPickerStyle, setColorPickerStyle] = useState<{ top: number; left: number } | null>(null);
    const colorPickerRef = useRef<HTMLDivElement>(null);
    const colorButtonRef = useRef<HTMLButtonElement>(null);
    const colorPopoverRef = useRef<HTMLDivElement>(null);

    useEffect(() => {
        setColorInput(color.toUpperCase());
    }, [color]);

    useEffect(() => {
        if (!isColorPickerOpen) return;
        const handleClickOutside = (event: MouseEvent) => {
            if (!(event.target instanceof Node)) return;
            if (colorPickerRef.current?.contains(event.target)) return;
            if (colorPopoverRef.current?.contains(event.target)) return;
            setIsColorPickerOpen(false);
        };
        document.addEventListener('mousedown', handleClickOutside);
        return () => document.removeEventListener('mousedown', handleClickOutside);
    }, [isColorPickerOpen]);

    useEffect(() => {
        if (!isColorPickerOpen || !colorButtonRef.current) return;
        const rect = colorButtonRef.current.getBoundingClientRect();
        const popoverWidth = 192;
        const left = Math.min(Math.max(8, rect.left), window.innerWidth - popoverWidth - 8);
        const top = rect.bottom + 8;
        setColorPickerStyle({ top, left });
    }, [isColorPickerOpen]);

    const presetColors = ['#FFFFFF', '#000000', '#FF3B30', '#FF9500', '#FFCC00', '#34C759', '#007AFF', '#5AC8FA', '#5856D6', '#AF52DE', '#FF2D55', '#8E8E93'];
    const normalizeHex = (value: string) => {
        const trimmed = value.trim();
        if (!trimmed) return '';
        return trimmed.startsWith('#') ? trimmed : `#${trimmed}`;
    };
    const isValidHex = (value: string) => /^#([0-9a-fA-F]{6}|[0-9a-fA-F]{3})$/.test(value);
    const applyHex = (value: string) => {
        const normalized = normalizeHex(value).toUpperCase();
        if (isValidHex(normalized)) {
            setColor(normalized);
            return normalized;
        }
        return '';
    };

    const handleSelectImage = async () => {
        try {
            if (window.runtime?.OpenFileDialog) {
                const res = await window.runtime.OpenFileDialog({
                    title: '选择水印图片',
                    canChooseFiles: true,
                    canChooseDirectories: false,
                    allowsMultipleSelection: false,
                    filters: [{
                        DisplayName: "Images",
                        Pattern: "*.jpg;*.jpeg;*.png;*.webp;*.gif;*.bmp;*.tiff;*.tif;*.svg"
                    }]
                } as any);
                const picked = Array.isArray(res) ? res[0] : res;
                if (typeof picked === 'string' && picked) {
                    setImagePath(picked);
                }
            }
        } catch (e) {
            console.error(e);
        }
    };
    const imageLabel = imagePath ? imagePath.replace(/\\/g, '/').split('/').pop() || imagePath : '点击上传水印图';
    const tileGapValue = Math.max(0, Math.round((margin.x + margin.y) / 2));

    return (
        <div className="space-y-6">
            <div className="grid grid-cols-1 lg:grid-cols-4 gap-6">
                <div className="space-y-5">
                    <div className="space-y-2">
                        <label className="text-sm font-medium text-gray-700 dark:text-gray-300">水印来源</label>
                        <SegmentedControl options={['文字', '图片']} value={type} onChange={setType} />
                    </div>

                    {type === '文字' ? (
                        <div className="space-y-3 animate-enter">
                            <input type="text" value={text} onChange={e => setText(e.target.value)} placeholder="© ImageFlow Pro" className="w-full px-3 py-2.5 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm focus:outline-none focus:ring-2 focus:ring-[#007AFF]/50 focus:border-[#007AFF] dark:text-white transition-all" />
                            <div className="flex gap-2 items-center">
                                 <div ref={colorPickerRef} className="relative">
                                     <button
                                         ref={colorButtonRef}
                                         type="button"
                                         onClick={() => setIsColorPickerOpen(prev => !prev)}
                                         className="w-10 h-10 rounded-lg border-2 border-white/20 shadow-sm shrink-0 cursor-pointer bg-transparent"
                                         style={{ backgroundColor: color }}
                                         aria-label="文字颜色"
                                         title="文字颜色"
                                     />
                                     {isColorPickerOpen && colorPickerStyle && createPortal(
                                         <div
                                             ref={colorPopoverRef}
                                             className="fixed z-[9999] w-48 rounded-xl border border-gray-200 dark:border-white/10 bg-white dark:bg-[#2C2C2E] shadow-xl p-3 animate-enter"
                                             style={colorPickerStyle}
                                         >
                                             <div className="grid grid-cols-6 gap-1.5">
                                                 {presetColors.map((hex) => (
                                                     <button
                                                         key={hex}
                                                         type="button"
                                                         onClick={() => {
                                                             setColor(hex);
                                                             setColorInput(hex);
                                                             setIsColorPickerOpen(false);
                                                         }}
                                                         className={`w-6 h-6 rounded-full border transition-all ${color.toUpperCase() === hex ? 'border-[#007AFF] ring-2 ring-[#007AFF]/30' : 'border-gray-200 dark:border-white/10 hover:scale-105'}`}
                                                         style={{ backgroundColor: hex }}
                                                         aria-label={`选择颜色 ${hex}`}
                                                     />
                                                 ))}
                                             </div>
                                             <div className="mt-3 flex items-center gap-2">
                                                 <div
                                                     className="w-7 h-7 rounded-md border border-gray-200 dark:border-white/10"
                                                     style={{ backgroundColor: color }}
                                                 />
                                                 <input
                                                     type="text"
                                                     value={colorInput}
                                                     onChange={(e) => {
                                                         const next = e.target.value;
                                                         setColorInput(next);
                                                         applyHex(next);
                                                     }}
                                                     onBlur={() => {
                                                         const applied = applyHex(colorInput);
                                                         if (!applied) setColorInput(color);
                                                     }}
                                                     placeholder="#FFFFFF"
                                                    className="flex-1 px-3 py-2 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-xs font-mono focus:outline-none focus:ring-2 focus:ring-[#007AFF]/50 focus:border-[#007AFF] dark:text-white transition-all"
                                                 />
                                             </div>
                                         </div>,
                                         document.body
                                     )}
                                  </div>
                                  <div className="flex-1">
                                      <CustomSelect 
                                         label="字体"
                                         options={fontOptions} 
                                         value={font} 
                                         onChange={setFont} 
                                      />
                                  </div>
                             </div>
                             <div className="space-y-1.5">
                                 <Switch label="启用系统字体 (Windows)" checked={useSystemFonts} onChange={setUseSystemFonts} />
                                 {useSystemFonts && (
                                     <div className="text-xs text-gray-500">
                                         {isSystemFontsLoading
                                             ? '正在读取系统字体...'
                                             : systemFontsCount > 0
                                                 ? `已读取 ${systemFontsCount} 个字体文件`
                                                 : '未检测到系统字体或读取失败'}
                                     </div>
                                 )}
                             </div>
                         </div>
                     ) : (
                         <div className="w-full h-28 rounded-xl bg-gray-50 dark:bg-white/5 border-2 border-dashed border-gray-200 dark:border-white/10 flex flex-col items-center justify-center text-sm text-gray-500 cursor-pointer hover:bg-gray-100 dark:hover:bg-white/10 transition-colors animate-enter" onClick={handleSelectImage}>
                             <Icon name="Upload" size={20} className="mb-2 opacity-50" />
                             <span className="text-xs text-gray-500">{imageLabel}</span>
                        </div>
                    )}
                </div>
                <div className="space-y-[18px]">
                    <StyledSlider label="不透明度" value={opacity} onChange={setOpacity} unit="%" />
                    <StyledSlider label="尺寸缩放" value={size} onChange={setSize} unit="%" />
                    <StyledSlider label="旋转角度" value={rotate} min={-180} max={180} onChange={setRotate} unit="°" />
                </div>
                <div className="flex flex-col gap-4 min-w-0">
                    <div className="space-y-2 w-full">
                        <label className="text-xs font-medium text-gray-500 uppercase tracking-wider block text-left">锚点位置</label>
                        <div className="flex justify-center">
                            <PositionGrid value={position} onChange={setPosition} />
                        </div>
                    </div>
                    <CustomSelect label="混合模式" options={['正常', '正片叠底 (Multiply)', '滤色 (Screen)', '叠加 (Overlay)', '柔光 (Soft Light)']} value={blendMode} onChange={setBlendMode} />
                </div>
                <div className="flex flex-col gap-4 min-w-0">
                    <div className="space-y-2 w-full">
                        {tiled ? (
                            <>
                                <label className="text-xs font-medium text-gray-500 uppercase tracking-wider block text-left">平铺间距</label>
                                <StyledSlider
                                    value={tileGapValue}
                                    min={0}
                                    max={240}
                                    unit="px"
                                    onChange={(val) => setMargin({ x: val, y: val })}
                                />
                            </>
                        ) : (
                            <>
                                <label className="text-xs font-medium text-gray-500 uppercase tracking-wider block text-left">边距偏移</label>
                                <div className="grid grid-cols-1 gap-2 justify-items-center">
                                    <div className="relative w-24">
                                        <span className="absolute left-2 top-1/2 -translate-y-1/2 text-xs text-gray-400 pointer-events-none">X</span>
                                        <input 
                                            type="number" 
                                            value={margin.x} 
                                            onChange={e => setMargin({...margin, x: Number(e.target.value)})} 
                                            className="w-full pl-6 pr-2 py-2 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm outline-none focus:border-[#007AFF] transition-colors dark:text-white text-right" 
                                        />
                                    </div>
                                    <div className="relative w-24">
                                        <span className="absolute left-2 top-1/2 -translate-y-1/2 text-xs text-gray-400 pointer-events-none">Y</span>
                                        <input 
                                            type="number" 
                                            value={margin.y} 
                                            onChange={e => setMargin({...margin, y: Number(e.target.value)})} 
                                            className="w-full pl-6 pr-2 py-2 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm outline-none focus:border-[#007AFF] transition-colors dark:text-white text-right" 
                                        />
                                    </div>
                                </div>
                            </>
                        )}
                    </div>
                    <Switch label="添加投影 (Shadow)" checked={shadow} onChange={setShadow} />
                    <Switch label="全屏水印 (平铺)" checked={tiled} onChange={setTiled} />
                </div>
            </div>
        </div>
    );
});

type AdjustSettingsProps = {
    exposure: number;
    setExposure: (v: number) => void;
    contrast: number;
    setContrast: (v: number) => void;
    saturation: number;
    setSaturation: (v: number) => void;
    sharpness: number;
    setSharpness: (v: number) => void;
    vibrance: number;
    setVibrance: (v: number) => void;
    hue: number;
    setHue: (v: number) => void;
    showCrop?: boolean;
    cropRatio?: string;
    setCropRatio?: (v: string) => void;
    rotate?: number;
    setRotate?: (v: number) => void;
    flipH?: boolean;
    setFlipH?: (v: boolean) => void;
};

type AdjustCropControlsProps = {
    cropRatio: string;
    setCropRatio: (v: string) => void;
    rotate: number;
    setRotate: (v: number) => void;
    flipH: boolean;
    setFlipH: (v: boolean) => void;
};

const AdjustCropControls = memo(({
    cropRatio,
    setCropRatio,
    rotate,
    setRotate,
    flipH,
    setFlipH,
}: AdjustCropControlsProps) => (
    <div className="space-y-2">
        <label className="text-sm font-medium text-gray-700 dark:text-gray-300">裁剪与旋转</label>
        <div className="flex flex-wrap gap-2">
            {['自由', '1:1', '4:3', '16:9', '9:16', '3:2'].map(r => {
                const active = cropRatio === r;
                return (
                    <button
                        key={r}
                        onClick={() => setCropRatio(r)}
                        className={`px-3 py-1.5 rounded-lg border text-xs font-medium whitespace-nowrap transition-colors ${
                            active
                                ? 'border-[#007AFF] text-[#007AFF] bg-[#007AFF]/10'
                                : 'border-gray-200 dark:border-white/10 text-gray-600 dark:text-gray-300 bg-white dark:bg-white/5 hover:border-[#007AFF] hover:text-[#007AFF]'
                        }`}
                    >
                        {r}
                    </button>
                );
            })}
        </div>
        <div className="grid grid-cols-2 gap-2 mt-2">
            <button
                onClick={() => setRotate((rotate + 90) % 360)}
                className="flex items-center justify-center gap-2 py-2 rounded-xl border border-gray-200 dark:border-white/10 hover:bg-gray-50 dark:hover:bg-white/5 transition-all text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-[#2C2C2E]"
            >
                <Icon name="RotateCw" size={16} /> 向右旋转 90°
            </button>
            <button
                onClick={() => setFlipH(!flipH)}
                className={`flex items-center justify-center gap-2 py-2 rounded-xl border transition-all text-sm font-medium ${
                    flipH
                        ? 'border-[#007AFF]/60 text-[#007AFF] bg-[#007AFF]/10'
                        : 'border-gray-200 dark:border-white/10 text-gray-700 dark:text-gray-300 bg-white dark:bg-[#2C2C2E] hover:bg-gray-50 dark:hover:bg-white/5'
                }`}
            >
                <span className="scale-x-[-1] inline-block"><Icon name="RotateCw" size={16} /></span> 水平翻转
            </button>
        </div>
    </div>
));

const AdjustSettings = memo(({
    exposure,
    setExposure,
    contrast,
    setContrast,
    saturation,
    setSaturation,
    sharpness,
    setSharpness,
    vibrance,
    setVibrance,
    hue,
    setHue,
    showCrop = true,
    cropRatio,
    setCropRatio,
    rotate,
    setRotate,
    flipH,
    setFlipH,
}: AdjustSettingsProps) => {
    const canShowCrop = showCrop
        && typeof cropRatio === 'string'
        && typeof setCropRatio === 'function'
        && typeof rotate === 'number'
        && typeof setRotate === 'function'
        && typeof flipH === 'boolean'
        && typeof setFlipH === 'function';

    return (
        <div className="space-y-4">
            {canShowCrop && (
                <AdjustCropControls
                    cropRatio={cropRatio}
                    setCropRatio={setCropRatio}
                    rotate={rotate}
                    setRotate={setRotate}
                    flipH={flipH}
                    setFlipH={setFlipH}
                />
            )}

            <div className="pt-3 border-t border-gray-100 dark:border-white/5">
                 <label className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-4 flex items-center gap-2">
                    <Icon name="Palette" size={16} className="text-[#007AFF]" />
                    专业调色
                 </label>
                 <div className="w-full">
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <StyledSlider label="曝光度" value={exposure} min={-100} max={100} onChange={setExposure} className="space-y-2" />
                    <StyledSlider label="对比度" value={contrast} min={-100} max={100} onChange={setContrast} className="space-y-2" />
                    <StyledSlider label="自然饱和度" value={vibrance} min={-100} max={100} onChange={setVibrance} className="space-y-2" />
                    <StyledSlider label="饱和度" value={saturation} min={-100} max={100} onChange={setSaturation} className="space-y-2" />
                    <StyledSlider label="色相偏移" value={hue} min={-180} max={180} onChange={setHue} unit="°" className="space-y-2" />
                    <StyledSlider label="锐化/模糊" value={sharpness} min={-100} max={100} onChange={setSharpness} className="space-y-2" />
                    </div>
                 </div>
            </div>
        </div>
    );
});

const FILTER_LABELS = [
    '原图', '鲜艳', '黑白', '复古', '冷调', '暖阳', '胶片', '赛博',
    '清新', '日系', 'Lomo', 'HDR', '褪色', '磨砂', '电影', '拍立得',
    '夕阳', '海蓝', '森系', '紫雾', '琥珀', '北欧', '旧照片', '黑金',
    '高调', '低调', '雾霭', '霓虹', '哑光', '冰感', '咖啡', '焦糖',
    '青橙', '银盐', '清锐', '低对比'
];
const FILTER_PRESETS = [
    'none', 'vivid', 'bw', 'retro', 'cool', 'warm', 'film', 'cyber',
    'fresh', 'japan', 'lomo', 'hdr', 'fade', 'frosted', 'cinema', 'polaroid',
    'sunset', 'ocean', 'forest', 'purple', 'amber', 'nordic', 'oldphoto', 'noir',
    'highkey', 'lowkey', 'haze', 'neon', 'matte', 'ice', 'coffee', 'caramel',
    'teal_orange', 'silver', 'crisp', 'low_contrast'
];

type FilterSettingsProps = {
    intensity: number;
    setIntensity: (v: number) => void;
    grain: number;
    setGrain: (v: number) => void;
    vignette: number;
    setVignette: (v: number) => void;
    selected: number;
    setSelected: (v: number) => void;
    previewSrc: string;
    getPreviewFilter: (index: number) => string;
};

type FilterControlsProps = {
    intensity: number;
    setIntensity: (v: number) => void;
    grain: number;
    setGrain: (v: number) => void;
    vignette: number;
    setVignette: (v: number) => void;
    footer?: React.ReactNode;
};

const FilterControls = memo(({
    intensity,
    setIntensity,
    grain,
    setGrain,
    vignette,
    setVignette,
    footer,
}: FilterControlsProps) => (
    <div className="bg-white dark:bg-[#2C2C2E] rounded-3xl p-5 shadow-sm border border-gray-100 dark:border-white/5 flex flex-col gap-4">
        <div className="grid grid-cols-1 gap-4">
            <StyledSlider label="滤镜强度" value={intensity} onChange={setIntensity} unit="%" className="space-y-2" />
            <StyledSlider label="颗粒感 (Grain)" value={grain} onChange={setGrain} className="space-y-2" />
            <StyledSlider label="暗角 (Vignette)" value={vignette} onChange={setVignette} className="space-y-2" />
        </div>
        {footer && (
            <div className="pt-3 border-t border-gray-100 dark:border-white/5">
                {footer}
            </div>
        )}
    </div>
));

const FilterSettings = memo(({
    intensity,
    setIntensity,
    grain,
    setGrain,
    vignette,
    setVignette,
    selected,
    setSelected,
    previewSrc,
    getPreviewFilter,
}: FilterSettingsProps) => {
    const hasPreview = Boolean(previewSrc);

    return (
        <div className="flex flex-col h-full min-h-0">
            <div className="flex-1 space-y-3 min-h-0 flex flex-col">
                <div className="flex items-center justify-between shrink-0">
                    <label className="text-sm font-medium text-gray-700 dark:text-gray-300">滤镜库 (LUTs)</label>
                    <button
                        onClick={() => {
                            setSelected(0);
                            setIntensity(80);
                            setGrain(0);
                            setVignette(0);
                        }}
                        className="text-xs text-[#007AFF] hover:underline font-medium"
                    >
                        重置效果
                    </button>
                </div>
                <div className="flex-1 min-h-0 overflow-y-auto no-scrollbar pr-1 pb-3">
                    <div
                        className="grid gap-1.5 content-start w-full"
                        style={{ gridTemplateColumns: 'repeat(auto-fit, minmax(88px, 1fr))' }}
                    >
                        {FILTER_LABELS.map((f, i) => (
                            <button 
                                key={i} 
                                onClick={() => setSelected(i)}
                                className={`
                                group relative flex flex-col items-center gap-1 p-1 rounded-md border transition-all duration-200 active:scale-95
                                ${selected === i 
                                    ? 'bg-[#007AFF]/5 border-[#007AFF] ring-1 ring-[#007AFF]/20 shadow-sm' 
                                    : 'bg-gray-50 dark:bg-white/5 border-transparent hover:border-gray-200 dark:hover:border-white/10 hover:shadow-sm'}
                            `}>
                                <div className={`w-full aspect-square rounded-md ${i===0 ? 'bg-gray-200 dark:bg-white/10' : 'bg-gradient-to-br from-gray-200 to-gray-300 dark:from-white/5 dark:to-white/10'} overflow-hidden relative shadow-inner`}>
                                    {hasPreview ? (
                                        <img
                                            src={previewSrc}
                                            alt={f}
                                            className="w-full h-full object-cover"
                                            style={{ filter: getPreviewFilter(i) || 'none' }}
                                            loading="lazy"
                                        />
                                    ) : (
                                        <div className={`absolute inset-0 opacity-20 ${i===1 ? 'bg-blue-500 mix-blend-overlay' : ''} ${i%2===0 && i!==0 ? 'bg-yellow-600 mix-blend-multiply' : ''} ${i%3===0 && i!==0 ? 'bg-purple-600 mix-blend-screen' : ''}`}></div>
                                    )}
                                </div>
                                <span className={`text-[12px] font-medium w-full text-center leading-tight break-words min-h-[2.2em] ${selected === i ? 'text-[#007AFF]' : 'text-gray-500 dark:text-gray-400 group-hover:text-gray-700 dark:group-hover:text-gray-200'}`}>
                                    {f}
                                </span>
                                {selected === i && <div className="absolute top-0.5 right-0.5 w-1.5 h-1.5 bg-[#007AFF] rounded-full shadow-sm animate-enter"></div>}
                            </button>
                        ))}
                    </div>
                </div>
            </div>
        </div>
    );
});

type PdfSettingsProps = {
    fileName: string;
    setFileName: (v: string) => void;
    size: string;
    setSize: (v: string) => void;
    layout: string;
    setLayout: (v: string) => void;
    fit: string;
    setFit: (v: string) => void;
    marginMm: number;
    setMarginMm: (v: number) => void;
    compression: string;
    setCompression: (v: string) => void;
    title: string;
    setTitle: (v: string) => void;
    author: string;
    setAuthor: (v: string) => void;
};

const PdfSettings = memo(({
    fileName,
    setFileName,
    size,
    setSize,
    layout,
    setLayout,
    fit,
    setFit,
    marginMm,
    setMarginMm,
    compression,
    setCompression,
    title,
    setTitle,
    author,
    setAuthor,
}: PdfSettingsProps) => {

    return (
         <div className="space-y-6">
            <CustomSelect 
                label="纸张尺寸" 
                options={[
                    'A0 (841 x 1189 mm)',
                    'A1 (594 x 841 mm)',
                    'A2 (420 x 594 mm)',
                    'A3 (297 x 420 mm)',
                    'A4 (210 x 297 mm)',
                    'A5 (148 x 210 mm)',
                    'A6 (105 x 148 mm)',
                    'B4 (250 x 353 mm)',
                    'B5 (176 x 250 mm)',
                    'B6 (125 x 176 mm)',
                    'Letter (8.5 x 11 in)',
                    'Legal (8.5 x 14 in)',
                    'Tabloid (11 x 17 in)',
                    'Ledger (17 x 11 in)',
                ]} 
                value={size}
                onChange={setSize}
            />
            
            <div className="space-y-2">
                <label className="text-sm font-medium text-gray-700 dark:text-gray-300">页面方向</label>
                <SegmentedControl options={['纵向', '横向']} value={layout} onChange={setLayout} />
            </div>

            <CustomSelect 
                label="图片填充方式" 
                options={['适应页面 (保持比例)', '充满页面 (可能裁剪)', '原始大小 (居中)']} 
                value={fit}
                onChange={setFit}
            />

            <div className="space-y-2">
                <label className="text-sm font-medium text-gray-700 dark:text-gray-300">页面边距 (mm)</label>
                <input
                    type="number"
                    min={0}
                    max={50}
                    step={1}
                    value={marginMm}
                    onChange={(e) => setMarginMm(Number(e.target.value || 0))}
                    className="w-full px-3 py-2.5 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm outline-none dark:text-white"
                />
            </div>

            <div className="space-y-2">
                <label className="text-sm font-medium text-gray-700 dark:text-gray-300">输出文件名</label>
                <div className="flex items-center gap-2">
                    <input
                        type="text"
                        value={fileName}
                        onChange={(e) => setFileName(e.target.value)}
                        placeholder="自动生成"
                        className="w-full px-3 py-2.5 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm outline-none dark:text-white"
                    />
                    <span className="text-xs text-gray-400 shrink-0">.pdf</span>
                </div>
            </div>

            <div className="pt-4 border-t border-gray-100 dark:border-white/5 space-y-4">
                 <div className="space-y-3">
                    <label className="text-sm font-medium text-gray-700 dark:text-gray-300">元数据</label>
                    <input type="text" value={title} onChange={(e) => setTitle(e.target.value)} placeholder="文档标题" className="w-full px-3 py-2.5 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm outline-none dark:text-white" />
                    <input type="text" value={author} onChange={(e) => setAuthor(e.target.value)} placeholder="作者" className="w-full px-3 py-2.5 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm outline-none dark:text-white" />
                 </div>
                <div className="h-px bg-gray-100 dark:bg-white/5" />
                <div className="space-y-2">
                    <label className="text-sm font-medium text-gray-700 dark:text-gray-300">PDF 压缩</label>
                    <SegmentedControl options={['不压缩', '轻度', '标准', '强力']} value={compression} onChange={setCompression} />
                </div>
                <div className="space-y-2">
                    <label className="text-sm font-medium text-gray-700 dark:text-gray-300">文档加密 (可选)</label>
                    <input type="password" placeholder="留空则不加密" className="w-full px-3 py-2.5 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm focus:outline-none focus:ring-2 focus:ring-[#007AFF]/50 focus:border-[#007AFF] dark:text-white transition-all" />
                </div>
            </div>
        </div>
    );
});

type GifSettingsProps = {
    mode: string;
    setMode: (v: string) => void;
    exportFormat: string;
    setExportFormat: (v: string) => void;
    speedPercent: number;
    setSpeedPercent: (v: number) => void;
    sourceType: 'gif' | 'images' | 'mixed' | 'empty';
    buildFps: number;
    setBuildFps: (v: number) => void;
};

const GifSettings = memo(({
    mode,
    setMode,
    exportFormat,
    setExportFormat,
    speedPercent,
    setSpeedPercent,
    sourceType,
    buildFps,
    setBuildFps,
}: GifSettingsProps) => {
    if (sourceType === 'images') {
        return (
            <div className="space-y-6">
                <div className="space-y-2">
                    <label className="text-sm font-medium text-gray-700 dark:text-gray-300">合成 GIF</label>
                    <div className="text-xs text-gray-500 dark:text-gray-400">
                        PNG/JPG 直接合成，其它格式会先转为 JPG。
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
                    options={['导出', '倒放', '修改帧率']}
                    value={mode}
                    onChange={setMode}
                />
            </div>

            {mode === '导出' && (
                <div className="space-y-3 animate-enter">
                    <CustomSelect
                        label="导出帧格式"
                        options={['PNG', 'JPG']}
                        value={exportFormat}
                        onChange={setExportFormat}
                    />
                    <div className="text-xs text-gray-500 dark:text-gray-400">
                        GIF 输入导出帧，图片输入合成 GIF（非 PNG/JPG 会先转为 JPG）。
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

            {sourceType === 'mixed' && (
                <div className="text-xs text-red-500">
                    请只选择 GIF 或图片序列，混合输入会导致操作失败。
                </div>
            )}

        </div>
    );
});

type InfoSettingsProps = {
    filePath: string;
    info: any | null;
    onExportJSON: () => void;
    onClearPrivacy: () => void;
    onEditMetadata: (key: string, value: any) => Promise<void>;
};

const META_GROUP_LABELS: Record<string, string> = {
    Basic: '基础信息',
    Image: '图像',
    EXIF: 'EXIF',
    Exif: 'EXIF',
    GPS: 'GPS',
    Interop: '互操作',
    IFD0: '主图像',
    IFD1: '缩略图',
    '0th': '主图像',
    '1st': '缩略图',
    Thumbnail: '缩略图',
    PNG: 'PNG',
    JPEG: 'JPEG',
    TIFF: 'TIFF',
    XMP: 'XMP',
    IPTC: 'IPTC',
    MakerNote: '厂商注释',
    exifread: 'EXIF',
    piexif: 'EXIF',
    extra: '扩展信息',
};

const META_TAG_LABELS: Record<string, string> = {
    Path: '路径',
    Size: '大小',
    Width: '宽度',
    Height: '高度',
    Dimensions: '尺寸',
    DPI: 'DPI',
    Make: '相机厂商',
    Model: '相机型号',
    Software: '软件',
    Artist: '作者',
    Copyright: '版权',
    ImageDescription: '图像描述',
    Orientation: '方向',
    XResolution: '水平分辨率',
    YResolution: '垂直分辨率',
    ResolutionUnit: '分辨率单位',
    DateTime: '修改时间',
    DateTimeOriginal: '拍摄时间',
    DateTimeDigitized: '数字化时间',
    SubSecTime: '亚秒时间',
    SubSecTimeOriginal: '亚秒拍摄时间',
    SubSecTimeDigitized: '亚秒数字化时间',
    ExifVersion: 'EXIF 版本',
    FlashpixVersion: 'Flashpix 版本',
    ColorSpace: '色彩空间',
    ComponentsConfiguration: '分量配置',
    CompressedBitsPerPixel: '压缩位深',
    ExposureTime: '快门速度',
    FNumber: '光圈',
    ExposureProgram: '曝光程序',
    ExposureBiasValue: '曝光补偿',
    ExposureMode: '曝光模式',
    ShutterSpeedValue: '快门速度值',
    ApertureValue: '光圈值',
    MaxApertureValue: '最大光圈值',
    BrightnessValue: '亮度值',
    ISOSpeedRatings: 'ISO',
    PhotographicSensitivity: 'ISO',
    SensitivityType: '感光度类型',
    FocalLength: '焦距',
    FocalLengthIn35mmFilm: '35mm 等效焦距',
    LensMake: '镜头厂商',
    LensModel: '镜头型号',
    LensSpecification: '镜头规格',
    WhiteBalance: '白平衡',
    MeteringMode: '测光模式',
    LightSource: '光源',
    Flash: '闪光灯',
    SceneType: '场景类型',
    SceneCaptureType: '场景捕捉类型',
    SensingMethod: '感光方式',
    FileSource: '文件来源',
    CustomRendered: '渲染设置',
    DigitalZoomRatio: '数字变焦',
    GainControl: '增益控制',
    Contrast: '对比度',
    Saturation: '饱和度',
    Sharpness: '锐度',
    SubjectDistance: '主体距离',
    SubjectDistanceRange: '主体距离范围',
    SubjectArea: '主体区域',
    ImageWidth: '图像宽度',
    ImageLength: '图像高度',
    ExifImageWidth: '图像宽度',
    ExifImageLength: '图像高度',
    PixelXDimension: '像素宽度',
    PixelYDimension: '像素高度',
    BitsPerSample: '每样本位数',
    SamplesPerPixel: '每像素采样数',
    Compression: '压缩方式',
    PlanarConfiguration: '平面配置',
    YCbCrSubSampling: '色度采样',
    YCbCrPositioning: '色度位置',
    GPSLatitude: '纬度',
    GPSLatitudeRef: '纬度参考',
    GPSLongitude: '经度',
    GPSLongitudeRef: '经度参考',
    GPSAltitude: '海拔',
    GPSAltitudeRef: '海拔参考',
    GPSTimeStamp: 'GPS 时间',
    GPSDateStamp: 'GPS 日期',
    GPSMapDatum: '地理基准',
    GPSDOP: 'GPS 精度',
    GPSSpeed: '速度',
    GPSSpeedRef: '速度单位',
    GPSTrack: '航向',
    GPSTrackRef: '航向参考',
    GPSImgDirection: '拍摄方向',
    GPSImgDirectionRef: '方向参考',
    GPSProcessingMethod: '定位方式',
    GPSAreaInformation: '区域信息',
    GPSDestLatitude: '目标纬度',
    GPSDestLatitudeRef: '目标纬度参考',
    GPSDestLongitude: '目标经度',
    GPSDestLongitudeRef: '目标经度参考',
    GPSDestBearing: '目标方位',
    GPSDestBearingRef: '目标方位参考',
    GPSDestDistance: '目标距离',
    GPSDestDistanceRef: '目标距离单位',
    GPSDifferential: '差分校正',
    GPSHPositioningError: '水平定位误差',
    UserComment: '用户注释',
    XPTitle: '标题',
    XPSubject: '主题',
    XPComment: '备注',
    XPKeywords: '关键词',
    XPAuthor: '作者',
    ThumbnailOffset: '缩略图偏移',
    ThumbnailLength: '缩略图长度',
    ThumbnailImageWidth: '缩略图宽度',
    ThumbnailImageLength: '缩略图高度',
    thumbnail_bytes: '缩略图数据',
};

const META_SOURCE_GROUPS = new Set(['exifread', 'piexif']);

const splitMetaKey = (key: string) => {
    const colonIndex = key.indexOf(':');
    if (colonIndex > 0) {
        return { group: key.slice(0, colonIndex), tag: key.slice(colonIndex + 1) };
    }
    const spaceIndex = key.indexOf(' ');
    if (spaceIndex > 0) {
        const maybeGroup = key.slice(0, spaceIndex);
        if (META_GROUP_LABELS[maybeGroup]) {
            return { group: maybeGroup, tag: key.slice(spaceIndex + 1) };
        }
    }
    return { group: '', tag: key };
};

const translateMetaTag = (tag: string) => {
    const trimmed = tag.trim();
    if (!trimmed) return '';
    if (META_TAG_LABELS[trimmed]) return META_TAG_LABELS[trimmed];
    const { group, tag: inner } = splitMetaKey(trimmed);
    if (group) {
        const groupLabel = META_GROUP_LABELS[group] || group;
        const innerLabel = META_TAG_LABELS[inner] || inner;
        return `${groupLabel}：${innerLabel}`;
    }
    return trimmed;
};

const translateMetaLabel = (rawKey: string) => {
    const trimmed = rawKey.trim();
    if (!trimmed) return rawKey;
    const { group, tag } = splitMetaKey(trimmed);
    const translatedTag = translateMetaTag(tag);
    if (group && META_SOURCE_GROUPS.has(group)) {
        return translatedTag || trimmed;
    }
    const groupLabel = META_GROUP_LABELS[group];
    if (!groupLabel) return translatedTag || trimmed;
    return `${groupLabel}：${translatedTag || tag}`;
};

const InfoSettings = memo(({
    filePath,
    info,
    onExportJSON,
    onClearPrivacy,
    onEditMetadata,
}: InfoSettingsProps) => {
    const [editingKey, setEditingKey] = useState<string | null>(null);
    const [editingValue, setEditingValue] = useState('');

    const buildRowsFromMap = (data?: Record<string, any>, editableKeys?: Set<string>) => {
        if (!data) return [];
        return Object.keys(data)
            .sort((a, b) => a.localeCompare(b))
            .map((key) => ({
                rawKey: key,
                label: translateMetaLabel(key),
                value: String(data[key]),
                editKey: key.startsWith('piexif:') ? key.slice('piexif:'.length) : key,
                editable: Boolean(editableKeys?.has(key.startsWith('piexif:') ? key.slice('piexif:'.length) : key))
                    && !(key.startsWith('piexif:') ? key.slice('piexif:'.length) : key).toLowerCase().includes('thumbnail'),
            }))
            .filter((row) => row.value !== '' && row.value !== 'undefined' && row.value !== 'null');
    };

    const meta = info?.metadata || {};
    const editableKeys = new Set(Object.keys(meta.piexif || {}));
    const parseResolution = (value: any) => {
        if (value === null || value === undefined) return null;
        const text = String(value).trim();
        if (!text) return null;
        const fractionMatch = text.match(/^(-?\d+(?:\.\d+)?)\s*\/\s*(-?\d+(?:\.\d+)?)$/);
        if (fractionMatch) {
            const num = Number(fractionMatch[1]);
            const den = Number(fractionMatch[2]);
            if (Number.isFinite(num) && Number.isFinite(den) && den !== 0) {
                return num / den;
            }
        }
        const nums = text.match(/-?\d+(?:\.\d+)?/g);
        if (!nums || nums.length === 0) return null;
        if (nums.length >= 2 && text.includes(',')) {
            const num = Number(nums[0]);
            const den = Number(nums[1]);
            if (Number.isFinite(num) && Number.isFinite(den) && den !== 0) {
                return num / den;
            }
        }
        const num = Number(nums[0]);
        return Number.isFinite(num) ? num : null;
    };
    const formatDpi = (value: number) => {
        if (!Number.isFinite(value)) return '';
        return Number.isInteger(value) ? `${value}` : `${value.toFixed(2)}`;
    };
    const formatBytes = (value: number) => {
        if (!Number.isFinite(value) || value < 0) return '';
        const units = ['B', 'KB', 'MB', 'GB', 'TB'];
        let size = value;
        let unitIndex = 0;
        while (size >= 1024 && unitIndex < units.length - 1) {
            size /= 1024;
            unitIndex += 1;
        }
        if (unitIndex === 0) {
            return `${value} B`;
        }
        const decimals = size >= 10 ? 1 : 2;
        return `${value} B (${size.toFixed(decimals)} ${units[unitIndex]})`;
    };
    const buildFlatMetadata = () => {
        let merged: Record<string, any> = {};
        if (info?.exif && Object.keys(info.exif).length > 0) {
            merged = { ...(info.exif as Record<string, any>) };
        } else {
            const groups = ['exifread', 'piexif', 'extra'];
            for (const group of groups) {
                const items = meta[group] || {};
                for (const [key, value] of Object.entries(items)) {
                    if (Object.prototype.hasOwnProperty.call(merged, key)) {
                        merged[`${group}:${key}`] = value;
                    } else {
                        merged[key] = value;
                    }
                }
            }
        }

        const basic: Record<string, any> = {};
        if (filePath) {
            basic['Basic:Path'] = filePath;
        }
        if (typeof info?.file_size === 'number') {
            basic['Basic:Size'] = formatBytes(info.file_size);
        }
        if (typeof info?.width === 'number') {
            basic['Basic:Width'] = info.width;
        }
        if (typeof info?.height === 'number') {
            basic['Basic:Height'] = info.height;
        }
        if (typeof info?.width === 'number' && typeof info?.height === 'number' && info.width && info.height) {
            basic['Basic:Dimensions'] = `${info.width}x${info.height}`;
        }

        const dpiFromPng = merged['PNG:DPI'];
        const xRes = merged['Image XResolution'] ?? merged['0th:XResolution'];
        const yRes = merged['Image YResolution'] ?? merged['0th:YResolution'];
        let dpiX = null;
        let dpiY = null;
        if (dpiFromPng) {
            const dpiText = String(dpiFromPng);
            const match = dpiText.match(/(-?\d+(?:\.\d+)?)\s*x\s*(-?\d+(?:\.\d+)?)/i);
            if (match) {
                dpiX = Number(match[1]);
                dpiY = Number(match[2]);
            } else {
                dpiX = parseResolution(dpiFromPng);
                dpiY = parseResolution(dpiFromPng);
            }
        } else {
            dpiX = parseResolution(xRes);
            dpiY = parseResolution(yRes);
        }
        if (dpiX || dpiY) {
            const left = dpiX ? formatDpi(dpiX) : '';
            const right = dpiY ? formatDpi(dpiY) : '';
            basic['Basic:DPI'] = right ? `${left}x${right}` : left;
        }

        return { ...basic, ...merged };
    };
    const flatMeta = buildFlatMetadata();
    let rows = buildRowsFromMap(flatMeta, editableKeys);
    if (!info?.success && info?.error) {
        rows = [{ rawKey: '错误', label: '错误', value: String(info.error), editKey: '', editable: false }];
    }

    const name = filePath ? filePath.replace(/\\/g, '/').split('/').pop() || filePath : '';
    const isEmpty = rows.length === 0;

    const startEdit = (key: string, value: string) => {
        setEditingKey(key);
        setEditingValue(value);
    };

    const commitEdit = async (key: string, value: string) => {
        if (!key) {
            setEditingKey(null);
            return;
        }
        const trimmed = value.trim();
        const payload = trimmed.toLowerCase() == 'null' ? null : trimmed;
        await onEditMetadata(key, payload);
        setEditingKey(null);
    };

    return (
        <div className="h-full flex flex-col">
            <div className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-4 flex items-center justify-between">
                <span>全部元数据</span>
                <span className="text-xs text-gray-400 font-normal">{name || '-'}</span>
            </div>
            <div className="bg-gray-50 dark:bg-white/5 rounded-xl border border-gray-200 dark:border-white/10 overflow-hidden text-sm flex-1 overflow-y-auto no-scrollbar">
                {isEmpty ? (
                    <div className="p-4 text-xs text-gray-500 dark:text-gray-400">
                        暂无可显示的元数据
                    </div>
                ) : (
                    <table className="w-full">
                        <tbody>
                            {rows.map((item, i) => {
                                const isEditing = editingKey === item.rawKey;
                                const isEditable = Boolean(item.editable) && !(item.value || '').startsWith('hex:');
                                return (
                                    <tr key={`${item.rawKey}-${i}`} className="border-b border-gray-100 dark:border-white/5 last:border-0 hover:bg-black/5 dark:hover:bg-white/5 transition-colors">
                                        <td className="py-2.5 px-4 text-gray-500 dark:text-gray-400 w-1/3">{item.label}</td>
                                        <td className="py-2.5 px-4 text-gray-900 dark:text-white font-mono text-xs break-all">
                                            {isEditing ? (
                                                <input
                                                    autoFocus
                                                    value={editingValue}
                                                    onChange={(e) => setEditingValue(e.target.value)}
                                                    onBlur={() => commitEdit(item.editKey, editingValue)}
                                                    onKeyDown={(e) => {
                                                        if (e.key === 'Enter') {
                                                            commitEdit(item.editKey, editingValue);
                                                        } else if (e.key === 'Escape') {
                                                            setEditingKey(null);
                                                        }
                                                    }}
                                                    className="w-full px-2 py-1 rounded bg-white dark:bg-[#1C1C1E] border border-gray-200 dark:border-white/10 text-xs font-mono outline-none"
                                                />
                                            ) : (
                                                <button
                                                    type="button"
                                                    onClick={() => isEditable && startEdit(item.rawKey, item.value)}
                                                    className={`text-left w-full ${isEditable ? 'cursor-text hover:text-[#007AFF]' : 'cursor-default'}`}
                                                >
                                                    {item.value}
                                                </button>
                                            )}
                                        </td>
                                    </tr>
                                );
                            })}
                        </tbody>
                    </table>
                )}
            </div>
            <div className="mt-4 flex gap-3 shrink-0">
                 <button onClick={onExportJSON} disabled={!info?.success} className="flex-1 py-2.5 rounded-xl border border-gray-200 dark:border-white/10 text-sm font-medium text-gray-600 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-white/5 transition-colors disabled:opacity-50 disabled:cursor-not-allowed">导出 JSON</button>
                 <button onClick={onClearPrivacy} disabled={!filePath} className="flex-1 py-2.5 rounded-xl border border-gray-200 dark:border-white/10 text-sm font-medium text-red-500 hover:bg-red-50 dark:hover:bg-red-500/10 hover:border-red-200 transition-colors disabled:opacity-50 disabled:cursor-not-allowed">清除隐私信息</button>
            </div>
        </div>
    );
});

const DetailView: React.FC<DetailViewProps> = ({ id, onBack, isActive = true, onTaskFailure }) => {
    const feature = FEATURES.find(f => f.id === id);
    if (!feature) return null;

    const isInfo = id === 'info';
    const isAdjustOrFilter = id === 'adjust' || id === 'filter';
    const isWatermark = id === 'watermark';
    const isPreviewFeature = isAdjustOrFilter || isWatermark;
    const [dropResult, setDropResult] = useState<ExpandDroppedPathsResult | null>(null);
    const [outputDir, setOutputDir] = useState('');
    const [isProcessing, setIsProcessing] = useState(false);
    const [progress, setProgress] = useState(0);
    const [lastMessage, setLastMessage] = useState<string>('');
    const [outputSettings, setOutputSettings] = useState<OutputSettings>(defaultOutputSettings);

    const [convFormat, setConvFormat] = useState('JPG');
    const [convQuality, setConvQuality] = useState(80);
    const [convCompressLevel, setConvCompressLevel] = useState(6); // 0-9 for PNG
    const [convIcoSizes, setConvIcoSizes] = useState<number[]>([16, 32, 48, 64, 128, 256, 512, 1024]);
    const [convResizeMode, setConvResizeMode] = useState('原图尺寸');
    const [convScalePercent, setConvScalePercent] = useState(100);
    const [convFixedWidth, setConvFixedWidth] = useState(0);
    const [convFixedHeight, setConvFixedHeight] = useState(0);
    const [convLongEdge, setConvLongEdge] = useState(1920);
    const [convKeepMetadata, setConvKeepMetadata] = useState(false);
    const [convMaintainAR, setConvMaintainAR] = useState(true);
    const [convOverwriteSource, setConvOverwriteSource] = useState(false);

    const [compMode, setCompMode] = useState('标准');
    const [compTargetSize, setCompTargetSize] = useState(false);
    const [compTargetSizeKB, setCompTargetSizeKB] = useState(500);
    const [compEngine, setCompEngine] = useState('自动 (推荐)');
    const [compOverwriteSource, setCompOverwriteSource] = useState(false);
    const [pdfSize, setPdfSize] = useState('A4 (210 x 297 mm)');
    const [pdfLayout, setPdfLayout] = useState('纵向');
    const [pdfFit, setPdfFit] = useState('适应页面 (保持比例)');
    const [pdfMarginMm, setPdfMarginMm] = useState(25.4);
    const [pdfCompression, setPdfCompression] = useState('不压缩');
    const [pdfFileName, setPdfFileName] = useState('');
    const [pdfTitle, setPdfTitle] = useState('');
    const [pdfAuthor, setPdfAuthor] = useState('');
    const [adjustExposure, setAdjustExposure] = useState(0);
    const [adjustContrast, setAdjustContrast] = useState(0);
    const [adjustSaturation, setAdjustSaturation] = useState(0);
    const [adjustSharpness, setAdjustSharpness] = useState(0);
    const [adjustVibrance, setAdjustVibrance] = useState(0);
    const [adjustHue, setAdjustHue] = useState(0);
    const [adjustRotate, setAdjustRotate] = useState(0);
    const [adjustFlipH, setAdjustFlipH] = useState(false);
    const [adjustFlipV, setAdjustFlipV] = useState(false);
    const [adjustCropRatio, setAdjustCropRatio] = useState('自由');
    const [filterIntensity, setFilterIntensity] = useState(80);
    const [filterGrain, setFilterGrain] = useState(0);
    const [filterVignette, setFilterVignette] = useState(0);
    const [filterSelected, setFilterSelected] = useState(0);
    const [gifMode, setGifMode] = useState('导出');
    const [gifExportFormat, setGifExportFormat] = useState('PNG');
    const [gifSpeedPercent, setGifSpeedPercent] = useState(100);
    const [gifBuildFps, setGifBuildFps] = useState(10);
    const [infoFilePath, setInfoFilePath] = useState('');
    const [infoPreview, setInfoPreview] = useState<any | null>(null);
    const [previewPath, setPreviewPath] = useState('');
    const [previewDataUrl, setPreviewDataUrl] = useState('');
    const previewContainerRef = useRef<HTMLDivElement>(null);
    const [previewContainerSize, setPreviewContainerSize] = useState({ width: 0, height: 0 });
    const [previewImageSize, setPreviewImageSize] = useState({ width: 0, height: 0 });
    const [cropOffset, setCropOffset] = useState({ x: 0, y: 0 });
    const [isCropDragging, setIsCropDragging] = useState(false);
    const cropDragRef = useRef<{ startX: number; startY: number; offsetX: number; offsetY: number } | null>(null);
    const [isComparing, setIsComparing] = useState(false);
    const [watermarkType, setWatermarkType] = useState('文字');
    const [watermarkText, setWatermarkText] = useState('© ImageFlow');
    const [watermarkImagePath, setWatermarkImagePath] = useState('');
    const [watermarkPosition, setWatermarkPosition] = useState('br');
    const [watermarkOpacity, setWatermarkOpacity] = useState(85);
    const [watermarkRotate, setWatermarkRotate] = useState(0);
    const [watermarkSize, setWatermarkSize] = useState(40);
    const [watermarkTiled, setWatermarkTiled] = useState(false);
    const [watermarkBlendMode, setWatermarkBlendMode] = useState('正常');
    const [watermarkShadow, setWatermarkShadow] = useState(false);
    const [watermarkMargin, setWatermarkMargin] = useState({ x: 20, y: 20 });
    const [watermarkFont, setWatermarkFont] = useState('Sans Serif');
    const [watermarkColor, setWatermarkColor] = useState('#FFFFFF');
    const [useSystemFonts, setUseSystemFonts] = useState(false);
    const [systemFonts, setSystemFonts] = useState<string[]>([]);
    const [isSystemFontsLoading, setIsSystemFontsLoading] = useState(false);
    const [watermarkImageSize, setWatermarkImageSize] = useState({ width: 0, height: 0 });

    const normalizeOutputSettings = (raw?: Partial<OutputSettings> | null): OutputSettings => {
        if (!raw) return defaultOutputSettings;
        return {
            output_prefix: typeof raw.output_prefix === 'string' && raw.output_prefix.trim()
                ? raw.output_prefix
                : defaultOutputSettings.output_prefix,
            output_template: typeof raw.output_template === 'string' && raw.output_template.trim()
                ? raw.output_template
                : defaultOutputSettings.output_template,
            preserve_folder_structure: typeof raw.preserve_folder_structure === 'boolean'
                ? raw.preserve_folder_structure
                : defaultOutputSettings.preserve_folder_structure,
            conflict_strategy: typeof raw.conflict_strategy === 'string' && raw.conflict_strategy.trim() === 'rename'
                ? 'rename'
                : defaultOutputSettings.conflict_strategy,
        };
    };

    const loadOutputSettings = async () => {
        const appAny = window.go?.main?.App as any;
        if (!appAny?.GetSettings) {
            return defaultOutputSettings;
        }
        try {
            const res = await appAny.GetSettings();
            const normalized = normalizeOutputSettings(res);
            setOutputSettings(normalized);
            return normalized;
        } catch (e) {
            console.error(e);
            return outputSettings;
        }
    };

    const isGifPath = (p: string) => {
        const normalized = p.replace(/\\/g, '/');
        const idx = normalized.lastIndexOf('.');
        if (idx === -1) return false;
        return normalized.slice(idx + 1).toLowerCase() === 'gif';
    };

    const gifInputType = useMemo(() => {
        const list = dropResult?.files || [];
        if (list.length === 0) return 'empty' as const;
        let hasGif = false;
        let hasOther = false;
        list.forEach((f) => {
            if (isGifPath(f.input_path)) {
                hasGif = true;
            } else {
                hasOther = true;
            }
        });
        if (hasGif && !hasOther) return 'gif' as const;
        if (!hasGif && hasOther) return 'images' as const;
        return 'mixed' as const;
    }, [dropResult]);

    const previewSrc = useMemo(() => {
        if (!previewPath) return '';
        if (previewDataUrl) return previewDataUrl;
        return toFileUrl(previewPath);
    }, [previewDataUrl, previewPath]);
    const watermarkImageSrc = useMemo(() => {
        if (!watermarkImagePath) return '';
        return toFileUrl(watermarkImagePath);
    }, [watermarkImagePath]);
    const baseFontOptions = useMemo(() => ([
        'Sans Serif',
        'Serif',
        'Mono',
        'Handwriting',
    ]), []);
    const systemFontOptions = useMemo(() => systemFonts.map((fontPath) => ({
        label: fontPath.replace(/\\/g, '/').split('/').pop() || fontPath,
        value: fontPath,
    })), [systemFonts]);
    const fontOptions = useMemo(() => (
        useSystemFonts ? [...baseFontOptions, ...systemFontOptions] : baseFontOptions
    ), [useSystemFonts, baseFontOptions, systemFontOptions]);

    useEffect(() => {
        setPreviewImageSize({ width: 0, height: 0 });
    }, [previewSrc]);

    useEffect(() => {
        if (useSystemFonts) return;
        if (baseFontOptions.includes(watermarkFont)) return;
        setWatermarkFont('Sans Serif');
    }, [useSystemFonts, watermarkFont, baseFontOptions]);

    useEffect(() => {
        if (!useSystemFonts) return;
        if (systemFonts.length > 0) return;
        const appAny = window.go?.main?.App as any;
        if (!appAny?.ListSystemFonts) return;
        let active = true;
        setIsSystemFontsLoading(true);
        (async () => {
            try {
                const res = await appAny.ListSystemFonts();
                if (!active) return;
                if (Array.isArray(res)) {
                    setSystemFonts(res);
                } else {
                    setSystemFonts([]);
                }
            } catch (err) {
                if (active) {
                    console.error(err);
                    setSystemFonts([]);
                }
            } finally {
                if (active) {
                    setIsSystemFontsLoading(false);
                }
            }
        })();
        return () => {
            active = false;
        };
    }, [useSystemFonts, systemFonts.length]);

    useEffect(() => {
        if (!watermarkImageSrc) {
            setWatermarkImageSize({ width: 0, height: 0 });
            return;
        }
        let active = true;
        const img = new Image();
        img.onload = () => {
            if (!active) return;
            setWatermarkImageSize({
                width: img.naturalWidth,
                height: img.naturalHeight,
            });
        };
        img.onerror = () => {
            if (active) {
                setWatermarkImageSize({ width: 0, height: 0 });
            }
        };
        img.src = watermarkImageSrc;
        return () => {
            active = false;
        };
    }, [watermarkImageSrc]);
    const previewFilter = useMemo(() => {
        if (!isAdjustOrFilter) return '';
        if (id === 'adjust') {
            return buildAdjustPreviewFilter(
                adjustExposure,
                adjustContrast,
                adjustSaturation,
                adjustVibrance,
                adjustHue,
                adjustSharpness,
            );
        }
        return buildFilterPreviewFilter(filterSelected, filterIntensity);
    }, [
        adjustExposure,
        adjustContrast,
        adjustSaturation,
        adjustVibrance,
        adjustHue,
        adjustSharpness,
        filterSelected,
        filterIntensity,
        id,
        isAdjustOrFilter,
    ]);
    const previewTransform = useMemo(() => {
        if (!isAdjustOrFilter || id !== 'adjust') return '';
        const rotate = ((adjustRotate % 360) + 360) % 360;
        const scaleX = adjustFlipH ? -1 : 1;
        const scaleY = adjustFlipV ? -1 : 1;
        return `rotate(${rotate}deg) scale(${scaleX}, ${scaleY})`;
    }, [adjustFlipH, adjustFlipV, adjustRotate, id, isAdjustOrFilter]);
    const previewCropRatio = useMemo(() => {
        if (!isAdjustOrFilter || id !== 'adjust') return null;
        return parseCropRatio(adjustCropRatio);
    }, [adjustCropRatio, id, isAdjustOrFilter]);
    const previewFrame = useMemo(() => {
        const containerWidth = previewContainerSize.width;
        const containerHeight = previewContainerSize.height;
        if (!containerWidth || !containerHeight) {
            return { width: 0, height: 0, left: 0, top: 0 };
        }
        if (!previewCropRatio) {
            return { width: containerWidth, height: containerHeight, left: 0, top: 0 };
        }
        const containerRatio = containerWidth / containerHeight;
        const targetRatio = previewCropRatio.w / previewCropRatio.h;
        if (!Number.isFinite(containerRatio) || !Number.isFinite(targetRatio)) {
            return { width: containerWidth, height: containerHeight, left: 0, top: 0 };
        }
        let frameWidth = containerWidth;
        let frameHeight = containerHeight;
        if (containerRatio > targetRatio) {
            frameHeight = containerHeight;
            frameWidth = frameHeight * targetRatio;
        } else {
            frameWidth = containerWidth;
            frameHeight = frameWidth / targetRatio;
        }
        const left = (containerWidth - frameWidth) / 2;
        const top = (containerHeight - frameHeight) / 2;
        return {
            width: Math.max(0, frameWidth),
            height: Math.max(0, frameHeight),
            left: Math.max(0, left),
            top: Math.max(0, top),
        };
    }, [previewContainerSize, previewCropRatio]);
    const showCropFrame = Boolean(previewCropRatio);
    const isCropInteractive = showCropFrame && id === 'adjust' && Boolean(previewSrc);
    const cropImageMetrics = useMemo(() => {
        if (!showCropFrame) return null;
        if (!previewImageSize.width || !previewImageSize.height) return null;
        if (!previewFrame.width || !previewFrame.height) return null;
        const rotation = ((adjustRotate % 360) + 360) % 360;
        const rotated = rotation === 90 || rotation === 270;
        const rotatedWidth = rotated ? previewImageSize.height : previewImageSize.width;
        const rotatedHeight = rotated ? previewImageSize.width : previewImageSize.height;
        const scale = Math.max(
            previewFrame.width / rotatedWidth,
            previewFrame.height / rotatedHeight,
        );
        const drawnWidth = previewImageSize.width * scale;
        const drawnHeight = previewImageSize.height * scale;
        const rotatedDrawnWidth = rotated ? drawnHeight : drawnWidth;
        const rotatedDrawnHeight = rotated ? drawnWidth : drawnHeight;
        const maxOffsetX = Math.max(0, (rotatedDrawnWidth - previewFrame.width) / 2);
        const maxOffsetY = Math.max(0, (rotatedDrawnHeight - previewFrame.height) / 2);
        return {
            drawnWidth,
            drawnHeight,
            rotatedDrawnWidth,
            rotatedDrawnHeight,
            maxOffsetX,
            maxOffsetY,
        };
    }, [showCropFrame, previewImageSize, previewFrame, adjustRotate]);
    const cropFocus = useMemo(() => {
        if (!cropImageMetrics) {
            return { x: 0.5, y: 0.5 };
        }
        const x = clampNumber(0.5 - cropOffset.x / cropImageMetrics.rotatedDrawnWidth, 0, 1);
        const y = clampNumber(0.5 - cropOffset.y / cropImageMetrics.rotatedDrawnHeight, 0, 1);
        return { x, y };
    }, [cropImageMetrics, cropOffset]);
    const watermarkFontFace = useMemo(() => {
        if (!useSystemFonts) return null;
        const text = watermarkFont || '';
        const isPath = /[\\/]/.test(text) || /\.(ttf|otf|ttc)$/i.test(text);
        if (!isPath) return null;
        const safe = text.replace(/[^a-zA-Z0-9]/g, '_');
        const name = `wm_font_${safe.slice(-24)}`;
        return {
            name,
            css: `@font-face { font-family: '${name}'; src: url('${toFileUrl(text)}'); }`,
        };
    }, [useSystemFonts, watermarkFont]);
    const watermarkFontFamily = useMemo(() => {
        if (watermarkFontFace?.name) return watermarkFontFace.name;
        const fallbackMap: Record<string, string> = {
            'Sans Serif': 'Arial, "Helvetica Neue", sans-serif',
            'Serif': '"Times New Roman", serif',
            'Mono': '"Courier New", monospace',
            'Handwriting': '"Comic Sans MS", cursive',
        };
        return fallbackMap[watermarkFont] || watermarkFont || 'Arial, sans-serif';
    }, [watermarkFont, watermarkFontFace]);
    const previewBaseMetrics = useMemo(() => {
        if (!previewContainerSize.width || !previewContainerSize.height) return null;
        if (!previewImageSize.width || !previewImageSize.height) return null;
        const scale = Math.min(
            previewContainerSize.width / previewImageSize.width,
            previewContainerSize.height / previewImageSize.height,
        );
        const width = previewImageSize.width * scale;
        const height = previewImageSize.height * scale;
        const left = (previewContainerSize.width - width) / 2;
        const top = (previewContainerSize.height - height) / 2;
        return {
            width,
            height,
            left,
            top,
            scale,
        };
    }, [previewContainerSize, previewImageSize]);
    const watermarkPreviewConfig = useMemo(() => {
        if (!isWatermark || !previewBaseMetrics) return null;
        const textValue = watermarkText.trim() || '© ImageFlow';
        const opacity = clampNumber(watermarkOpacity / 100, 0, 1);
        const scale = clampNumber(watermarkSize / 100, 0.02, 1);
        const fontSize = Math.max(8, Math.round(36 * (watermarkSize / 40)));
        const previewFontSize = Math.max(8, Math.round(fontSize * previewBaseMetrics.scale));
        const offsetX = Math.max(0, Number(watermarkMargin.x) || 0) * previewBaseMetrics.scale;
        const offsetY = Math.max(0, Number(watermarkMargin.y) || 0) * previewBaseMetrics.scale;
        const blendMap: Record<string, string> = {
            '正常': 'normal',
            '正片叠底 (Multiply)': 'multiply',
            '滤色 (Screen)': 'screen',
            '叠加 (Overlay)': 'overlay',
            '柔光 (Soft Light)': 'soft-light',
        };
        return {
            textValue,
            opacity,
            scale,
            fontSize,
            previewFontSize,
            offsetX,
            offsetY,
            blendMode: blendMap[watermarkBlendMode] || 'normal',
        };
    }, [
        isWatermark,
        previewBaseMetrics,
        watermarkText,
        watermarkOpacity,
        watermarkSize,
        watermarkMargin,
        watermarkBlendMode,
    ]);
    const watermarkTextMetrics = useMemo(() => {
        if (!watermarkPreviewConfig) return null;
        if (typeof document === 'undefined') return null;
        const canvas = document.createElement('canvas');
        const ctx = canvas.getContext('2d');
        if (!ctx) return null;
        ctx.font = `${watermarkPreviewConfig.fontSize}px ${watermarkFontFamily}`;
        const metrics = ctx.measureText(watermarkPreviewConfig.textValue);
        const width = Math.max(1, Math.ceil(metrics.width));
        const height = Math.max(1, Math.ceil(watermarkPreviewConfig.fontSize * 1.2));
        return { width, height };
    }, [watermarkPreviewConfig, watermarkFontFamily]);
    const watermarkImagePreviewSize = useMemo(() => {
        if (!previewBaseMetrics || !watermarkPreviewConfig) return null;
        if (!watermarkImageSize.width || !watermarkImageSize.height) return null;
        const width = previewBaseMetrics.width * watermarkPreviewConfig.scale;
        const height = width * (watermarkImageSize.height / watermarkImageSize.width);
        return { width, height };
    }, [previewBaseMetrics, watermarkPreviewConfig, watermarkImageSize]);
    const watermarkTileMetrics = useMemo(() => {
        if (!previewBaseMetrics || !watermarkPreviewConfig) return null;
        if (watermarkType === '图片') {
            return watermarkImagePreviewSize;
        }
        if (!watermarkTextMetrics) return null;
        return {
            width: watermarkTextMetrics.width * previewBaseMetrics.scale,
            height: watermarkTextMetrics.height * previewBaseMetrics.scale,
        };
    }, [previewBaseMetrics, watermarkPreviewConfig, watermarkType, watermarkImagePreviewSize, watermarkTextMetrics]);
    const watermarkPositionStyle = useMemo(() => {
        if (!watermarkPreviewConfig) return {};
        const offsetX = watermarkPreviewConfig.offsetX;
        const offsetY = watermarkPreviewConfig.offsetY;
        const style: React.CSSProperties = {};
        switch (watermarkPosition) {
            case 'tl':
                style.left = offsetX;
                style.top = offsetY;
                break;
            case 'tc':
                style.left = '50%';
                style.top = offsetY;
                style.transform = 'translateX(-50%)';
                break;
            case 'tr':
                style.right = offsetX;
                style.top = offsetY;
                break;
            case 'ml':
                style.left = offsetX;
                style.top = '50%';
                style.transform = 'translateY(-50%)';
                break;
            case 'mc':
                style.left = '50%';
                style.top = '50%';
                style.transform = 'translate(-50%, -50%)';
                break;
            case 'mr':
                style.right = offsetX;
                style.top = '50%';
                style.transform = 'translateY(-50%)';
                break;
            case 'bl':
                style.left = offsetX;
                style.bottom = offsetY;
                break;
            case 'bc':
                style.left = '50%';
                style.bottom = offsetY;
                style.transform = 'translateX(-50%)';
                break;
            case 'br':
            default:
                style.right = offsetX;
                style.bottom = offsetY;
                break;
        }
        const baseTransform = style.transform ? `${style.transform} ` : '';
        style.transform = `${baseTransform}rotate(${watermarkRotate}deg)`;
        style.transformOrigin = 'center';
        return style;
    }, [watermarkPreviewConfig, watermarkPosition, watermarkRotate]);
    const watermarkTileGrid = useMemo(() => {
        if (!watermarkTiled || !previewBaseMetrics || !watermarkPreviewConfig || !watermarkTileMetrics) return null;
        const gapX = watermarkPreviewConfig.offsetX;
        const gapY = watermarkPreviewConfig.offsetY;
        const stepX = Math.max(1, watermarkTileMetrics.width + gapX);
        const stepY = Math.max(1, watermarkTileMetrics.height + gapY);
        const coverWidth = previewBaseMetrics.width * 2;
        const coverHeight = previewBaseMetrics.height * 2;
        let cols = Math.ceil(coverWidth / stepX);
        let rows = Math.ceil(coverHeight / stepY);
        const total = cols * rows;
        if (total > 400) {
            const ratio = Math.sqrt(400 / total);
            cols = Math.max(1, Math.floor(cols * ratio));
            rows = Math.max(1, Math.floor(rows * ratio));
        }
        return {
            cols,
            rows,
            coverWidth,
            coverHeight,
        };
    }, [watermarkTiled, previewBaseMetrics, watermarkPreviewConfig, watermarkTileMetrics]);
    const watermarkTilePositions = useMemo(() => {
        if (!watermarkTileGrid || !watermarkTileMetrics || !watermarkPreviewConfig) return null;
        const gapX = watermarkPreviewConfig.offsetX;
        const gapY = watermarkPreviewConfig.offsetY;
        const stepX = Math.max(1, watermarkTileMetrics.width + gapX);
        const stepY = Math.max(1, watermarkTileMetrics.height + gapY);
        const startX = -watermarkTileMetrics.width;
        const startY = -watermarkTileMetrics.height;
        const positions: Array<{ x: number; y: number }> = [];
        for (let row = 0; row < watermarkTileGrid.rows; row += 1) {
            const y = startY + row * stepY;
            for (let col = 0; col < watermarkTileGrid.cols; col += 1) {
                const x = startX + col * stepX;
                positions.push({ x, y });
            }
        }
        return positions;
    }, [watermarkTileGrid, watermarkTileMetrics, watermarkPreviewConfig]);
    const previewGrainOpacity = isAdjustOrFilter && id === 'filter'
        ? clampNumber(filterGrain / 100 * 0.35, 0, 0.35)
        : 0;
    const previewVignetteOpacity = isAdjustOrFilter && id === 'filter'
        ? clampNumber(filterVignette / 100 * 0.6, 0, 0.6)
        : 0;
    const isCompareActive = isComparing && Boolean(previewSrc);
    const effectivePreviewFilter = isCompareActive ? '' : previewFilter;
    const effectivePreviewTransform = isCompareActive ? '' : previewTransform;
    const effectiveGrainOpacity = isCompareActive ? 0 : previewGrainOpacity;
    const effectiveVignetteOpacity = isCompareActive ? 0 : previewVignetteOpacity;

    useEffect(() => {
        if (!showCropFrame) {
            setCropOffset({ x: 0, y: 0 });
            return;
        }
        setCropOffset({ x: 0, y: 0 });
    }, [adjustCropRatio, previewPath, showCropFrame]);

    useEffect(() => {
        if (!cropImageMetrics) return;
        setCropOffset((prev) => {
            const nextX = clampNumber(prev.x, -cropImageMetrics.maxOffsetX, cropImageMetrics.maxOffsetX);
            const nextY = clampNumber(prev.y, -cropImageMetrics.maxOffsetY, cropImageMetrics.maxOffsetY);
            if (nextX === prev.x && nextY === prev.y) return prev;
            return { x: nextX, y: nextY };
        });
    }, [cropImageMetrics]);

    useEffect(() => {
        const el = previewContainerRef.current;
        if (!el || typeof ResizeObserver === 'undefined') return;
        const observer = new ResizeObserver((entries) => {
            if (!entries.length) return;
            const { width, height } = entries[0].contentRect;
            setPreviewContainerSize((prev) => (
                prev.width === width && prev.height === height ? prev : { width, height }
            ));
        });
        observer.observe(el);
        return () => observer.disconnect();
    }, []);

    const handleCropPointerDown = useCallback((e: React.PointerEvent<HTMLDivElement>) => {
        if (!isCropInteractive || !cropImageMetrics) return;
        e.preventDefault();
        e.stopPropagation();
        setIsCropDragging(true);
        (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
        cropDragRef.current = {
            startX: e.clientX,
            startY: e.clientY,
            offsetX: cropOffset.x,
            offsetY: cropOffset.y,
        };
    }, [cropOffset, cropImageMetrics, isCropInteractive]);

    const handleCropPointerMove = useCallback((e: React.PointerEvent<HTMLDivElement>) => {
        if (!cropDragRef.current || !cropImageMetrics) return;
        const dx = e.clientX - cropDragRef.current.startX;
        const dy = e.clientY - cropDragRef.current.startY;
        const nextX = clampNumber(
            cropDragRef.current.offsetX + dx,
            -cropImageMetrics.maxOffsetX,
            cropImageMetrics.maxOffsetX,
        );
        const nextY = clampNumber(
            cropDragRef.current.offsetY + dy,
            -cropImageMetrics.maxOffsetY,
            cropImageMetrics.maxOffsetY,
        );
        setCropOffset((prev) => (
            prev.x === nextX && prev.y === nextY ? prev : { x: nextX, y: nextY }
        ));
    }, [cropImageMetrics]);

    const handleCropPointerUp = useCallback((e: React.PointerEvent<HTMLDivElement>) => {
        if (!cropDragRef.current) return;
        try {
            (e.currentTarget as HTMLElement).releasePointerCapture(e.pointerId);
        } catch {
            // ignore
        }
        cropDragRef.current = null;
        setIsCropDragging(false);
    }, []);

    useEffect(() => {
        loadOutputSettings();
    }, [id]);

    useEffect(() => {
        if (gifInputType === 'images' && gifMode !== '导出') {
            setGifMode('导出');
        }
    }, [gifInputType, gifMode]);

    const renderSettings = () => {
        switch(id) {
            case 'converter':
                return (
                    <ConverterSettings
                        format={convFormat}
                        setFormat={setConvFormat}
                        quality={convQuality}
                        setQuality={setConvQuality}
                        compressLevel={convCompressLevel}
                        setCompressLevel={setConvCompressLevel}
                        icoSizes={convIcoSizes}
                        setIcoSizes={setConvIcoSizes}
                        resizeMode={convResizeMode}
                        setResizeMode={setConvResizeMode}
                        scalePercent={convScalePercent}
                        setScalePercent={setConvScalePercent}
                        fixedWidth={convFixedWidth}
                        setFixedWidth={setConvFixedWidth}
                        fixedHeight={convFixedHeight}
                        setFixedHeight={setConvFixedHeight}
                        longEdge={convLongEdge}
                        setLongEdge={setConvLongEdge}
                        keepMetadata={convKeepMetadata}
                        setKeepMetadata={setConvKeepMetadata}
                        maintainAR={convMaintainAR}
                        setMaintainAR={setConvMaintainAR}
                        overwriteSource={convOverwriteSource}
                        setOverwriteSource={setConvOverwriteSource}
                    />
                );
            case 'compressor':
                return (
                    <CompressorSettings
                        mode={compMode}
                        setMode={setCompMode}
                        targetSize={compTargetSize}
                        setTargetSize={setCompTargetSize}
                        targetSizeKB={compTargetSizeKB}
                        setTargetSizeKB={setCompTargetSizeKB}
                        engine={compEngine}
                        setEngine={setCompEngine}
                        overwriteSource={compOverwriteSource}
                        setOverwriteSource={setCompOverwriteSource}
                    />
                );
            case 'watermark':
                return (
                    <WatermarkSettings
                        type={watermarkType}
                        setType={setWatermarkType}
                        text={watermarkText}
                        setText={setWatermarkText}
                        imagePath={watermarkImagePath}
                        setImagePath={setWatermarkImagePath}
                        position={watermarkPosition}
                        setPosition={setWatermarkPosition}
                        opacity={watermarkOpacity}
                        setOpacity={setWatermarkOpacity}
                        rotate={watermarkRotate}
                        setRotate={setWatermarkRotate}
                        size={watermarkSize}
                        setSize={setWatermarkSize}
                        tiled={watermarkTiled}
                        setTiled={setWatermarkTiled}
                        blendMode={watermarkBlendMode}
                        setBlendMode={setWatermarkBlendMode}
                        shadow={watermarkShadow}
                        setShadow={setWatermarkShadow}
                        margin={watermarkMargin}
                        setMargin={setWatermarkMargin}
                        font={watermarkFont}
                        setFont={setWatermarkFont}
                        color={watermarkColor}
                        setColor={setWatermarkColor}
                        useSystemFonts={useSystemFonts}
                        setUseSystemFonts={setUseSystemFonts}
                        isSystemFontsLoading={isSystemFontsLoading}
                        fontOptions={fontOptions}
                        systemFontsCount={systemFonts.length}
                    />
                );
            case 'adjust':
                return (
                    <AdjustSettings
                        exposure={adjustExposure}
                        setExposure={setAdjustExposure}
                        contrast={adjustContrast}
                        setContrast={setAdjustContrast}
                        saturation={adjustSaturation}
                        setSaturation={setAdjustSaturation}
                        sharpness={adjustSharpness}
                        setSharpness={setAdjustSharpness}
                        vibrance={adjustVibrance}
                        setVibrance={setAdjustVibrance}
                        hue={adjustHue}
                        setHue={setAdjustHue}
                        showCrop={false}
                    />
                );
            case 'filter':
                return (
                    <FilterSettings
                        intensity={filterIntensity}
                        setIntensity={setFilterIntensity}
                        grain={filterGrain}
                        setGrain={setFilterGrain}
                        vignette={filterVignette}
                        setVignette={setFilterVignette}
                        selected={filterSelected}
                        setSelected={setFilterSelected}
                        previewSrc={previewSrc}
                        getPreviewFilter={(index) => buildFilterPreviewFilter(index, filterIntensity)}
                    />
                );
            case 'pdf':
                return (
                    <PdfSettings
                        fileName={pdfFileName}
                        setFileName={(value) => setPdfFileName(sanitizePdfInputName(value))}
                        size={pdfSize}
                        setSize={setPdfSize}
                        layout={pdfLayout}
                        setLayout={setPdfLayout}
                        fit={pdfFit}
                        setFit={setPdfFit}
                        marginMm={pdfMarginMm}
                        setMarginMm={setPdfMarginMm}
                        compression={pdfCompression}
                        setCompression={setPdfCompression}
                        title={pdfTitle}
                        setTitle={setPdfTitle}
                        author={pdfAuthor}
                        setAuthor={setPdfAuthor}
                    />
                );
            case 'gif':
                return (
                    <GifSettings
                        mode={gifMode}
                        setMode={setGifMode}
                        exportFormat={gifExportFormat}
                        setExportFormat={setGifExportFormat}
                        speedPercent={gifSpeedPercent}
                        setSpeedPercent={setGifSpeedPercent}
                        sourceType={gifInputType}
                        buildFps={gifBuildFps}
                        setBuildFps={setGifBuildFps}
                    />
                );
            case 'info': {
                const onExportJSON = () => {
                    if (!infoPreview?.success) return;
                    try {
                        const content = JSON.stringify(infoPreview, null, 2);
                        const blob = new Blob([content], { type: 'application/json;charset=utf-8' });
                        const url = URL.createObjectURL(blob);
                        const a = document.createElement('a');
                        const name = (infoFilePath || 'image').replace(/\\/g, '/').split('/').pop() || 'image';
                        a.href = url;
                        a.download = `${name}.json`;
                        a.click();
                        URL.revokeObjectURL(url);
                    } catch (e) {
                        console.error(e);
                    }
                };

                const onClearPrivacy = async () => {
                    if (!infoFilePath) return;
                    try {
                        const normalized = infoFilePath.replace(/\\/g, '/');
                        const idx = normalized.lastIndexOf('/');
                        const dir = idx >= 0 ? normalized.slice(0, idx) : '';
                        const name = idx >= 0 ? normalized.slice(idx + 1) : normalized;
                        const baseName = stripExtension(name);
                        const ext = extname(name);
                        const outputName = buildOutputName(`${baseName}_clean`, {
                            template: outputSettings.output_template,
                            prefix: outputSettings.output_prefix,
                            seq: 1,
                            op: 'privacy',
                            date: new Date(),
                        });
                        const rawPath = (dir ? `${dir}/` : '') + (ext ? `${outputName}.${ext}` : outputName);
                        const appAny = (window.go?.main?.App as any);

                        if (!appAny?.StripMetadata) {
                            setLastMessage('后端未接入隐私清理接口');
                            return;
                        }
                        let resolvedPath = normalizePath(rawPath);
                        if (appAny?.ResolveOutputPath && outputSettings.conflict_strategy === 'rename') {
                            try {
                                const res = await appAny.ResolveOutputPath({
                                    base_path: normalizePath(rawPath),
                                    strategy: 'rename',
                                    reserved: [],
                                });
                                if (res?.success && res.output_path) {
                                    resolvedPath = normalizePath(res.output_path);
                                }
                            } catch (err) {
                                console.error(err);
                            }
                        }
                        setLastMessage('');
                        const res = await appAny.StripMetadata({ input_path: infoFilePath, output_path: resolvedPath, overwrite: false });
                        if (res?.success) {
                            setLastMessage(`隐私清理完成：${res.output_path || resolvedPath}`);
                            const refreshed = await window.go.main.App.GetInfo({ input_path: res.output_path || resolvedPath });
                            setInfoFilePath(res.output_path || resolvedPath);
                            setInfoPreview(refreshed);
                        } else {
                            setLastMessage(res?.error || '隐私清理失败');
                        }
                    } catch (e: any) {
                        console.error(e);
                        setLastMessage(typeof e?.message === 'string' ? e.message : '隐私清理失败');
                    }
                };

                const onEditMetadata = async (key: string, value: any) => {
                    if (!infoFilePath) return;
                    const appAny = (window.go?.main?.App as any);
                    if (!appAny?.EditMetadata) {
                        setLastMessage('后端未接入元数据编辑接口');
                        return;
                    }
                    setLastMessage('');
                    try {
                        const res = await appAny.EditMetadata({
                            input_path: infoFilePath,
                            output_path: infoFilePath,
                            exif_data: { [key]: value },
                            overwrite: true,
                        });
                        if (res?.success) {
                            setLastMessage('元数据已更新');
                            const refreshed = await window.go.main.App.GetInfo({ input_path: infoFilePath });
                            setInfoPreview(refreshed);
                        } else {
                            setLastMessage(res?.error || '元数据编辑失败');
                        }
                    } catch (e: any) {
                        console.error(e);
                        setLastMessage(typeof e?.message === 'string' ? e.message : '元数据编辑失败');
                    }
                };

                return (
                    <InfoSettings
                        filePath={infoFilePath}
                        info={infoPreview}
                        onExportJSON={onExportJSON}
                        onClearPrivacy={onClearPrivacy}
                        onEditMetadata={onEditMetadata}
                    />
                );
            }
            default: return <div className="text-gray-400 text-sm text-center py-10">暂无特殊设置</div>;
        }
    };

    const handleFilesSelected = (_files: File[]) => {};

    const inputCount = dropResult?.files?.length ?? 0;
    const effectiveOutputDir = useMemo(() => {
        if (outputDir) return outputDir;
        if (dropResult?.files?.[0]?.source_root) return dropResult.files[0].source_root;
        return '';
    }, [dropResult, outputDir]);

    const handleSelectOutputDir = async () => {
        setLastMessage('');
        try {
            if (window.go?.main?.App?.SelectOutputDirectory) {
                const dir = await window.go.main.App.SelectOutputDirectory();
                if (typeof dir === 'string' && dir.trim()) {
                    setOutputDir(dir);
                }
                return;
            }
            if (window.runtime?.OpenDirectoryDialog) {
                const dir = await window.runtime.OpenDirectoryDialog({ title: '选择输出位置' });
                if (typeof dir === 'string' && dir.trim()) {
                    setOutputDir(dir);
                }
            }
        } catch (e) {
            console.error(e);
            setLastMessage('选择输出目录失败');
        }
    };

    const normalizePath = (p: string) => p.replace(/\\/g, '/').replace(/\/+$/, '');
    const joinPath = (base: string, rel: string) => `${normalizePath(base)}/${rel.replace(/^\/+/, '')}`;
    const basename = (p: string) => p.replace(/\\/g, '/').split('/').pop() || p;
    const replaceExt = (p: string, ext: string) => {
        const normalized = p.replace(/\\/g, '/');
        const idx = normalized.lastIndexOf('.');
        if (idx === -1) return `${normalized}.${ext}`;
        return `${normalized.slice(0, idx)}.${ext}`;
    };
    const extname = (p: string) => {
        const normalized = p.replace(/\\/g, '/');
        const idx = normalized.lastIndexOf('.');
        if (idx === -1) return '';
        return normalized.slice(idx + 1).toLowerCase();
    };
    const matchesFormat = (format: string, ext: string) => {
        const f = (format || '').toLowerCase();
        const e = (ext || '').toLowerCase();
        if (f === 'jpg' || f === 'jpeg') return e === 'jpg' || e === 'jpeg';
        if (f === 'tif' || f === 'tiff') return e === 'tif' || e === 'tiff';
        return f === e;
    };
    const sanitizeFileName = (name: string) => (name || '').trim().replace(/[\\/:*?"<>|]+/g, '_');
    const stripExtension = (name: string) => {
        const idx = name.lastIndexOf('.');
        if (idx <= 0) return name;
        return name.slice(0, idx);
    };
    const sanitizeOutputName = (name: string) => {
        const cleaned = sanitizeFileName(name).replace(/[. ]+$/g, '').trim();
        if (!cleaned) return '';
        const upper = cleaned.toUpperCase();
        const reserved = new Set([
            'CON', 'PRN', 'AUX', 'NUL',
            'COM1', 'COM2', 'COM3', 'COM4', 'COM5', 'COM6', 'COM7', 'COM8', 'COM9',
            'LPT1', 'LPT2', 'LPT3', 'LPT4', 'LPT5', 'LPT6', 'LPT7', 'LPT8', 'LPT9',
        ]);
        if (reserved.has(upper)) {
            return `_${cleaned}`;
        }
        return cleaned;
    };
    const formatDatePart = (date: Date, pattern: string) => {
        const yyyy = `${date.getFullYear()}`;
        const MM = `${date.getMonth() + 1}`.padStart(2, '0');
        const dd = `${date.getDate()}`.padStart(2, '0');
        const HH = `${date.getHours()}`.padStart(2, '0');
        const mm = `${date.getMinutes()}`.padStart(2, '0');
        const ss = `${date.getSeconds()}`.padStart(2, '0');
        return pattern
            .replace(/YYYY/g, yyyy)
            .replace(/MM/g, MM)
            .replace(/DD/g, dd)
            .replace(/HH/g, HH)
            .replace(/mm/g, mm)
            .replace(/ss/g, ss);
    };
    const applyOutputTemplate = (
        template: string,
        vars: {
            basename: string;
            prefix: string;
            seq: number;
            op: string;
            date: Date;
        },
    ) => template.replace(/\{([a-z_]+)(?::([^}]+))?\}/gi, (_match, keyRaw, fmtRaw) => {
        const key = String(keyRaw).toLowerCase();
        const fmt = typeof fmtRaw === 'string' ? fmtRaw : '';
        if (key === 'basename') return vars.basename;
        if (key === 'prefix') return vars.prefix;
        if (key === 'op') return vars.op;
        if (key === 'date') {
            const pattern = fmt || 'YYYYMMDD';
            return formatDatePart(vars.date, pattern);
        }
        if (key === 'time') {
            const pattern = fmt || 'HHmmss';
            return formatDatePart(vars.date, pattern);
        }
        if (key === 'seq') {
            const width = /^\d+$/.test(fmt) ? Number(fmt) : 0;
            const text = String(vars.seq);
            return width > 0 ? text.padStart(width, '0') : text;
        }
        return '';
    });
    const buildOutputName = (
        baseName: string,
        options: {
            template: string;
            prefix: string;
            seq: number;
            op: string;
            date: Date;
        },
    ) => {
        const template = (options.template || defaultOutputSettings.output_template).trim() || defaultOutputSettings.output_template;
        const prefixClean = sanitizeFileName(options.prefix || '');
        const prefixValue = prefixClean ? `${prefixClean}_` : '';
        const templateHasPrefix = template.includes('{prefix}');
        const rendered = applyOutputTemplate(template, {
            basename: baseName,
            prefix: prefixValue,
            seq: options.seq,
            op: options.op,
            date: options.date,
        }).trim();
        let name = rendered || baseName;
        if (!templateHasPrefix && prefixValue) {
            name = `${prefixValue}${name}`;
        }
        name = sanitizeOutputName(name);
        if (!name) {
            name = sanitizeOutputName(baseName) || 'output';
        }
        if (name.length > 200) {
            name = name.slice(0, 200);
        }
        return name;
    };
    const getRelPath = (file: DroppedFile, preserveStructure: boolean) => {
        if (preserveStructure && file.is_from_dir_drop) {
            return file.relative_path || basename(file.input_path);
        }
        return basename(file.input_path);
    };
    const getRelDir = (relPath: string) => {
        const normalized = relPath.replace(/\\/g, '/');
        const idx = normalized.lastIndexOf('/');
        if (idx === -1) return '';
        return normalized.slice(0, idx);
    };
    const buildOutputRelPath = (
        file: DroppedFile,
        options: {
            ext?: string;
            suffix?: string;
            seq: number;
            op: string;
            template: string;
            prefix: string;
            preserveStructure: boolean;
            date: Date;
        },
    ) => {
        const rel = getRelPath(file, options.preserveStructure).replace(/\\/g, '/');
        const relDir = getRelDir(rel);
        const fileName = basename(rel);
        const baseName = stripExtension(fileName);
        const suffix = options.suffix || '';
        const ext = (options.ext || extname(fileName)).toLowerCase();
        const name = buildOutputName(`${baseName}${suffix}`, {
            template: options.template,
            prefix: options.prefix,
            seq: options.seq,
            op: options.op,
            date: options.date,
        });
        const fullName = ext ? `${name}.${ext}` : name;
        return relDir ? `${relDir}/${fullName}` : fullName;
    };
    const sanitizePdfInputName = (name: string) => sanitizeFileName(stripExtension(name || ''));
    const normalizePdfFileName = (name: string) => {
        const cleaned = sanitizeFileName(stripExtension(name || ''));
        return cleaned || 'document';
    };
    const buildSuggestedPdfName = (list: DroppedFile[], hasDirectory?: boolean) => {
        if (!list.length) return 'document';
        const baseFromPath = (p: string) => stripExtension(basename(p));
        let name = '';
        if (hasDirectory && list[0]?.source_root) {
            name = baseFromPath(list[0].source_root);
        }
        const baseNames = list.map((f) => baseFromPath(f.input_path)).filter(Boolean);
        if (!name && baseNames.length > 0) {
            if (baseNames.length === 1) {
                name = baseNames[0];
            } else {
                let prefix = baseNames[0];
                for (const item of baseNames.slice(1)) {
                    while (prefix && !item.startsWith(prefix)) {
                        prefix = prefix.slice(0, -1);
                    }
                    if (!prefix) break;
                }
                const trimmed = prefix.replace(/[_\-\s.]+$/g, '').trim();
                name = trimmed.length >= 3 ? trimmed : baseNames[0];
                if (!hasDirectory) {
                    name = `${name}_merged`;
                }
            }
        }
        return normalizePdfFileName(name);
    };
    const getErrorMessage = (error: unknown, fallback: string) => {
        if (typeof error === 'string' && error.trim()) return error.trim();
        if (typeof (error as any)?.message === 'string' && (error as any).message.trim()) {
            return (error as any).message.trim();
        }
        return fallback;
    };
    const reportTaskFailure = (taskName: string, filePath: string, reason: unknown, fallback = '处理失败') => {
        if (!onTaskFailure) return;
        onTaskFailure({
            taskName,
            imageName: basename(filePath),
            reason: getErrorMessage(reason, fallback),
        });
    };
    const reportBatchTaskFailure = (taskName: string, files: DroppedFile[], reason: unknown, fallback = '处理失败') => {
        if (!onTaskFailure) return;
        const firstPath = files[0]?.input_path || '';
        const firstName = firstPath ? basename(firstPath) : '批量输入';
        const imageName = files.length > 1 ? `${firstName} 等` : firstName;
        onTaskFailure({
            taskName,
            imageName,
            reason: getErrorMessage(reason, fallback),
        });
    };
    const clampTwoLinesStyle: React.CSSProperties = {
        display: '-webkit-box',
        WebkitLineClamp: 2,
        WebkitBoxOrient: 'vertical',
        overflow: 'hidden',
    };
    function toFileUrl(p: string) {
        if (!p) return '';
        const normalized = p.replace(/\\/g, '/');
        if (/^[a-zA-Z]:/.test(normalized)) {
            return `file:///${encodeURI(normalized)}`;
        }
        if (normalized.startsWith('/')) {
            return `file://${encodeURI(normalized)}`;
        }
        return `file:///${encodeURI(normalized)}`;
    }
    function clampNumber(value: number, min: number, max: number) {
        return Math.min(max, Math.max(min, value));
    }
    function parseCropRatio(value: string) {
        const text = (value || '').trim().toLowerCase();
        if (!text || text === '自由' || text === 'free' || text === 'original') {
            return null;
        }
        if (!text.includes(':')) return null;
        const parts = text.split(':');
        if (parts.length !== 2) return null;
        const w = Number(parts[0]);
        const h = Number(parts[1]);
        if (!Number.isFinite(w) || !Number.isFinite(h) || w <= 0 || h <= 0) {
            return null;
        }
        return { w, h };
    }
    function buildAdjustPreviewFilter(
        exposure: number,
        contrast: number,
        saturation: number,
        vibrance: number,
        hue: number,
        sharpness: number,
    ) {
        const brightness = clampNumber(1 + exposure / 100, 0, 2);
        const baseContrast = clampNumber(1 + contrast / 100, 0, 2);
        const satCombined = saturation + vibrance * 0.6;
        const saturate = clampNumber(1 + satCombined / 100, 0, 3);
        const blurPx = sharpness < 0 ? clampNumber(Math.abs(sharpness) / 35, 0, 6) : 0;
        const sharpnessBoost = sharpness > 0 ? clampNumber(1 + sharpness / 200, 1, 1.8) : 1;
        const hueDeg = clampNumber(hue, -180, 180);

        const parts = [
            `brightness(${brightness})`,
            `contrast(${(baseContrast * sharpnessBoost).toFixed(3)})`,
            `saturate(${saturate})`,
            `hue-rotate(${hueDeg}deg)`,
        ];
        if (blurPx > 0) {
            parts.push(`blur(${blurPx.toFixed(2)}px)`);
        }
        return parts.join(' ');
    }
    function buildFilterPreviewFilter(index: number, intensity: number) {
        const t = clampNumber(intensity / 100, 0, 1);
        const mix = (base: number, target: number) => base + (target - base) * t;

        switch (index) {
            case 1: // 鲜艳
                return `saturate(${mix(1, 1.8)}) contrast(${mix(1, 1.25)})`;
            case 2: // 黑白
                return `grayscale(${t})`;
            case 3: // 复古
                return `sepia(${mix(0, 0.8)}) contrast(${mix(1, 1.15)})`;
            case 4: // 冷调
                return `saturate(${mix(1, 1.2)}) hue-rotate(${mix(0, -18)}deg)`;
            case 5: // 暖阳
                return `sepia(${mix(0, 0.35)}) hue-rotate(${mix(0, 12)}deg) saturate(${mix(1, 1.2)})`;
            case 6: // 胶片
                return `sepia(${mix(0, 1)}) contrast(${mix(1, 1.2)}) brightness(${mix(1, 1.05)})`;
            case 7: // 赛博
                return `hue-rotate(${mix(0, 90)}deg) saturate(${mix(1, 1.5)}) contrast(${mix(1, 1.2)})`;
            case 8: // 清新
                return `brightness(${mix(1, 1.08)}) saturate(${mix(1, 1.25)})`;
            case 9: // 日系
                return `brightness(${mix(1, 1.12)}) contrast(${mix(1, 0.88)}) saturate(${mix(1, 1.1)})`;
            case 10: // Lomo
                return `contrast(${mix(1, 1.3)}) saturate(${mix(1, 1.35)})`;
            case 11: // HDR
                return `contrast(${mix(1, 1.45)}) saturate(${mix(1, 1.2)})`;
            case 12: // 褪色
                return `saturate(${mix(1, 0.65)}) brightness(${mix(1, 1.08)}) contrast(${mix(1, 0.9)})`;
            case 13: // 磨砂
                return `blur(${mix(0, 2)}px)`;
            case 14: // 电影
                return `contrast(${mix(1, 1.25)}) saturate(${mix(1, 0.9)})`;
            case 15: // 拍立得
                return `sepia(${mix(0, 1)}) contrast(${mix(1, 1.1)}) brightness(${mix(1, 1.08)})`;
            case 16: // 夕阳
                return `sepia(${mix(0, 0.35)}) saturate(${mix(1, 1.3)}) hue-rotate(${mix(0, -10)}deg) brightness(${mix(1, 1.05)})`;
            case 17: // 海蓝
                return `hue-rotate(${mix(0, -25)}deg) saturate(${mix(1, 1.2)}) brightness(${mix(1, 1.02)})`;
            case 18: // 森系
                return `hue-rotate(${mix(0, 18)}deg) saturate(${mix(1, 1.15)}) contrast(${mix(1, 1.05)})`;
            case 19: // 紫雾
                return `hue-rotate(${mix(0, 35)}deg) saturate(${mix(1, 1.2)})`;
            case 20: // 琥珀
                return `sepia(${mix(0, 0.5)}) saturate(${mix(1, 1.1)}) contrast(${mix(1, 1.08)})`;
            case 21: // 北欧
                return `brightness(${mix(1, 1.08)}) saturate(${mix(1, 0.9)}) contrast(${mix(1, 0.92)})`;
            case 22: // 旧照片
                return `sepia(${mix(0, 0.7)}) contrast(${mix(1, 0.9)}) brightness(${mix(1, 1.05)}) saturate(${mix(1, 0.8)})`;
            case 23: // 黑金
                return `grayscale(${mix(0, 1)}) contrast(${mix(1, 1.35)})`;
            case 24: // 高调
                return `brightness(${mix(1, 1.2)}) contrast(${mix(1, 0.9)})`;
            case 25: // 低调
                return `brightness(${mix(1, 0.75)}) contrast(${mix(1, 1.2)})`;
            case 26: // 雾霭
                return `contrast(${mix(1, 0.85)}) brightness(${mix(1, 1.05)})`;
            case 27: // 霓虹
                return `saturate(${mix(1, 1.6)}) contrast(${mix(1, 1.15)}) hue-rotate(${mix(0, 15)}deg)`;
            case 28: // 哑光
                return `contrast(${mix(1, 0.85)}) saturate(${mix(1, 0.9)}) brightness(${mix(1, 1.03)})`;
            case 29: // 冰感
                return `hue-rotate(${mix(0, -10)}deg) saturate(${mix(1, 0.85)}) brightness(${mix(1, 1.1)})`;
            case 30: // 咖啡
                return `sepia(${mix(0, 0.5)}) brightness(${mix(1, 0.98)}) contrast(${mix(1, 1.05)})`;
            case 31: // 焦糖
                return `sepia(${mix(0, 0.4)}) saturate(${mix(1, 1.2)}) contrast(${mix(1, 1.1)})`;
            case 32: // 青橙
                return `hue-rotate(${mix(0, 12)}deg) saturate(${mix(1, 1.25)}) contrast(${mix(1, 1.1)})`;
            case 33: // 银盐
                return `grayscale(${mix(0, 0.4)}) contrast(${mix(1, 1.15)}) brightness(${mix(1, 1.02)})`;
            case 34: // 清锐
                return `contrast(${mix(1, 1.25)}) saturate(${mix(1, 1.1)})`;
            case 35: // 低对比
                return `contrast(${mix(1, 0.8)}) saturate(${mix(1, 0.95)})`;
            default:
                return '';
        }
    }

    useEffect(() => {
        if (!dropResult || dropResult.files.length === 0) {
            setPdfFileName('');
            return;
        }
        const suggested = buildSuggestedPdfName(dropResult.files, dropResult.has_directory);
        setPdfFileName(suggested);
    }, [dropResult]);

    useEffect(() => {
        if (!isPreviewFeature) {
            setPreviewPath('');
            return;
        }
        const first = dropResult?.files?.[0];
        if (first?.input_path) {
            setPreviewPath(normalizePath(first.input_path));
        } else {
            setPreviewPath('');
        }
    }, [dropResult, isPreviewFeature]);

    useEffect(() => {
        if (!isPreviewFeature) {
            setPreviewDataUrl('');
            return;
        }
        if (!previewPath) {
            setPreviewDataUrl('');
            return;
        }
        let cancelled = false;
        const appAny = window.go?.main?.App as any;
        if (!appAny?.GetImagePreview) {
            setPreviewDataUrl('');
            return;
        }
        setPreviewDataUrl('');
        (async () => {
            try {
                const res = await appAny.GetImagePreview({ input_path: previewPath });
                if (cancelled) return;
                if (res?.success && res.data_url) {
                    setPreviewDataUrl(res.data_url);
                } else {
                    setPreviewDataUrl('');
                }
            } catch (err) {
                if (!cancelled) {
                    setPreviewDataUrl('');
                }
            }
        })();
        return () => {
            cancelled = true;
        };
    }, [previewPath, isPreviewFeature]);

    const loadInfoForPath = async (p: string) => {
        if (!window.go?.main?.App?.GetInfo) {
            setLastMessage('未检测到 Wails 运行环境');
            return;
        }
        const normalized = normalizePath(p);
        setIsProcessing(true);
        setProgress(0);
        setLastMessage('');
        setInfoFilePath(normalized);
        setInfoPreview(null);
        try {
            const info = await window.go.main.App.GetInfo({ input_path: normalized });
            setInfoPreview(info);
            if (info?.success) {
                setLastMessage(`信息读取完成：${basename(normalized)}`);
            } else {
                setLastMessage(info?.error || '信息读取失败');
            }
        } catch (err: any) {
            console.error(`Failed to get info ${p}:`, err);
            const msg = typeof err?.message === 'string' ? err.message : '信息读取失败';
            setInfoPreview({ success: false, error: msg });
            setLastMessage(msg);
        } finally {
            setProgress(100);
            setIsProcessing(false);
        }
    };

    const handleStartProcessing = async () => {
        if (isProcessing) return;
        setLastMessage('');
        setProgress(0);
        if (!dropResult || dropResult.files.length === 0) {
            setLastMessage('请先拖入文件或文件夹');
            return;
        }
        if (!window.go?.main?.App) {
            setLastMessage('未检测到 Wails 运行环境');
            return;
        }

        const outDir = effectiveOutputDir;
        if (id !== 'info' && !outDir) {
            setLastMessage('请选择输出目录');
            return;
        }

        const outputSettingsSnapshot = await loadOutputSettings();
        const preserveStructure = Boolean(outputSettingsSnapshot.preserve_folder_structure);
        const outputTemplate = outputSettingsSnapshot.output_template || defaultOutputSettings.output_template;
        const outputPrefix = outputSettingsSnapshot.output_prefix || defaultOutputSettings.output_prefix;
        const conflictStrategy = outputSettingsSnapshot.conflict_strategy || defaultOutputSettings.conflict_strategy;
        const reservedPaths = new Set<string>();
        const batchTime = new Date();
        const resolveUniquePath = async (candidate: string) => {
            const normalized = normalizePath(candidate);
            if (conflictStrategy !== 'rename') {
                reservedPaths.add(normalized);
                return normalized;
            }
            const appAny = window.go?.main?.App as any;
            if (!appAny?.ResolveOutputPath) {
                reservedPaths.add(normalized);
                return normalized;
            }
            try {
                const res = await appAny.ResolveOutputPath({
                    base_path: normalized,
                    strategy: 'rename',
                    reserved: Array.from(reservedPaths),
                });
                const resolved = res?.success && res.output_path ? normalizePath(res.output_path) : normalized;
                reservedPaths.add(resolved);
                return resolved;
            } catch (err) {
                console.error(err);
                reservedPaths.add(normalized);
                return normalized;
            }
        };

        setIsProcessing(true);
        try {
            const files = dropResult.files;
            const total = files.length;
            let completed = 0;

            if (id === 'converter') {
                const format = convFormat.toLowerCase();
                const quality = convQuality;
                const compress_level = convCompressLevel;
                const ico_sizes = convIcoSizes;
                const effectiveIcoSizes = (ico_sizes || []).filter((s) => typeof s === 'number' && s > 0);
                const resizeModeMap: Record<string, string> = {
                    '原图尺寸': 'original',
                    '按比例': 'percent',
                    '固定宽高': 'fixed',
                    '最长边': 'long_edge',
                };
                const resize_mode = resizeModeMap[convResizeMode] ?? 'original';
                const chunkSize = total >= 80 ? 20 : 1;
                let seq = 1;
                for (let i = 0; i < files.length; i += chunkSize) {
                    const group = files.slice(i, i + chunkSize);
                    const chunk = [];
                    for (const f of group) {
                        const input_path = normalizePath(f.input_path);
                        const canOverwrite = convOverwriteSource && matchesFormat(format, extname(f.input_path));
                        let output_path = input_path;
                        if (!canOverwrite) {
                            const rel = buildOutputRelPath(f, {
                                ext: format,
                                seq,
                                op: 'converter',
                                template: outputTemplate,
                                prefix: outputPrefix,
                                preserveStructure,
                                date: batchTime,
                            });
                            output_path = await resolveUniquePath(joinPath(outDir, rel));
                        }
                        chunk.push({
                            input_path,
                            output_path,
                            format,
                            quality,
                            compress_level,
                            ico_sizes: format === 'ico' ? (effectiveIcoSizes.length ? effectiveIcoSizes : [16]) : [],
                            width: resize_mode === 'fixed' ? convFixedWidth : 0,
                            height: resize_mode === 'fixed' ? convFixedHeight : 0,
                            maintain_ar: convMaintainAR,
                            resize_mode,
                            scale_percent: resize_mode === 'percent' ? convScalePercent : 0,
                            long_edge: resize_mode === 'long_edge' ? convLongEdge : 0,
                            keep_metadata: convKeepMetadata,
                        });
                        seq += 1;
                    }
                    try {
                        if (chunk.length === 1) {
                            const res = await window.go.main.App.Convert(chunk[0]);
                            if (!(res as any)?.success) {
                                reportTaskFailure('格式转换', chunk[0].input_path, (res as any)?.error, '转换失败');
                            }
                        } else {
                            const res = await window.go.main.App.ConvertBatch(chunk);
                            if (Array.isArray(res)) {
                                res.forEach((item: any, idx: number) => {
                                    if (!item?.success) {
                                        reportTaskFailure('格式转换', chunk[idx]?.input_path || item?.input_path || '', item?.error, '转换失败');
                                    }
                                });
                            }
                        }
                    } catch (err) {
                        console.error(err);
                        reportBatchTaskFailure('格式转换', group, err, '转换失败');
                    }

                    completed += chunk.length;
                    setProgress((completed / total) * 100);
                }
                setLastMessage(`转换完成：${completed}/${total} 项`);
                return;
            }

            if (id === 'compressor') {
                const levelMap: Record<string, number> = {
                    '无损': 1,
                    '轻度': 2,
                    '标准': 3,
                    '强力': 4,
                    '极限': 5,
                };
                const engineMap: Record<string, string> = {
                    '自动 (推荐)': 'auto',
                    'MozJPEG (JPEG)': 'mozjpeg',
                    'PNGQuant (PNG)': 'pngquant',
                    'OxiPNG (PNG 无损)': 'oxipng',
                    'Pillow (兼容)': 'pillow',
                };
                const level = levelMap[compMode] ?? 3;
                const engine = engineMap[compEngine] ?? 'auto';
                const targetSizeKB = compTargetSize ? Math.max(0, Number(compTargetSizeKB) || 0) : 0;

                let failed = 0;
                let warned = 0;

                const chunkSize = total >= 80 ? 20 : 1;
                let seq = 1;
                for (let i = 0; i < files.length; i += chunkSize) {
                    const group = files.slice(i, i + chunkSize);
                    const chunk = [];
                    for (const f of group) {
                        const input_path = normalizePath(f.input_path);
                        let output_path = input_path;
                        if (!compOverwriteSource) {
                            const rel = buildOutputRelPath(f, {
                                seq,
                                op: 'compressor',
                                template: outputTemplate,
                                prefix: outputPrefix,
                                preserveStructure,
                                date: batchTime,
                            });
                            output_path = await resolveUniquePath(joinPath(outDir, rel));
                        }
                        chunk.push({
                            input_path,
                            output_path,
                            level,
                            engine,
                            target_size_kb: targetSizeKB,
                            strip_metadata: true,
                        });
                        seq += 1;
                    }
                    try {
                        if (chunk.length === 1) {
                            const res = await window.go.main.App.Compress(chunk[0]);
                            if ((res as any)?.warning) warned++;
                            if (!(res as any)?.success) {
                                failed += 1;
                                reportTaskFailure('图片压缩', chunk[0].input_path, (res as any)?.error, '压缩失败');
                            }
                        } else {
                            const res = await window.go.main.App.CompressBatch(chunk);
                            warned += (res as any[]).filter(r => r?.warning).length;
                            (res as any[]).forEach((item: any, idx: number) => {
                                if (!item?.success) {
                                    failed += 1;
                                    reportTaskFailure('图片压缩', chunk[idx]?.input_path || item?.input_path || '', item?.error, '压缩失败');
                                }
                            });
                        }
                    } catch (err) {
                        console.error(err);
                        failed += chunk.length;
                        reportBatchTaskFailure('图片压缩', group, err, '压缩失败');
                    }

                    completed += chunk.length;
                    setProgress((completed / total) * 100);
                }
                const extra = failed > 0 || warned > 0 ? `（失败 ${failed}，未达目标 ${warned}）` : '';
                setLastMessage(`压缩完成：${completed}/${total} 项${extra}`);
                return;
            }

            if (id === 'watermark') {
                const isText = watermarkType === '文字';
                if (!isText && !watermarkImagePath) {
                    setLastMessage('请先选择水印图片');
                    return;
                }
                const fontMap: Record<string, string> = {
                    'Sans Serif': 'arial',
                    'Serif': 'times',
                    'Mono': 'cour',
                    'Handwriting': 'comic sans ms',
                };
                const blendMap: Record<string, string> = {
                    '正常': 'normal',
                    '正片叠底 (Multiply)': 'multiply',
                    '滤色 (Screen)': 'screen',
                    '叠加 (Overlay)': 'overlay',
                    '柔光 (Soft Light)': 'soft_light',
                };
                const textValue = watermarkText.trim() || '© ImageFlow';
                const opacity = clampNumber(watermarkOpacity / 100, 0, 1);
                const scale = clampNumber(watermarkSize / 100, 0.02, 1);
                const fontSize = Math.max(8, Math.round(36 * (watermarkSize / 40)));
                const fontName = fontMap[watermarkFont] || watermarkFont;
                const blendMode = blendMap[watermarkBlendMode] || 'normal';
                const positionMap: Record<string, string> = {
                    ml: 'cl',
                    mc: 'c',
                    mr: 'cr',
                };
                const resolvedPosition = positionMap[watermarkPosition] || watermarkPosition;

                let seq = 1;
                for (const f of files) {
                    const outRel = buildOutputRelPath(f, {
                        suffix: '_watermark',
                        seq,
                        op: 'watermark',
                        template: outputTemplate,
                        prefix: outputPrefix,
                        preserveStructure,
                        date: batchTime,
                    });
                    const req = {
                        input_path: normalizePath(f.input_path),
                        output_path: await resolveUniquePath(joinPath(outDir, outRel)),
                        watermark_type: isText ? 'text' : 'image',
                        text: textValue,
                        image_path: watermarkImagePath ? normalizePath(watermarkImagePath) : '',
                        position: resolvedPosition,
                        opacity,
                        scale,
                        font_size: fontSize,
                        font_color: watermarkColor,
                        rotation: watermarkRotate,
                        font_name: fontName,
                        blend_mode: blendMode,
                        tiled: watermarkTiled,
                        shadow: watermarkShadow,
                        offset_x: Math.max(0, Number(watermarkMargin.x) || 0),
                        offset_y: Math.max(0, Number(watermarkMargin.y) || 0),
                    };
                    try {
                        const res = await window.go.main.App.AddWatermark(req);
                        if (!(res as any)?.success) {
                            reportTaskFailure('图片水印', f.input_path, (res as any)?.error, '水印失败');
                        }
                    } catch (err) {
                        console.error(`Failed to watermark ${f.input_path}:`, err);
                        reportTaskFailure('图片水印', f.input_path, err, '水印失败');
                    }
                    completed++;
                    seq += 1;
                    setProgress((completed / total) * 100);
                }
                setLastMessage(`水印完成：${completed}/${total} 项`);
                return;
            }

            if (id === 'adjust') {
                let seq = 1;
                for (const f of files) {
                    const outRel = buildOutputRelPath(f, {
                        suffix: '_adjusted',
                        seq,
                        op: 'adjust',
                        template: outputTemplate,
                        prefix: outputPrefix,
                        preserveStructure,
                        date: batchTime,
                    });
                    const cropRatio = adjustCropRatio === '自由' ? '' : adjustCropRatio;
                    const cropMode = cropRatio ? `focus:${cropFocus.x.toFixed(4)},${cropFocus.y.toFixed(4)}` : '';
                    const req = {
                        input_path: normalizePath(f.input_path),
                        output_path: await resolveUniquePath(joinPath(outDir, outRel)),
                        rotate: adjustRotate,
                        flip_h: adjustFlipH,
                        flip_v: adjustFlipV,
                        brightness: 0,
                        exposure: adjustExposure,
                        contrast: adjustContrast,
                        saturation: adjustSaturation,
                        vibrance: adjustVibrance,
                        hue: adjustHue,
                        sharpness: adjustSharpness,
                        crop_ratio: cropRatio,
                        crop_mode: cropMode,
                    };
                    try {
                        const res = await window.go.main.App.Adjust(req);
                        if (!(res as any)?.success) {
                            reportTaskFailure('图片调整', f.input_path, (res as any)?.error, '调整失败');
                        }
                    } catch (err) {
                        console.error(`Failed to adjust ${f.input_path}:`, err);
                        reportTaskFailure('图片调整', f.input_path, err, '调整失败');
                    }
                    completed++;
                    seq += 1;
                    setProgress((completed / total) * 100);
                }
                setLastMessage(`调整完成：${completed}/${total} 项`);
                return;
            }

            if (id === 'filter') {
                const filterPreset = FILTER_PRESETS[filterSelected] || 'none';
                const intensity = clampNumber(filterIntensity / 100, 0, 1);
                const grain = clampNumber(filterGrain / 100, 0, 1);
                const vignette = clampNumber(filterVignette / 100, 0, 1);
                let seq = 1;
                for (const f of files) {
                    const outRel = buildOutputRelPath(f, {
                        suffix: '_filtered',
                        seq,
                        op: 'filter',
                        template: outputTemplate,
                        prefix: outputPrefix,
                        preserveStructure,
                        date: batchTime,
                    });
                    const req = {
                        input_path: normalizePath(f.input_path),
                        output_path: await resolveUniquePath(joinPath(outDir, outRel)),
                        filter_type: filterPreset,
                        intensity,
                        grain,
                        vignette,
                    };
                    try {
                        const res = await window.go.main.App.ApplyFilter(req);
                        if (!(res as any)?.success) {
                            reportTaskFailure('图片滤镜', f.input_path, (res as any)?.error, '滤镜失败');
                        }
                    } catch (err) {
                        console.error(`Failed to filter ${f.input_path}:`, err);
                        reportTaskFailure('图片滤镜', f.input_path, err, '滤镜失败');
                    }
                    completed++;
                    seq += 1;
                    setProgress((completed / total) * 100);
                }
                setLastMessage(`滤镜完成：${completed}/${total} 项`);
                return;
            }

            if (id === 'pdf') {
                const sizeMap: Record<string, string> = {
                    'A0 (841 x 1189 mm)': 'A0',
                    'A1 (594 x 841 mm)': 'A1',
                    'A2 (420 x 594 mm)': 'A2',
                    'A3 (297 x 420 mm)': 'A3',
                    'A4 (210 x 297 mm)': 'A4',
                    'A5 (148 x 210 mm)': 'A5',
                    'A6 (105 x 148 mm)': 'A6',
                    'B4 (250 x 353 mm)': 'B4',
                    'B5 (176 x 250 mm)': 'B5',
                    'B6 (125 x 176 mm)': 'B6',
                    'Letter (8.5 x 11 in)': 'Letter',
                    'Legal (8.5 x 14 in)': 'Legal',
                    'Tabloid (11 x 17 in)': 'Tabloid',
                    'Ledger (17 x 11 in)': 'Ledger',
                };
                const compressionMap: Record<string, number> = {
                    '不压缩': 0,
                    '轻度': 1,
                    '标准': 2,
                    '强力': 3,
                };
                const fitMap: Record<string, string> = {
                    '适应页面 (保持比例)': 'contain',
                    '充满页面 (可能裁剪)': 'cover',
                    '原始大小 (居中)': 'original',
                };
                const marginPoints = Math.max(0, Math.round((Number(pdfMarginMm) || 0) * 72 / 25.4));
                const pdfBaseName = normalizePdfFileName(pdfFileName) || 'output';
                const pdfName = buildOutputName(pdfBaseName, {
                    template: outputTemplate,
                    prefix: outputPrefix,
                    seq: 1,
                    op: 'pdf',
                    date: batchTime,
                });
                const req: any = {
                    image_paths: files.map(f => normalizePath(f.input_path)),
                    output_path: await resolveUniquePath(joinPath(outDir, `${pdfName}.pdf`)),
                    page_size: sizeMap[pdfSize] ?? 'A4',
                    layout: pdfLayout === '横向' ? 'landscape' : 'portrait',
                    margin: marginPoints,
                    compression_level: compressionMap[pdfCompression] ?? 0,
                    fit_mode: fitMap[pdfFit] ?? 'contain',
                    title: pdfTitle.trim(),
                    author: pdfAuthor.trim(),
                };
                const res = await window.go.main.App.GeneratePDF(req);
                setProgress(100);
                if ((res as any)?.success) {
                    setLastMessage(`PDF 已生成：${req.output_path}`);
                } else {
                    reportTaskFailure('转 PDF', files[0]?.input_path || '', (res as any)?.error, 'PDF 生成失败');
                    setLastMessage((res as any)?.error || 'PDF 生成失败');
                }
                return;
            }

            if (id === 'gif') {
                const appAny = window.go?.main?.App as any;
                if (!appAny?.SplitGIF) {
                    setLastMessage('后端未接入 GIF 接口');
                    return;
                }

                const isGifFile = (path: string) => extname(path) === 'gif';
                const gifFiles = files.filter(f => isGifFile(f.input_path));
                const otherFiles = files.filter(f => !isGifFile(f.input_path));

                if (gifMode === '导出') {
                    if (gifFiles.length > 0 && otherFiles.length > 0) {
                        setLastMessage('导出模式请只选择 GIF 或图片序列');
                        return;
                    }

                    if (gifFiles.length > 0) {
                        const outputFormat = gifExportFormat.toLowerCase();
                        let failed = 0;
                        let seq = 1;
                        for (const f of gifFiles) {
                            const name = basename(f.input_path).replace(/\.[^.]+$/, '');
                            const rel = getRelPath(f, preserveStructure);
                            const relDir = getRelDir(rel);
                            const folderName = buildOutputName(`${name}_frames`, {
                                template: outputTemplate,
                                prefix: outputPrefix,
                                seq,
                                op: 'gif_frames',
                                date: batchTime,
                            });
                            const rawOutputDir = relDir ? joinPath(outDir, `${relDir}/${folderName}`) : joinPath(outDir, folderName);
                            const outputDir = await resolveUniquePath(rawOutputDir);
                            const req = {
                                action: 'export_frames',
                                input_path: normalizePath(f.input_path),
                                output_dir: outputDir,
                                output_format: outputFormat,
                            };
                            try {
                                const res = await appAny.SplitGIF(req);
                                if (!res?.success) {
                                    failed++;
                                    reportTaskFailure('GIF 导出', f.input_path, res?.error, '导出失败');
                                }
                            } catch (err) {
                                console.error(`Failed to export GIF frames ${f.input_path}:`, err);
                                failed++;
                                reportTaskFailure('GIF 导出', f.input_path, err, '导出失败');
                            }
                            completed++;
                            seq += 1;
                            setProgress((completed / gifFiles.length) * 100);
                        }
                        const extra = failed > 0 ? `（失败 ${failed}）` : '';
                        setLastMessage(`导出完成：${completed}/${gifFiles.length} 项${extra}`);
                        return;
                    }

                    const first = files[0];
                    const baseName = dropResult?.has_directory && first?.source_root
                        ? basename(first.source_root)
                        : basename(first.input_path).replace(/\.[^.]+$/, '');
                    const safeName = baseName || 'output';
                    const combinedName = buildOutputName(`${safeName}_combined`, {
                        template: outputTemplate,
                        prefix: outputPrefix,
                        seq: 1,
                        op: 'gif_build',
                        date: batchTime,
                    });
                    const outputPath = await resolveUniquePath(joinPath(outDir, `${combinedName}.gif`));
                    try {
                        const res = await appAny.SplitGIF({
                            action: 'build_gif',
                            input_paths: files.map(f => normalizePath(f.input_path)),
                            output_path: outputPath,
                            fps: gifBuildFps,
                        });
                        if (res?.success) {
                            setLastMessage(`合成完成：${res.output_path || outputPath}`);
                        } else {
                            reportTaskFailure('GIF 合成', files[0]?.input_path || '', res?.error, '合成失败');
                            setLastMessage(res?.error || '合成失败');
                        }
                    } catch (err) {
                        console.error('Failed to build GIF:', err);
                        reportTaskFailure('GIF 合成', files[0]?.input_path || '', err, '合成失败');
                        setLastMessage('合成失败');
                    } finally {
                        setProgress(100);
                    }
                    return;
                }

                if (gifFiles.length === 0) {
                    setLastMessage('请先选择 GIF 文件');
                    return;
                }
                if (otherFiles.length > 0) {
                    setLastMessage('倒放与修改帧率只支持 GIF 输入');
                    return;
                }

                const action = gifMode === '倒放' ? 'reverse' : 'change_speed';
                const speedFactor = gifSpeedPercent / 100;
                let failed = 0;
                let seq = 1;
                for (const f of gifFiles) {
                    const suffix = action === 'reverse' ? '_reverse' : `_speed_${gifSpeedPercent}`;
                    const outRel = buildOutputRelPath(f, {
                        suffix,
                        seq,
                        op: 'gif',
                        template: outputTemplate,
                        prefix: outputPrefix,
                        preserveStructure,
                        date: batchTime,
                    });
                    const outputPath = await resolveUniquePath(joinPath(outDir, outRel));
                    const req: any = {
                        action,
                        input_path: normalizePath(f.input_path),
                        output_path: outputPath,
                    };
                    if (action === 'change_speed') {
                        req.speed_factor = speedFactor;
                    }
                    try {
                        const res = await appAny.SplitGIF(req);
                        if (!res?.success) {
                            failed++;
                            reportTaskFailure(`GIF ${gifMode}`, f.input_path, res?.error, '处理失败');
                        }
                    } catch (err) {
                        console.error(`Failed to process GIF ${f.input_path}:`, err);
                        failed++;
                        reportTaskFailure(`GIF ${gifMode}`, f.input_path, err, '处理失败');
                    }
                    completed++;
                    seq += 1;
                    setProgress((completed / gifFiles.length) * 100);
                }
                const extra = failed > 0 ? `（失败 ${failed}）` : '';
                setLastMessage(`${gifMode}完成：${completed}/${gifFiles.length} 项${extra}`);
                return;
            }

            if (id === 'info') {
                const f = files[0];
                await loadInfoForPath(f.input_path);
                return;
            }

            setLastMessage('该功能暂未接入');
        } catch (e: any) {
            console.error(e);
            reportBatchTaskFailure(feature.title, dropResult?.files || [], e, '处理失败');
            setLastMessage(typeof e?.message === 'string' ? e.message : '处理失败');
        } finally {
            setIsProcessing(false);
        }
    };

    const selectedDropPath = id === 'info' ? infoFilePath : isPreviewFeature ? previewPath : undefined;
    const previewLabel = previewPath ? previewPath.replace(/\\/g, '/').split('/').pop() || '' : '';
    const dropZone = (
        <FileDropZone 
            isActive={isActive}
            onFilesSelected={handleFilesSelected}
            onPathsExpanded={(result) => {
                setDropResult(result);
                if (id !== 'info') {
                    setLastMessage('');
                    setProgress(0);
                    return;
                }
                const selected = infoFilePath
                    ? result?.files?.some((f) => normalizePath(f.input_path) === normalizePath(infoFilePath))
                    : false;
                if (selected) return;
                const first = result?.files?.[0];
                if (first?.input_path) {
                    loadInfoForPath(first.input_path);
                    return;
                }
                setInfoFilePath('');
                setInfoPreview(null);
                setLastMessage('');
            }}
            onItemSelect={(file) => {
                if (!file?.input_path) return;
                if (id === 'info') {
                    loadInfoForPath(file.input_path);
                    return;
                }
                if (isPreviewFeature) {
                    setPreviewPath(normalizePath(file.input_path));
                }
            }}
            selectedPath={selectedDropPath}
            acceptedFormats="image/*,.svg"
            allowMultiple={true}
            title="拖拽文件 / 文件夹到这里"
            subTitle=""
        />
    );

    const showInputInSettings = !isInfo && id !== 'adjust';
    const showActionInSettings = !isInfo && id !== 'adjust' && id !== 'filter';

    const renderInputSection = (compact = false) => (
        <div className={`${compact ? 'space-y-1 pb-2' : 'space-y-2 pb-3'} border-b border-gray-100 dark:border-white/5 ${compact ? 'mb-2' : 'mb-3'}`}>
            <div className="flex items-center justify-between gap-2 flex-wrap text-sm">
                <div className="flex items-center gap-2">
                    <span className="text-gray-700 dark:text-gray-300 font-medium">输入项</span>
                    <span className="text-gray-500 dark:text-gray-400 font-mono text-xs">{inputCount}</span>
                </div>
                <button
                    onClick={handleSelectOutputDir}
                    className="shrink-0 px-2.5 py-1.5 rounded-xl border border-gray-200 dark:border-white/10 hover:bg-gray-50 dark:hover:bg-white/5 transition-all text-[13px] font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-[#2C2C2E]"
                >
                    选择输出位置
                </button>
            </div>
            {effectiveOutputDir && (
                <div
                    className="text-xs text-gray-500 dark:text-gray-400 font-mono break-all bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 rounded-xl px-3 py-2"
                    style={clampTwoLinesStyle}
                >
                    {effectiveOutputDir}
                </div>
            )}
            {lastMessage && (
                <div
                    className="text-xs text-gray-600 dark:text-gray-300 bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 rounded-xl px-3 py-2 break-all"
                    style={clampTwoLinesStyle}
                >
                    {lastMessage}
                </div>
            )}
        </div>
    );

    const renderActionContent = (compact = false) => (
        <>
            {(isProcessing || progress > 0) && (
                <ProgressBar progress={progress} label={isProcessing ? "正在处理..." : "已完成"} />
            )}
            <button 
                onClick={handleStartProcessing} 
                disabled={isProcessing}
                className={`w-full ${compact ? 'py-3' : 'py-3.5'} rounded-xl font-semibold shadow-lg shadow-blue-500/25 transition-all active:scale-[0.98] flex items-center justify-center gap-2 text-white ${isProcessing ? 'bg-gray-400 cursor-not-allowed' : 'bg-gradient-to-r from-[#007AFF] to-[#0055FF] hover:to-[#0044DD]'}`}
            >
                <Icon name="Wand2" size={18} /> {isProcessing ? '处理中...' : '开始处理'}
            </button>
        </>
    );

    const renderActionSection = (compact = false) => (
        <div className={`${compact ? 'pt-3' : 'pt-4'} border-t border-gray-100 dark:border-white/5 mt-auto shrink-0 space-y-3`}>
            {renderActionContent(compact)}
        </div>
    );

    const filterControlsPanel = (
        <FilterControls
            intensity={filterIntensity}
            setIntensity={setFilterIntensity}
            grain={filterGrain}
            setGrain={setFilterGrain}
            vignette={filterVignette}
            setVignette={setFilterVignette}
            footer={renderActionContent(true)}
        />
    );

    const settingsPanel = (
        <div
            className={`bg-white dark:bg-[#2C2C2E] rounded-3xl p-5 shadow-sm border border-gray-100 dark:border-white/5 flex flex-col overflow-hidden h-full min-h-0 ${isInfo ? 'lg:col-span-4' : ''}`}
        >
            {showInputInSettings && renderInputSection()}
            {isInfo && lastMessage && (
                <div
                    className="mb-4 text-xs text-gray-600 dark:text-gray-300 bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 rounded-xl px-3 py-2 break-all"
                    style={clampTwoLinesStyle}
                >
                    {lastMessage}
                </div>
            )}
            
            <div className="flex-1 min-h-0 overflow-y-auto no-scrollbar px-1 pb-2">
                {renderSettings()}
            </div>

            {showActionInSettings && renderActionSection()}
        </div>
    );

    const watermarkPreviewOverlay = isWatermark && !isCompareActive && previewSrc && previewBaseMetrics && watermarkPreviewConfig ? (
        <div className="absolute inset-0 pointer-events-none">
            {watermarkFontFace?.css && (
                <style>{watermarkFontFace.css}</style>
            )}
            <div
                className="absolute"
                style={{
                    left: previewBaseMetrics.left,
                    top: previewBaseMetrics.top,
                    width: previewBaseMetrics.width,
                    height: previewBaseMetrics.height,
                    overflow: 'hidden',
                }}
            >
                {watermarkTiled && watermarkTilePositions && watermarkTileMetrics ? (
                    watermarkTilePositions.map((pos, idx) => (
                        watermarkType === '图片' ? (
                            watermarkImageSrc ? (
                                <img
                                    key={`wm-img-${idx}`}
                                    src={watermarkImageSrc}
                                    className="absolute"
                                    style={{
                                        left: pos.x,
                                        top: pos.y,
                                        width: watermarkTileMetrics.width,
                                        height: watermarkTileMetrics.height,
                                        opacity: watermarkPreviewConfig.opacity,
                                        mixBlendMode: watermarkPreviewConfig.blendMode as any,
                                        transform: `rotate(${watermarkRotate}deg)`,
                                        transformOrigin: 'center',
                                        filter: watermarkShadow ? 'drop-shadow(0 2px 6px rgba(0,0,0,0.35))' : undefined,
                                    }}
                                    alt="watermark-tile"
                                />
                            ) : null
                        ) : (
                            <div
                                key={`wm-text-${idx}`}
                                className="absolute"
                                style={{
                                    left: pos.x,
                                    top: pos.y,
                                    fontFamily: watermarkFontFamily,
                                    fontSize: watermarkPreviewConfig.previewFontSize,
                                    lineHeight: 1.2,
                                    color: watermarkColor,
                                    opacity: watermarkPreviewConfig.opacity,
                                    mixBlendMode: watermarkPreviewConfig.blendMode as any,
                                    transform: `rotate(${watermarkRotate}deg)`,
                                    transformOrigin: 'center',
                                    whiteSpace: 'pre-line',
                                    textShadow: watermarkShadow ? '0 2px 6px rgba(0,0,0,0.35)' : undefined,
                                }}
                            >
                                {watermarkPreviewConfig.textValue}
                            </div>
                        )
                    ))
                ) : (
                    watermarkType === '图片' ? (
                        watermarkImageSrc && watermarkImagePreviewSize ? (
                            <img
                                src={watermarkImageSrc}
                                className="absolute"
                                style={{
                                    ...watermarkPositionStyle,
                                    width: watermarkImagePreviewSize.width,
                                    height: watermarkImagePreviewSize.height,
                                    opacity: watermarkPreviewConfig.opacity,
                                    mixBlendMode: watermarkPreviewConfig.blendMode as any,
                                    filter: watermarkShadow ? 'drop-shadow(0 2px 6px rgba(0,0,0,0.35))' : undefined,
                                }}
                                alt="watermark"
                            />
                        ) : null
                    ) : (
                        <div
                            className="absolute"
                            style={{
                                ...watermarkPositionStyle,
                                fontFamily: watermarkFontFamily,
                                fontSize: watermarkPreviewConfig.previewFontSize,
                                lineHeight: 1.2,
                                color: watermarkColor,
                                opacity: watermarkPreviewConfig.opacity,
                                mixBlendMode: watermarkPreviewConfig.blendMode as any,
                                whiteSpace: 'pre-line',
                                textShadow: watermarkShadow ? '0 2px 6px rgba(0,0,0,0.35)' : undefined,
                            }}
                        >
                            {watermarkPreviewConfig.textValue}
                        </div>
                    )
                )}
            </div>
        </div>
    ) : null;

    const previewPanel = (
        <div className="bg-white dark:bg-[#2C2C2E] rounded-3xl p-5 shadow-sm border border-gray-100 dark:border-white/5 flex flex-col h-full min-h-0 overflow-hidden">
            <div className="flex items-center justify-between mb-4 gap-3">
                <span className="text-sm font-medium text-gray-700 dark:text-gray-300">实时预览</span>
                <div className="flex items-center gap-2">
                    {previewLabel && (
                        <span className="text-[11px] text-gray-400 font-mono truncate max-w-[180px]">
                            {previewLabel}
                        </span>
                    )}
                    <button
                        type="button"
                        disabled={!previewSrc}
                        aria-pressed={isCompareActive}
                        onPointerDown={() => setIsComparing(true)}
                        onPointerUp={() => setIsComparing(false)}
                        onPointerLeave={() => setIsComparing(false)}
                        onPointerCancel={() => setIsComparing(false)}
                        onBlur={() => setIsComparing(false)}
                        className={`flex items-center gap-1.5 text-[11px] px-2 py-1 rounded-full border transition ${
                            previewSrc
                                ? 'text-gray-600 dark:text-gray-300 border-gray-200 dark:border-white/10 bg-gray-50 dark:bg-white/5 hover:text-gray-900 dark:hover:text-white'
                                : 'text-gray-300 dark:text-gray-600 border-gray-100 dark:border-white/5 bg-gray-50 dark:bg-white/5 cursor-not-allowed'
                        }`}
                    >
                        <Icon name="Layers" size={12} />
                        {isCompareActive ? '原图' : '按住对比'}
                    </button>
                </div>
            </div>
            <div
                ref={previewContainerRef}
                className="relative w-full flex-1 min-h-[220px] rounded-2xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 overflow-hidden flex items-center justify-center"
            >
                {previewSrc ? (
                    showCropFrame && cropImageMetrics ? (
                        <div className="w-full h-full relative">
                            <div className="absolute inset-0 flex items-center justify-center pointer-events-none select-none">
                                <div
                                    className="relative"
                                    style={{
                                        width: cropImageMetrics.drawnWidth,
                                        height: cropImageMetrics.drawnHeight,
                                        transform: `translate(${cropOffset.x}px, ${cropOffset.y}px)`,
                                    }}
                                >
                                    <div
                                        className="relative w-full h-full"
                                        style={{
                                            transform: effectivePreviewTransform || 'none',
                                            transformOrigin: 'center',
                                        }}
                                    >
                                        <img
                                            src={previewSrc}
                                            className="w-full h-full object-cover transition-all duration-150 pointer-events-none"
                                            style={{
                                                filter: effectivePreviewFilter || 'none',
                                            }}
                                            onLoad={(e) => {
                                                const img = e.currentTarget;
                                                if (img.naturalWidth && img.naturalHeight) {
                                                    setPreviewImageSize({
                                                        width: img.naturalWidth,
                                                        height: img.naturalHeight,
                                                    });
                                                }
                                            }}
                                            alt="preview"
                                        />
                                        <div
                                            className="pointer-events-none absolute inset-0 transition-opacity duration-150"
                                            style={{
                                                opacity: effectiveVignetteOpacity,
                                                background: 'radial-gradient(circle at center, rgba(0,0,0,0) 35%, rgba(0,0,0,0.85) 100%)',
                                            }}
                                        />
                                        <div
                                            className="pointer-events-none absolute inset-0 mix-blend-soft-light transition-opacity duration-150"
                                            style={{
                                                opacity: effectiveGrainOpacity,
                                                backgroundImage: 'repeating-linear-gradient(0deg, rgba(0,0,0,0.2) 0, rgba(0,0,0,0.2) 1px, rgba(0,0,0,0) 1px, rgba(0,0,0,0) 3px), repeating-linear-gradient(90deg, rgba(0,0,0,0.15) 0, rgba(0,0,0,0.15) 1px, rgba(0,0,0,0) 1px, rgba(0,0,0,0) 4px)',
                                            }}
                                        />
                                    </div>
                                </div>
                            </div>
                            <div
                                className={`absolute border-2 border-dashed border-gray-300/80 dark:border-white/40 ${isCropInteractive ? (isCropDragging ? 'cursor-grabbing' : 'cursor-grab') : ''}`}
                                style={{
                                    width: previewFrame.width,
                                    height: previewFrame.height,
                                    left: previewFrame.left,
                                    top: previewFrame.top,
                                    boxShadow: '0 0 0 9999px rgba(0,0,0,0.35)',
                                    touchAction: 'none',
                                }}
                                onPointerDown={handleCropPointerDown}
                                onPointerMove={handleCropPointerMove}
                                onPointerUp={handleCropPointerUp}
                                onPointerCancel={handleCropPointerUp}
                            />
                        </div>
                    ) : (
                        <div className="w-full h-full flex items-center justify-center pointer-events-none select-none">
                            <div
                                className="relative w-full h-full"
                                style={{
                                    transform: effectivePreviewTransform || 'none',
                                    transformOrigin: 'center',
                                }}
                            >
                                <img
                                    src={previewSrc}
                                    className="w-full h-full object-contain transition-all duration-150 pointer-events-none"
                                    style={{
                                        filter: effectivePreviewFilter || 'none',
                                    }}
                                    onLoad={(e) => {
                                        const img = e.currentTarget;
                                        if (img.naturalWidth && img.naturalHeight) {
                                            setPreviewImageSize({
                                                width: img.naturalWidth,
                                                height: img.naturalHeight,
                                            });
                                        }
                                    }}
                                    alt="preview"
                                />
                                <div
                                    className="pointer-events-none absolute inset-0 transition-opacity duration-150"
                                    style={{
                                        opacity: effectiveVignetteOpacity,
                                        background: 'radial-gradient(circle at center, rgba(0,0,0,0) 35%, rgba(0,0,0,0.85) 100%)',
                                    }}
                                />
                                <div
                                    className="pointer-events-none absolute inset-0 mix-blend-soft-light transition-opacity duration-150"
                                    style={{
                                        opacity: effectiveGrainOpacity,
                                        backgroundImage: 'repeating-linear-gradient(0deg, rgba(0,0,0,0.2) 0, rgba(0,0,0,0.2) 1px, rgba(0,0,0,0) 1px, rgba(0,0,0,0) 3px), repeating-linear-gradient(90deg, rgba(0,0,0,0.15) 0, rgba(0,0,0,0.15) 1px, rgba(0,0,0,0) 1px, rgba(0,0,0,0) 4px)',
                                    }}
                                />
                            </div>
                        </div>
                    )
                ) : (
                    <div className="text-xs text-gray-400">拖入图片后显示预览</div>
                )}
                {watermarkPreviewOverlay}
            </div>
        </div>
    );

    if (isWatermark) {
        return (
            <div className="h-full flex flex-col p-1">
                <div className="grid grid-cols-1 lg:grid-cols-[minmax(360px,4fr)_minmax(0,6fr)] gap-6 min-h-0">
                    <div className="h-[320px] lg:h-[420px]">
                        {dropZone}
                    </div>
                    <div className="h-[320px] lg:h-[420px]">
                        {previewPanel}
                    </div>
                </div>
                <div className="mt-6 flex-1 min-h-0">
                    {settingsPanel}
                </div>
            </div>
        );
    }

    return (
        <div className="h-full flex flex-col p-1">
            <div className={`flex-1 grid grid-cols-1 ${isInfo ? 'lg:grid-cols-6' : isAdjustOrFilter ? 'lg:grid-cols-[280px_minmax(0,1fr)]' : 'lg:grid-cols-3'} gap-6 min-h-0`}>
                {/* Left Side: Upload Area (Shared) - Replaced with FileDropZone */}
                <div className={`h-full min-h-0 ${isAdjustOrFilter ? '' : isInfo ? 'lg:col-span-2' : 'lg:col-span-2'}`}>
                    {id === 'adjust' ? (
                        <div className="h-full flex flex-col gap-4 min-h-0">
                            <div className="flex-1 min-h-0">
                                {dropZone}
                            </div>
                            <div className="bg-white dark:bg-[#2C2C2E] rounded-3xl p-4 shadow-sm border border-gray-100 dark:border-white/5 flex flex-col gap-4 min-h-0 overflow-hidden">
                                <div className="flex-1 overflow-y-auto no-scrollbar">
                                    {renderInputSection(true)}
                                    <AdjustCropControls
                                        cropRatio={adjustCropRatio}
                                        setCropRatio={setAdjustCropRatio}
                                        rotate={adjustRotate}
                                        setRotate={setAdjustRotate}
                                        flipH={adjustFlipH}
                                        setFlipH={setAdjustFlipH}
                                    />
                                </div>
                                {renderActionSection(true)}
                            </div>
                        </div>
                    ) : id === 'filter' ? (
                        <div className="h-full flex flex-col gap-4 min-h-0">
                            <div className="flex-1 min-h-0">
                                {dropZone}
                            </div>
                            <div className="shrink-0">
                                {filterControlsPanel}
                            </div>
                        </div>
                    ) : (
                        dropZone
                    )}
                </div>

                {/* Right Side: Specific Settings Panel (Secondary Menu) */}
                <div className={`h-full min-h-0 ${isAdjustOrFilter ? '' : isInfo ? 'lg:col-span-4' : ''}`}>
                    {isAdjustOrFilter ? (
                        <div className="h-full flex flex-col gap-4 min-h-0 overflow-hidden">
                            <div className={`${id === 'adjust' ? 'flex-[1.2]' : 'flex-1'} min-h-0`}>
                                {previewPanel}
                            </div>
                            <div className={`${id === 'adjust' ? 'flex-[0.8]' : 'flex-1'} min-h-0`}>
                                {settingsPanel}
                            </div>
                        </div>
                    ) : (
                        settingsPanel
                    )}
                </div>
            </div>
        </div>
    );
};

export default memo(DetailView);
