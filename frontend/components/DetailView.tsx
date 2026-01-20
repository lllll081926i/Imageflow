import React, { useMemo, useState, memo } from 'react';
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

const AdjustSettings = memo(() => {
    const [exposure, setExposure] = useState(0);
    const [contrast, setContrast] = useState(0);
    const [saturation, setSaturation] = useState(0);
    const [sharpness, setSharpness] = useState(0);
    const [vibrance, setVibrance] = useState(0);
    const [hue, setHue] = useState(0);

    return (
        <div className="space-y-6">
            <div className="space-y-3">
                <label className="text-sm font-medium text-gray-700 dark:text-gray-300">裁剪与旋转</label>
                <div className="flex gap-2 overflow-x-auto no-scrollbar pb-1">
                    {['自由', '1:1', '4:3', '16:9', '9:16', '3:2'].map(r => (
                         <button key={r} className="px-3 py-1.5 rounded-lg border border-gray-200 dark:border-white/10 text-xs font-medium bg-white dark:bg-white/5 whitespace-nowrap hover:border-[#007AFF] hover:text-[#007AFF] transition-colors">{r}</button>
                    ))}
                </div>
                <div className="grid grid-cols-2 gap-3 mt-2">
                    <button className="flex items-center justify-center gap-2 py-2.5 rounded-xl border border-gray-200 dark:border-white/10 hover:bg-gray-50 dark:hover:bg-white/5 transition-all text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-[#2C2C2E]">
                        <Icon name="RotateCw" size={16} /> 向右旋转 90°
                    </button>
                    <button className="flex items-center justify-center gap-2 py-2.5 rounded-xl border border-gray-200 dark:border-white/10 hover:bg-gray-50 dark:hover:bg-white/5 transition-all text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-[#2C2C2E]">
                         <span className="scale-x-[-1] inline-block"><Icon name="RotateCw" size={16} /></span> 水平翻转
                    </button>
                </div>
            </div>

            <div className="pt-4 border-t border-gray-100 dark:border-white/5">
                 <label className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-4 flex items-center gap-2">
                    <Icon name="Palette" size={16} className="text-[#007AFF]" />
                    专业调色
                 </label>
                 <div className="space-y-5">
                    <StyledSlider label="曝光度" value={exposure} min={-100} max={100} onChange={setExposure} />
                    <StyledSlider label="对比度" value={contrast} min={-100} max={100} onChange={setContrast} />
                    <StyledSlider label="自然饱和度" value={vibrance} min={-100} max={100} onChange={setVibrance} />
                    <StyledSlider label="饱和度" value={saturation} min={-100} max={100} onChange={setSaturation} />
                    <StyledSlider label="色相偏移" value={hue} min={-180} max={180} onChange={setHue} unit="°" />
                    <StyledSlider label="锐化/模糊" value={sharpness} min={-100} max={100} onChange={setSharpness} />
                 </div>
            </div>
        </div>
    );
});

const FilterSettings = memo(() => {
    const [intensity, setIntensity] = useState(80);
    const [grain, setGrain] = useState(0);
    const [vignette, setVignette] = useState(0);
    const [selected, setSelected] = useState(0);
    const filters = ['原图', '鲜艳', '黑白', '复古', '冷调', '暖阳', '胶片', '赛博', '清新', '日系', 'Lomo', 'HDR', '褪色', '磨砂', '电影', '拍立得'];

    return (
        <div className="flex flex-col h-full">
            <div className="flex-1 space-y-3 min-h-0 flex flex-col">
                <div className="flex items-center justify-between shrink-0">
                    <label className="text-sm font-medium text-gray-700 dark:text-gray-300">滤镜库 (LUTs)</label>
                    <button onClick={() => setSelected(0)} className="text-xs text-[#007AFF] hover:underline font-medium">重置效果</button>
                </div>
                <div className="grid grid-cols-3 gap-2 overflow-y-auto no-scrollbar pr-1 pb-1 content-start">
                    {filters.map((f, i) => (
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

            <div className="shrink-0 pt-4 border-t border-gray-100 dark:border-white/5 mt-4 space-y-4">
                 <StyledSlider label="滤镜强度" value={intensity} onChange={setIntensity} unit="%" />
                 <StyledSlider label="颗粒感 (Grain)" value={grain} onChange={setGrain} />
                 <StyledSlider label="暗角 (Vignette)" value={vignette} onChange={setVignette} />
            </div>
        </div>
    );
});

type PdfSettingsProps = {
    size: string;
    setSize: (v: string) => void;
    layout: string;
    setLayout: (v: string) => void;
    fit: string;
    setFit: (v: string) => void;
    compression: string;
    setCompression: (v: string) => void;
    title: string;
    setTitle: (v: string) => void;
    author: string;
    setAuthor: (v: string) => void;
};

const PdfSettings = memo(({
    size,
    setSize,
    layout,
    setLayout,
    fit,
    setFit,
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
                options={['A4 (210 x 297 mm)', 'A3 (297 x 420 mm)', 'Letter', 'Legal', '原图尺寸混合']} 
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

const GifSettings = memo(() => {
     const [mode, setMode] = useState('导出序列帧');
     const [fps, setFps] = useState(12);
     const [loop, setLoop] = useState('无限循环');

     return (
         <div className="space-y-6">
            <div className="space-y-2">
                <label className="text-sm font-medium text-gray-700 dark:text-gray-300">处理模式</label>
                <SegmentedControl 
                    options={['导出序列帧', '倒放 GIF', '修改帧率', '视频转 GIF']}
                    value={mode}
                    onChange={setMode}
                />
            </div>

            {mode === '导出序列帧' && (
                <div className="space-y-4 animate-enter">
                    <CustomSelect label="导出格式" options={['PNG (无损)', 'JPG (小体积)', 'WEBP']} value="PNG (无损)" onChange={()=>{}} />
                    <Switch label="打包为 ZIP" checked={true} onChange={()=>{}} />
                </div>
            )}

            {mode === '修改帧率' && (
                <div className="animate-enter space-y-4">
                     <StyledSlider label="目标帧率 (FPS)" value={fps} min={1} max={60} onChange={setFps} />
                     <CustomSelect label="循环模式" options={['无限循环', '播放一次', '播放三次']} value={loop} onChange={setLoop} />
                </div>
            )}
            
            <div className="p-3 rounded-xl bg-purple-50 dark:bg-purple-500/10 text-xs text-purple-700 dark:text-purple-400 mt-2 border border-purple-100 dark:border-purple-500/10">
                高性能 GIF 引擎，支持解析全局调色板、透明通道及帧延迟信息。
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
                label: key,
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
        rows = [{ label: '错误', value: String(info.error), editKey: '', editable: false }];
    }

    const name = filePath ? filePath.replace(/\\/g, '/').split('/').pop() || filePath : '';
    const isEmpty = rows.length === 0;

    const startEdit = (label: string, value: string) => {
        setEditingKey(label);
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
                                const isEditing = editingKey === item.label;
                                const isEditable = Boolean(item.editable) && !(item.value || '').startsWith('hex:');
                                return (
                                    <tr key={`${item.label}-${i}`} className="border-b border-gray-100 dark:border-white/5 last:border-0 hover:bg-black/5 dark:hover:bg-white/5 transition-colors">
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
                                                    onClick={() => isEditable && startEdit(item.label, item.value)}
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
    const [pdfCompression, setPdfCompression] = useState('不压缩');
    const [pdfTitle, setPdfTitle] = useState('');
    const [pdfAuthor, setPdfAuthor] = useState('');
    const [infoFilePath, setInfoFilePath] = useState('');
    const [infoPreview, setInfoPreview] = useState<any | null>(null);

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
            case 'adjust': return <AdjustSettings />;
            case 'filter': return <FilterSettings />;
            case 'pdf':
                return (
                    <PdfSettings
                        size={pdfSize}
                        setSize={setPdfSize}
                        layout={pdfLayout}
                        setLayout={setPdfLayout}
                        fit={pdfFit}
                        setFit={setPdfFit}
                        compression={pdfCompression}
                        setCompression={setPdfCompression}
                        title={pdfTitle}
                        setTitle={setPdfTitle}
                        author={pdfAuthor}
                        setAuthor={setPdfAuthor}
                    />
                );
            case 'gif': return <GifSettings />;
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
    const addPrefixToRelPath = (rel: string, prefix: string) => {
        const p = sanitizeFilePrefix(prefix);
        if (!p) return rel;
        const normalized = rel.replace(/\\/g, '/');
        const idx = normalized.lastIndexOf('/');
        const dir = idx >= 0 ? normalized.slice(0, idx + 1) : '';
        const name = idx >= 0 ? normalized.slice(idx + 1) : normalized;
        return `${dir}${p}_${name}`;
    };

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
                    'A4 (210 x 297 mm)': 'A4',
                    'A3 (297 x 420 mm)': 'A3',
                    'Letter': 'Letter',
                    'Legal': 'Legal',
                };
                const compressionMap: Record<string, number> = {
                    '不压缩': 0,
                    '轻度': 1,
                    '标准': 2,
                    '强力': 3,
                };
                const req = {
                    image_paths: files.map(f => normalizePath(f.input_path)),
                    output_path: joinPath(outDir, 'output.pdf'),
                    page_size: sizeMap[pdfSize] ?? 'A4',
                    layout: pdfLayout === '横向' ? 'landscape' : 'portrait',
                    margin: 72,
                    compression_level: compressionMap[pdfCompression] ?? 0,
                    title: pdfTitle.trim(),
                    author: pdfAuthor.trim(),
                };
                await window.go.main.App.GeneratePDF(req);
                setProgress(100);
                setLastMessage(`PDF 已生成：${req.output_path}`);
                return;
            }

            if (id === 'gif') {
                for (const f of files) {
                    const name = basename(f.input_path).replace(/\.[^.]+$/, '');
                    const req = {
                        input_path: normalizePath(f.input_path),
                        output_dir: joinPath(outDir, `${name}_frames`),
                        start_frame: 0,
                        end_frame: 0,
                        format: 'png',
                    };
                    try {
                        await window.go.main.App.SplitGIF(req);
                    } catch (err) {
                        console.error(`Failed to split gif ${f.input_path}:`, err);
                    }
                    completed++;
                    setProgress((completed / total) * 100);
                }
                setLastMessage(`GIF 拆分完成：${completed}/${total} 项`);
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

    return (
        <div className="h-full flex flex-col p-1">
            <div className={`flex-1 grid grid-cols-1 ${isInfo ? 'lg:grid-cols-6' : 'lg:grid-cols-3'} gap-6 min-h-0`}>
                {/* Left Side: Upload Area (Shared) - Replaced with FileDropZone */}
                <div className={`h-full ${isInfo ? 'lg:col-span-2' : 'lg:col-span-2'}`}>
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
                            if (id !== 'info') return;
                            if (file?.input_path) {
                                loadInfoForPath(file.input_path);
                            }
                        }}
                        selectedPath={id === 'info' ? infoFilePath : undefined}
                        acceptedFormats="image/*,.svg"
                        allowMultiple={true}
                        title="拖拽文件 / 文件夹到这里"
                        subTitle=""
                    />
                </div>

                {/* Right Side: Specific Settings Panel (Secondary Menu) */}
                <div className={`bg-white dark:bg-[#2C2C2E] rounded-3xl p-5 shadow-sm border border-gray-100 dark:border-white/5 flex flex-col h-full overflow-hidden ${isInfo ? 'lg:col-span-4' : ''}`}>
                    {!isInfo && (
                        <div className="space-y-3 pb-4 border-b border-gray-100 dark:border-white/5 mb-4 shrink-0">
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
                                <div className="text-xs text-gray-500 dark:text-gray-400 font-mono break-all bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 rounded-xl px-3 py-2">
                                    {effectiveOutputDir}
                                </div>
                            )}
                            {lastMessage && (
                                <div className="text-xs text-gray-600 dark:text-gray-300 bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 rounded-xl px-3 py-2">
                                    {lastMessage}
                                </div>
                            )}
                        </div>
                    )}
                    {isInfo && lastMessage && (
                        <div className="mb-4 text-xs text-gray-600 dark:text-gray-300 bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 rounded-xl px-3 py-2">
                            {lastMessage}
                        </div>
                    )}
                    
                    <div className="flex-1 overflow-y-auto no-scrollbar px-1 pb-2">
                        {renderSettings()}
                    </div>

                    {!isInfo && (
                        <div className="pt-4 border-t border-gray-100 dark:border-white/5 mt-auto shrink-0 space-y-3">
                            {(isProcessing || progress > 0) && (
                                <ProgressBar progress={progress} label={isProcessing ? "正在处理..." : "已完成"} />
                            )}
                            <button 
                                onClick={handleStartProcessing} 
                                disabled={isProcessing}
                                className={`w-full py-3.5 rounded-xl font-semibold shadow-lg shadow-blue-500/25 transition-all active:scale-[0.98] flex items-center justify-center gap-2 text-white ${isProcessing ? 'bg-gray-400 cursor-not-allowed' : 'bg-gradient-to-r from-[#007AFF] to-[#0055FF] hover:to-[#0044DD]'}`}
                            >
                                <Icon name="Wand2" size={18} /> {isProcessing ? '处理中...' : '开始处理'}
                            </button>
                        </div>
                    )}
                </div>
            </div>
        </div>
    );
};

export default DetailView;
