import React, { useMemo, useState, useEffect, memo } from 'react';
import Icon from './Icon';
import { FEATURES } from '../constants';
import { ViewState } from '../types';
import { Switch, StyledSlider, CustomSelect, SegmentedControl, PositionGrid, FileDropZone, ProgressBar } from './Controls';

interface DetailViewProps {
    id: ViewState;
    onBack: () => void;
    isActive?: boolean;
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
    filePrefix: string;
    setFilePrefix: (v: string) => void;
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
    filePrefix,
    setFilePrefix,
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
                            <input type="number" value={fixedWidth || ''} onChange={e => setFixedWidth(Number(e.target.value || 0))} className="w-full px-3 py-2 rounded-lg bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm focus:border-[#007AFF] focus:ring-1 focus:ring-[#007AFF] outline-none dark:text-white" placeholder="自动" />
                        </div>
                        <div className="flex-1 space-y-1">
                            <label className="text-xs text-gray-500">高度 (px)</label>
                            <input type="number" value={fixedHeight || ''} onChange={e => setFixedHeight(Number(e.target.value || 0))} className="w-full px-3 py-2 rounded-lg bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm focus:border-[#007AFF] focus:ring-1 focus:ring-[#007AFF] outline-none dark:text-white" placeholder="自动" />
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
                        <input type="number" value={longEdge || ''} onChange={e => setLongEdge(Number(e.target.value || 0))} className="w-full px-3 py-2 rounded-lg bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm focus:border-[#007AFF] focus:ring-1 focus:ring-[#007AFF] outline-none dark:text-white" placeholder="2048" />
                    </div>
                    <div className="flex-1" />
                </div>
            )}

            <div className="pt-2 border-t border-gray-100 dark:border-white/5 space-y-3">
                <Switch label="保留元数据 (EXIF)" checked={keepMetadata} onChange={setKeepMetadata} />
            </div>

            <div className="pt-2 border-t border-gray-100 dark:border-white/5 space-y-3">
                <div className="space-y-2">
                    <label className="text-sm font-medium text-gray-700 dark:text-gray-300">输出文件前缀</label>
                    <input type="text" value={filePrefix} onChange={e => setFilePrefix(e.target.value)} placeholder="例如: IF" className="w-full px-3 py-2.5 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm focus:border-[#007AFF] focus:ring-1 focus:ring-[#007AFF] outline-none dark:text-white" />
                </div>
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
    filePrefix: string;
    setFilePrefix: (v: string) => void;
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
    filePrefix,
    setFilePrefix,
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
                        <input type="number" value={targetSizeKB} onChange={e => setTargetSizeKB(Number(e.target.value))} placeholder="例如: 500" className="w-full px-4 py-2.5 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm focus:outline-none focus:ring-2 focus:ring-[#007AFF]/50 focus:border-[#007AFF] dark:text-white transition-all" />
                     </div>
                )}
            </div>

             <div className="pt-2 border-t border-gray-100 dark:border-white/5 space-y-4">
                 <div className="p-3 rounded-xl bg-green-50 dark:bg-green-500/10 text-xs text-green-700 dark:text-green-400 leading-relaxed border border-green-100 dark:border-green-500/10">
                    {mode === '无损' ? '无损优化：JPG 使用 MozJPEG，PNG 使用 OxiPNG。' : `引擎选择：${engine}。目标大小会按需逐步降低质量直至满足。`}
                </div>
                <div className="text-xs text-gray-500 dark:text-gray-400">
                    默认移除隐私元数据，保留 DPI 与色域信息。
                </div>
            </div>

            <div className="pt-2 border-t border-gray-100 dark:border-white/5 space-y-3">
                <div className="space-y-2">
                    <label className="text-sm font-medium text-gray-700 dark:text-gray-300">输出文件前缀</label>
                    <input type="text" value={filePrefix} onChange={e => setFilePrefix(e.target.value)} placeholder="例如: IF" className="w-full px-3 py-2.5 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm focus:border-[#007AFF] focus:ring-1 focus:ring-[#007AFF] outline-none dark:text-white" />
                </div>
                <Switch label="直接覆盖源文件" checked={overwriteSource} onChange={setOverwriteSource} />
            </div>
        </div>
    );
});

const WatermarkSettings = memo(() => {
    const [type, setType] = useState('文字');
    const [position, setPosition] = useState('br');
    const [opacity, setOpacity] = useState(85);
    const [rotate, setRotate] = useState(0);
    const [size, setSize] = useState(40);
    const [tiled, setTiled] = useState(false);
    const [blendMode, setBlendMode] = useState('正常');
    const [shadow, setShadow] = useState(false);
    const [margin, setMargin] = useState({x: 20, y: 20});
    // Fix: Added state for font selection
    const [font, setFont] = useState('Sans Serif');

    return (
        <div className="space-y-6">
            <div className="space-y-2">
                <label className="text-sm font-medium text-gray-700 dark:text-gray-300">水印来源</label>
                <SegmentedControl options={['文字', '图片']} value={type} onChange={setType} />
            </div>

            {type === '文字' ? (
                <div className="space-y-3 animate-enter">
                    <input type="text" placeholder="© ImageFlow Pro" className="w-full px-4 py-2.5 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm focus:outline-none focus:ring-2 focus:ring-[#007AFF]/50 focus:border-[#007AFF] dark:text-white transition-all" />
                    <div className="flex gap-2">
                         <div className="w-10 h-10 rounded-lg bg-black cursor-pointer border-2 border-white/20 shadow-sm shrink-0" title="Text Color" />
                         <div className="flex-1">
                             <CustomSelect 
                                options={['Sans Serif', 'Serif', 'Mono', 'Handwriting']} 
                                value={font} 
                                onChange={setFont} 
                             />
                         </div>
                    </div>
                </div>
            ) : (
                <div className="w-full h-24 rounded-xl bg-gray-50 dark:bg-white/5 border-2 border-dashed border-gray-200 dark:border-white/10 flex flex-col items-center justify-center text-sm text-gray-500 cursor-pointer hover:bg-gray-100 dark:hover:bg-white/10 transition-colors animate-enter">
                    <Icon name="Upload" size={20} className="mb-2 opacity-50" />
                    <span>点击上传水印图</span>
                </div>
            )}

            <div className="grid grid-cols-[1fr_auto] gap-6 pt-2">
                <div className="space-y-5">
                    <StyledSlider label="不透明度" value={opacity} onChange={setOpacity} unit="%" />
                    <StyledSlider label="尺寸缩放" value={size} onChange={setSize} unit="%" />
                    <StyledSlider label="旋转角度" value={rotate} min={-180} max={180} onChange={setRotate} unit="°" />
                </div>
                <div className="space-y-2">
                    <label className="text-sm font-medium text-gray-700 dark:text-gray-300 block text-center">锚点位置</label>
                    <PositionGrid value={position} onChange={setPosition} />
                </div>
            </div>

            <div className="grid grid-cols-2 gap-4">
                 <div className="space-y-1">
                     <label className="text-xs text-gray-500">水平边距 X</label>
                     <input type="number" value={margin.x} onChange={e => setMargin({...margin, x: Number(e.target.value)})} className="w-full px-3 py-2 rounded-lg bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm outline-none dark:text-white" />
                 </div>
                 <div className="space-y-1">
                     <label className="text-xs text-gray-500">垂直边距 Y</label>
                     <input type="number" value={margin.y} onChange={e => setMargin({...margin, y: Number(e.target.value)})} className="w-full px-3 py-2 rounded-lg bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm outline-none dark:text-white" />
                 </div>
            </div>

            <div className="pt-2 border-t border-gray-100 dark:border-white/5 space-y-4">
                <CustomSelect label="混合模式" options={['正常', '正片叠底 (Multiply)', '滤色 (Screen)', '叠加 (Overlay)', '柔光 (Soft Light)']} value={blendMode} onChange={setBlendMode} />
                <Switch label="添加投影 (Shadow)" checked={shadow} onChange={setShadow} />
                <Switch label="全屏平铺模式" checked={tiled} onChange={setTiled} />
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
};

const AdjustCropControls = memo(() => (
    <div className="space-y-2">
        <label className="text-sm font-medium text-gray-700 dark:text-gray-300">裁剪与旋转</label>
        <div className="flex flex-wrap gap-2">
            {['自由', '1:1', '4:3', '16:9', '9:16', '3:2'].map(r => (
                <button key={r} className="px-3 py-1.5 rounded-lg border border-gray-200 dark:border-white/10 text-xs font-medium bg-white dark:bg-white/5 whitespace-nowrap hover:border-[#007AFF] hover:text-[#007AFF] transition-colors">{r}</button>
            ))}
        </div>
        <div className="grid grid-cols-2 gap-2 mt-2">
            <button className="flex items-center justify-center gap-2 py-2 rounded-xl border border-gray-200 dark:border-white/10 hover:bg-gray-50 dark:hover:bg-white/5 transition-all text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-[#2C2C2E]">
                <Icon name="RotateCw" size={16} /> 向右旋转 90°
            </button>
            <button className="flex items-center justify-center gap-2 py-2 rounded-xl border border-gray-200 dark:border-white/10 hover:bg-gray-50 dark:hover:bg-white/5 transition-all text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-[#2C2C2E]">
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
}: AdjustSettingsProps) => {

    return (
        <div className="space-y-4">
            {showCrop && <AdjustCropControls />}

            <div className="pt-3 border-t border-gray-100 dark:border-white/5">
                 <label className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-4 flex items-center gap-2">
                    <Icon name="Palette" size={16} className="text-[#007AFF]" />
                    专业调色
                 </label>
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
    );
});

const FILTER_LABELS = ['原图', '鲜艳', '黑白', '复古', '冷调', '暖阳', '胶片', '赛博', '清新', '日系', 'Lomo', 'HDR', '褪色', '磨砂', '电影', '拍立得'];

type FilterSettingsProps = {
    intensity: number;
    setIntensity: (v: number) => void;
    grain: number;
    setGrain: (v: number) => void;
    vignette: number;
    setVignette: (v: number) => void;
    selected: number;
    setSelected: (v: number) => void;
};

const FilterSettings = memo(({
    intensity,
    setIntensity,
    grain,
    setGrain,
    vignette,
    setVignette,
    selected,
    setSelected,
}: FilterSettingsProps) => {

    return (
        <div className="flex flex-col h-full">
            <div className="flex-1 space-y-3 min-h-0 flex flex-col">
                <div className="flex items-center justify-between shrink-0">
                    <label className="text-sm font-medium text-gray-700 dark:text-gray-300">滤镜库 (LUTs)</label>
                    <button onClick={() => setSelected(0)} className="text-xs text-[#007AFF] hover:underline font-medium">重置效果</button>
                </div>
                <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-2 overflow-y-auto no-scrollbar pr-1 pb-1 content-start">
                    {FILTER_LABELS.map((f, i) => (
                        <button 
                            key={i} 
                            onClick={() => setSelected(i)}
                            className={`
                            group relative flex flex-col items-center gap-2 p-2 rounded-xl border transition-all duration-200 active:scale-95
                            ${selected === i 
                                ? 'bg-[#007AFF]/5 border-[#007AFF] ring-1 ring-[#007AFF]/20 shadow-sm' 
                                : 'bg-gray-50 dark:bg-white/5 border-transparent hover:border-gray-200 dark:hover:border-white/10 hover:shadow-sm'}
                        `}>
                            <div className={`w-full aspect-square rounded-lg ${i===0 ? 'bg-gray-200 dark:bg-white/10' : 'bg-gradient-to-br from-gray-200 to-gray-300 dark:from-white/5 dark:to-white/10'} overflow-hidden relative shadow-inner`}>
                                <div className={`absolute inset-0 opacity-20 ${i===1 ? 'bg-blue-500 mix-blend-overlay' : ''} ${i%2===0 && i!==0 ? 'bg-yellow-600 mix-blend-multiply' : ''} ${i%3===0 && i!==0 ? 'bg-purple-600 mix-blend-screen' : ''}`}></div>
                            </div>
                            <span className={`text-[10px] font-medium truncate w-full text-center ${selected === i ? 'text-[#007AFF]' : 'text-gray-500 dark:text-gray-400 group-hover:text-gray-700 dark:group-hover:text-gray-200'}`}>
                                {f}
                            </span>
                            {selected === i && <div className="absolute top-1.5 right-1.5 w-1.5 h-1.5 bg-[#007AFF] rounded-full shadow-sm animate-enter"></div>}
                        </button>
                    ))}
                </div>
            </div>

            <div className="shrink-0 pt-3 border-t border-gray-100 dark:border-white/5 mt-3">
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <StyledSlider label="滤镜强度" value={intensity} onChange={setIntensity} unit="%" className="space-y-2" />
                    <StyledSlider label="颗粒感 (Grain)" value={grain} onChange={setGrain} className="space-y-2" />
                    <StyledSlider label="暗角 (Vignette)" value={vignette} onChange={setVignette} className="space-y-2 col-span-2" />
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
                    className="w-full px-3 py-2 rounded-lg bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm outline-none dark:text-white"
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
                        className="w-full px-3 py-2 rounded-lg bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm outline-none dark:text-white"
                    />
                    <span className="text-xs text-gray-400 shrink-0">.pdf</span>
                </div>
            </div>

            <div className="pt-4 border-t border-gray-100 dark:border-white/5 space-y-4">
                 <div className="space-y-3">
                    <label className="text-sm font-medium text-gray-700 dark:text-gray-300">元数据</label>
                    <input type="text" value={title} onChange={(e) => setTitle(e.target.value)} placeholder="文档标题" className="w-full px-3 py-2 rounded-lg bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm outline-none dark:text-white" />
                    <input type="text" value={author} onChange={(e) => setAuthor(e.target.value)} placeholder="作者" className="w-full px-3 py-2 rounded-lg bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm outline-none dark:text-white" />
                 </div>
                <div className="h-px bg-gray-100 dark:bg-white/5" />
                <div className="space-y-2">
                    <label className="text-sm font-medium text-gray-700 dark:text-gray-300">PDF 压缩</label>
                    <SegmentedControl options={['不压缩', '轻度', '标准', '强力']} value={compression} onChange={setCompression} />
                </div>
                <div className="space-y-2">
                    <label className="text-sm font-medium text-gray-700 dark:text-gray-300">文档加密 (可选)</label>
                    <input type="password" placeholder="留空则不加密" className="w-full px-4 py-2.5 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm focus:outline-none focus:ring-2 focus:ring-[#007AFF]/50 focus:border-[#007AFF] dark:text-white transition-all" />
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

            <div className="p-3 rounded-xl bg-purple-50 dark:bg-purple-500/10 text-xs text-purple-700 dark:text-purple-400 mt-2 border border-purple-100 dark:border-purple-500/10">
                仅保留导出、倒放与帧率调整三项核心操作。
            </div>
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
                 <button onClick={onExportJSON} disabled={!info?.success} className="flex-1 py-2 rounded-lg border border-gray-200 dark:border-white/10 text-sm font-medium text-gray-600 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-white/5 transition-colors disabled:opacity-50 disabled:cursor-not-allowed">导出 JSON</button>
                 <button onClick={onClearPrivacy} disabled={!filePath} className="flex-1 py-2 rounded-lg border border-gray-200 dark:border-white/10 text-sm font-medium text-red-500 hover:bg-red-50 dark:hover:bg-red-500/10 hover:border-red-200 transition-colors disabled:opacity-50 disabled:cursor-not-allowed">清除隐私信息</button>
            </div>
        </div>
    );
});

const DetailView: React.FC<DetailViewProps> = ({ id, onBack, isActive = true }) => {
    const feature = FEATURES.find(f => f.id === id);
    if (!feature) return null;

    const isInfo = id === 'info';
    const isAdjustOrFilter = id === 'adjust' || id === 'filter';
    const [dropResult, setDropResult] = useState<ExpandDroppedPathsResult | null>(null);
    const [preserveFolderStructure, setPreserveFolderStructure] = useState(true);
    const [outputDir, setOutputDir] = useState('');
    const [isProcessing, setIsProcessing] = useState(false);
    const [progress, setProgress] = useState(0);
    const [lastMessage, setLastMessage] = useState<string>('');

    const [convFormat, setConvFormat] = useState('JPG');
    const [convQuality, setConvQuality] = useState(80);
    const [convCompressLevel, setConvCompressLevel] = useState(6); // 0-9 for PNG
    const [convIcoSizes, setConvIcoSizes] = useState<number[]>([16, 32, 48, 64, 128, 256]);
    const [convResizeMode, setConvResizeMode] = useState('原图尺寸');
    const [convScalePercent, setConvScalePercent] = useState(100);
    const [convFixedWidth, setConvFixedWidth] = useState(0);
    const [convFixedHeight, setConvFixedHeight] = useState(0);
    const [convLongEdge, setConvLongEdge] = useState(1920);
    const [convKeepMetadata, setConvKeepMetadata] = useState(false);
    const [convMaintainAR, setConvMaintainAR] = useState(true);
    const [convFilePrefix, setConvFilePrefix] = useState('IF');
    const [convOverwriteSource, setConvOverwriteSource] = useState(false);

    const [compMode, setCompMode] = useState('标准');
    const [compTargetSize, setCompTargetSize] = useState(false);
    const [compTargetSizeKB, setCompTargetSizeKB] = useState(500);
    const [compEngine, setCompEngine] = useState('自动 (推荐)');
    const [compFilePrefix, setCompFilePrefix] = useState('IF');
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
    const previewGrainOpacity = isAdjustOrFilter && id === 'filter'
        ? clampNumber(filterGrain / 100 * 0.35, 0, 0.35)
        : 0;
    const previewVignetteOpacity = isAdjustOrFilter && id === 'filter'
        ? clampNumber(filterVignette / 100 * 0.6, 0, 0.6)
        : 0;

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
                        filePrefix={convFilePrefix}
                        setFilePrefix={setConvFilePrefix}
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
                        filePrefix={compFilePrefix}
                        setFilePrefix={setCompFilePrefix}
                        overwriteSource={compOverwriteSource}
                        setOverwriteSource={setCompOverwriteSource}
                    />
                );
            case 'watermark': return <WatermarkSettings />;
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
                        const outPath = (dir ? `${dir}/` : '') + `IF_${name}`;

                        const appAny = (window.go?.main?.App as any);
                        if (!appAny?.StripMetadata) {
                            setLastMessage('后端未接入隐私清理接口');
                            return;
                        }
                        setLastMessage('');
                        const res = await appAny.StripMetadata({ input_path: infoFilePath, output_path: outPath, overwrite: false });
                        if (res?.success) {
                            setLastMessage(`隐私清理完成：${res.output_path || outPath}`);
                            const refreshed = await window.go.main.App.GetInfo({ input_path: res.output_path || outPath });
                            setInfoFilePath(res.output_path || outPath);
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

    const handleFilesSelected = (files: File[]) => {
        console.log("Files received by backend logic:", files);
        // Implementation for passing files to Go/Rust backend would go here
    };

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
                const dir = await window.runtime.OpenDirectoryDialog({ title: '选择输出文件夹' });
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
    const addSuffix = (p: string, suffix: string) => {
        const normalized = p.replace(/\\/g, '/');
        const idx = normalized.lastIndexOf('.');
        if (idx === -1) return `${normalized}${suffix}`;
        return `${normalized.slice(0, idx)}${suffix}${normalized.slice(idx)}`;
    };
    const sanitizeFilePrefix = (prefix: string) => (prefix || '').trim().replace(/[\\/:*?"<>|]+/g, '_');
    const sanitizeFileName = (name: string) => (name || '').trim().replace(/[\\/:*?"<>|]+/g, '_');
    const stripExtension = (name: string) => {
        const idx = name.lastIndexOf('.');
        if (idx <= 0) return name;
        return name.slice(0, idx);
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
                return `saturate(${mix(1, 1.8)}) contrast(${mix(1, 1.2)})`;
            case 2: // 黑白
                return `grayscale(${t}) contrast(${mix(1, 1.1)})`;
            case 3: // 复古
                return `sepia(${mix(0, 0.8)}) contrast(${mix(1, 1.15)})`;
            case 4: // 冷调
                return `saturate(${mix(1, 1.2)}) hue-rotate(${mix(0, -18)}deg)`;
            case 5: // 暖阳
                return `sepia(${mix(0, 0.35)}) hue-rotate(${mix(0, 12)}deg) saturate(${mix(1, 1.2)})`;
            case 6: // 胶片
                return `sepia(${mix(0, 0.4)}) contrast(${mix(1, 1.2)}) brightness(${mix(1, 1.05)})`;
            case 7: // 赛博
                return `hue-rotate(${mix(0, 90)}deg) saturate(${mix(1, 1.5)}) contrast(${mix(1, 1.1)})`;
            case 8: // 清新
                return `brightness(${mix(1, 1.08)}) saturate(${mix(1, 1.15)})`;
            case 9: // 日系
                return `brightness(${mix(1, 1.12)}) contrast(${mix(1, 0.95)}) saturate(${mix(1, 1.05)})`;
            case 10: // Lomo
                return `contrast(${mix(1, 1.25)}) saturate(${mix(1, 1.35)})`;
            case 11: // HDR
                return `contrast(${mix(1, 1.4)}) saturate(${mix(1, 1.2)})`;
            case 12: // 褪色
                return `saturate(${mix(1, 0.7)}) brightness(${mix(1, 1.08)})`;
            case 13: // 磨砂
                return `blur(${mix(0, 2.2)}px)`;
            case 14: // 电影
                return `contrast(${mix(1, 1.18)}) saturate(${mix(1, 1.1)}) brightness(${mix(1, 0.98)})`;
            case 15: // 拍立得
                return `sepia(${mix(0, 0.55)}) contrast(${mix(1, 1.1)})`;
            default:
                return '';
        }
    }
    const addPrefixToRelPath = (rel: string, prefix: string) => {
        const p = sanitizeFilePrefix(prefix);
        if (!p) return rel;
        const normalized = rel.replace(/\\/g, '/');
        const idx = normalized.lastIndexOf('/');
        const dir = idx >= 0 ? normalized.slice(0, idx + 1) : '';
        const name = idx >= 0 ? normalized.slice(idx + 1) : normalized;
        return `${dir}${p}_${name}`;
    };

    useEffect(() => {
        if (!dropResult || dropResult.files.length === 0) {
            setPdfFileName('');
            return;
        }
        const suggested = buildSuggestedPdfName(dropResult.files, dropResult.has_directory);
        setPdfFileName(suggested);
    }, [dropResult]);

    useEffect(() => {
        if (!isAdjustOrFilter) {
            setPreviewPath('');
            return;
        }
        const first = dropResult?.files?.[0];
        if (first?.input_path) {
            setPreviewPath(normalizePath(first.input_path));
        } else {
            setPreviewPath('');
        }
    }, [dropResult, isAdjustOrFilter]);

    useEffect(() => {
        if (!isAdjustOrFilter) {
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
    }, [previewPath, isAdjustOrFilter]);

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
                const buildReq = (f: DroppedFile) => {
                    const input_path = normalizePath(f.input_path);
                    const rel = preserveFolderStructure && f.is_from_dir_drop ? f.relative_path : basename(f.input_path);
                    const baseOutRel = replaceExt(rel, format);
                    const outRel = baseOutRel;
                    const namedRel = convOverwriteSource ? outRel : addPrefixToRelPath(outRel, convFilePrefix);
                    const canOverwrite = convOverwriteSource && matchesFormat(format, extname(f.input_path));
                    const output_path = canOverwrite ? input_path : joinPath(outDir, namedRel);
                    return {
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
                    };
                };

                const requests = files.map((f) => buildReq(f));

                const chunkSize = requests.length >= 80 ? 20 : 1;
                for (let i = 0; i < requests.length; i += chunkSize) {
                    const chunk = requests.slice(i, i + chunkSize);
                    try {
                        if (chunk.length === 1) {
                            await window.go.main.App.Convert(chunk[0]);
                        } else {
                            await window.go.main.App.ConvertBatch(chunk);
                        }
                    } catch (err) {
                        console.error(err);
                    }

                    completed += chunk.length;
                    setProgress((completed / requests.length) * 100);
                }
                setLastMessage(`转换完成：${completed}/${requests.length} 项`);
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

                const buildReq = (f: DroppedFile) => {
                    const rel = preserveFolderStructure && f.is_from_dir_drop ? f.relative_path : basename(f.input_path);
                    if (compOverwriteSource) {
                        return {
                            input_path: normalizePath(f.input_path),
                            output_path: normalizePath(f.input_path),
                            level,
                            engine,
                            target_size_kb: targetSizeKB,
                            strip_metadata: true,
                        };
                    }
                    const namedRel = addPrefixToRelPath(rel, compFilePrefix);
                    return {
                        input_path: normalizePath(f.input_path),
                        output_path: joinPath(outDir, namedRel),
                        level,
                        engine,
                        target_size_kb: targetSizeKB,
                        strip_metadata: true,
                    };
                };

                let failed = 0;
                let warned = 0;

                const chunkSize = total >= 80 ? 20 : 1;
                for (let i = 0; i < files.length; i += chunkSize) {
                    const chunk = files.slice(i, i + chunkSize).map(buildReq);
                    try {
                        if (chunk.length === 1) {
                            const res = await window.go.main.App.Compress(chunk[0]);
                            if ((res as any)?.warning) warned++;
                        } else {
                            const res = await window.go.main.App.CompressBatch(chunk);
                            warned += (res as any[]).filter(r => r?.warning).length;
                        }
                    } catch (err) {
                        console.error(err);
                        failed += chunk.length;
                    }

                    completed += chunk.length;
                    setProgress((completed / total) * 100);
                }
                const extra = failed > 0 || warned > 0 ? `（失败 ${failed}，未达目标 ${warned}）` : '';
                setLastMessage(`压缩完成：${completed}/${total} 项${extra}`);
                return;
            }

            if (id === 'watermark') {
                for (const f of files) {
                    const rel = preserveFolderStructure && f.is_from_dir_drop ? f.relative_path : basename(f.input_path);
                    const outRel = addSuffix(rel, '_watermark');
                    const req = {
                        input_path: normalizePath(f.input_path),
                        output_path: joinPath(outDir, outRel),
                        watermark_type: 'text',
                        text: '© ImageFlow',
                        image_path: '',
                        position: 'br',
                        opacity: 0.85,
                        scale: 0.2,
                        font_size: 36,
                        font_color: '#FFFFFF',
                        rotation: 0,
                    };
                    try {
                        await window.go.main.App.AddWatermark(req);
                    } catch (err) {
                        console.error(`Failed to watermark ${f.input_path}:`, err);
                    }
                    completed++;
                    setProgress((completed / total) * 100);
                }
                setLastMessage(`水印完成：${completed}/${total} 项`);
                return;
            }

            if (id === 'adjust') {
                for (const f of files) {
                    const rel = preserveFolderStructure && f.is_from_dir_drop ? f.relative_path : basename(f.input_path);
                    const outRel = addSuffix(rel, '_adjusted');
                    const req = {
                        input_path: normalizePath(f.input_path),
                        output_path: joinPath(outDir, outRel),
                        rotate: 0,
                        flip_h: false,
                        flip_v: false,
                        brightness: 0,
                        contrast: 0,
                        saturation: 0,
                        hue: 0,
                    };
                    try {
                        await window.go.main.App.Adjust(req);
                    } catch (err) {
                        console.error(`Failed to adjust ${f.input_path}:`, err);
                    }
                    completed++;
                    setProgress((completed / total) * 100);
                }
                setLastMessage(`调整完成：${completed}/${total} 项`);
                return;
            }

            if (id === 'filter') {
                for (const f of files) {
                    const rel = preserveFolderStructure && f.is_from_dir_drop ? f.relative_path : basename(f.input_path);
                    const outRel = addSuffix(rel, '_filtered');
                    const req = {
                        input_path: normalizePath(f.input_path),
                        output_path: joinPath(outDir, outRel),
                        filter_type: 'grayscale',
                        intensity: 1.0,
                    };
                    try {
                        await window.go.main.App.ApplyFilter(req);
                    } catch (err) {
                        console.error(`Failed to filter ${f.input_path}:`, err);
                    }
                    completed++;
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
                const pdfBaseName = normalizePdfFileName(pdfFileName);
                const req: any = {
                    image_paths: files.map(f => normalizePath(f.input_path)),
                    output_path: joinPath(outDir, `${pdfBaseName}.pdf`),
                    page_size: sizeMap[pdfSize] ?? 'A4',
                    layout: pdfLayout === '横向' ? 'landscape' : 'portrait',
                    margin: marginPoints,
                    compression_level: compressionMap[pdfCompression] ?? 0,
                    fit_mode: fitMap[pdfFit] ?? 'contain',
                    title: pdfTitle.trim(),
                    author: pdfAuthor.trim(),
                };
                await window.go.main.App.GeneratePDF(req);
                setProgress(100);
                setLastMessage(`PDF 已生成：${req.output_path}`);
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
                        for (const f of gifFiles) {
                            const name = basename(f.input_path).replace(/\.[^.]+$/, '');
                            const outputDir = joinPath(outDir, `${name}_frames`);
                            const req = {
                                action: 'export_frames',
                                input_path: normalizePath(f.input_path),
                                output_dir: outputDir,
                                output_format: outputFormat,
                            };
                            try {
                                const res = await appAny.SplitGIF(req);
                                if (!res?.success) failed++;
                            } catch (err) {
                                console.error(`Failed to export GIF frames ${f.input_path}:`, err);
                                failed++;
                            }
                            completed++;
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
                    const outputPath = joinPath(outDir, `${safeName}_combined.gif`);
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
                            setLastMessage(res?.error || '合成失败');
                        }
                    } catch (err) {
                        console.error('Failed to build GIF:', err);
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
                for (const f of gifFiles) {
                    const rel = preserveFolderStructure && f.is_from_dir_drop ? f.relative_path : basename(f.input_path);
                    const suffix = action === 'reverse' ? '_reverse' : `_speed_${gifSpeedPercent}`;
                    const outRel = addSuffix(rel, suffix);
                    const outputPath = joinPath(outDir, outRel);
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
                        if (!res?.success) failed++;
                    } catch (err) {
                        console.error(`Failed to process GIF ${f.input_path}:`, err);
                        failed++;
                    }
                    completed++;
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
            setLastMessage(typeof e?.message === 'string' ? e.message : '处理失败');
        } finally {
            setIsProcessing(false);
        }
    };

    const selectedDropPath = id === 'info' ? infoFilePath : isAdjustOrFilter ? previewPath : undefined;
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
                if (isAdjustOrFilter) {
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
    const showActionInSettings = !isInfo && id !== 'adjust';

    const renderInputSection = (compact = false) => (
        <div className={`${compact ? 'space-y-2 pb-3' : 'space-y-3 pb-4'} border-b border-gray-100 dark:border-white/5 ${compact ? 'mb-3' : 'mb-4'}`}>
            <div className="flex items-center justify-between text-sm">
                <span className="text-gray-700 dark:text-gray-300 font-medium">输入项</span>
                <span className="text-gray-500 dark:text-gray-400 font-mono text-xs">{inputCount}</span>
            </div>
            {dropResult?.has_directory && (
                <Switch label="保持原文件夹结构" checked={preserveFolderStructure} onChange={setPreserveFolderStructure} />
            )}
            <button
                onClick={handleSelectOutputDir}
                className="w-full py-2.5 rounded-xl border border-gray-200 dark:border-white/10 hover:bg-gray-50 dark:hover:bg-white/5 transition-all text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-[#2C2C2E]"
            >
                选择输出文件夹
            </button>
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

    const renderActionSection = (compact = false) => (
        <div className={`${compact ? 'pt-3' : 'pt-4'} border-t border-gray-100 dark:border-white/5 mt-auto shrink-0 space-y-3`}>
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
        </div>
    );

    const settingsPanel = (
        <div
            className={`bg-white dark:bg-[#2C2C2E] rounded-3xl p-5 shadow-sm border border-gray-100 dark:border-white/5 flex flex-col overflow-hidden ${
                isAdjustOrFilter ? 'flex-1 min-h-0' : 'h-full'
            } ${isInfo ? 'lg:col-span-4' : ''}`}
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
            
            <div className="flex-1 overflow-y-auto no-scrollbar px-1 pb-2">
                {renderSettings()}
            </div>

            {showActionInSettings && renderActionSection()}
        </div>
    );

    const previewPanel = (
        <div className="bg-white dark:bg-[#2C2C2E] rounded-3xl p-5 shadow-sm border border-gray-100 dark:border-white/5 flex flex-col h-full min-h-0 overflow-hidden">
            <div className="flex items-center justify-between mb-4">
                <span className="text-sm font-medium text-gray-700 dark:text-gray-300">实时预览</span>
                {previewLabel && (
                    <span className="text-[11px] text-gray-400 font-mono truncate max-w-[180px]">
                        {previewLabel}
                    </span>
                )}
            </div>
            <div className="relative w-full flex-1 min-h-[180px] rounded-2xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 overflow-hidden flex items-center justify-center">
                {previewSrc ? (
                    <>
                        <img
                            src={previewSrc}
                            className="max-w-full max-h-full object-contain transition-all duration-150"
                            style={{ filter: previewFilter || 'none' }}
                            alt="preview"
                        />
                        <div
                            className="pointer-events-none absolute inset-0 transition-opacity duration-150"
                            style={{
                                opacity: previewVignetteOpacity,
                                background: 'radial-gradient(circle at center, rgba(0,0,0,0) 35%, rgba(0,0,0,0.85) 100%)',
                            }}
                        />
                        <div
                            className="pointer-events-none absolute inset-0 mix-blend-soft-light transition-opacity duration-150"
                            style={{
                                opacity: previewGrainOpacity,
                                backgroundImage: 'repeating-linear-gradient(0deg, rgba(0,0,0,0.2) 0, rgba(0,0,0,0.2) 1px, rgba(0,0,0,0) 1px, rgba(0,0,0,0) 3px), repeating-linear-gradient(90deg, rgba(0,0,0,0.15) 0, rgba(0,0,0,0.15) 1px, rgba(0,0,0,0) 1px, rgba(0,0,0,0) 4px)',
                            }}
                        />
                    </>
                ) : (
                    <div className="text-xs text-gray-400">拖入图片后显示预览</div>
                )}
            </div>
        </div>
    );

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
                                    <AdjustCropControls />
                                </div>
                                {renderActionSection(true)}
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
                            <div className="flex-1 min-h-0">
                                {previewPanel}
                            </div>
                            <div className="shrink-0">
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

export default DetailView;
