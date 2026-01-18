import React, { useMemo, useState, memo } from 'react';
import Icon from './Icon';
import { FEATURES } from '../constants';
import { ViewState } from '../types';
import { Switch, StyledSlider, CustomSelect, SegmentedControl, PositionGrid, FileDropZone, ProgressBar } from './Controls';

interface DetailViewProps {
    id: ViewState;
    onBack: () => void;
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

const ConverterSettings = memo(() => {
    const [format, setFormat] = useState('JPG');
    const [quality, setQuality] = useState(90);
    const [resizeMode, setResizeMode] = useState('原图尺寸');
    const [keepMetadata, setKeepMetadata] = useState(true);
    const [colorSpace, setColorSpace] = useState('保持原样');
    const [dpi, setDpi] = useState(72);

    return (
        <div className="space-y-4">
            {/* Top Row: Format & Quality */}
            <div className="grid grid-cols-[1fr_2fr] gap-4">
                <CustomSelect 
                    label="目标格式" 
                    options={['JPG', 'PNG', 'WEBP', 'AVIF', 'TIFF', 'ICO', 'BMP']} 
                    value={format}
                    onChange={setFormat}
                />
                <div className="space-y-2">
                    <label className="text-sm font-medium text-gray-700 dark:text-gray-300">输出质量</label>
                    <div className="h-10 flex items-center">
                        <StyledSlider value={quality} onChange={setQuality} unit="%" className="w-full" />
                    </div>
                </div>
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
                 <StyledSlider label="缩放比例" value={75} min={10} max={200} unit="%" onChange={() => {}} />
            )}

            {resizeMode === '固定宽高' && (
                <div className="flex gap-3 animate-enter">
                    <div className="flex-1 space-y-1">
                        <label className="text-xs text-gray-500">宽度 (px)</label>
                        <input type="number" className="w-full px-3 py-2 rounded-lg bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm focus:border-[#007AFF] focus:ring-1 focus:ring-[#007AFF] outline-none dark:text-white" placeholder="1920" />
                    </div>
                    <div className="flex-1 space-y-1">
                        <label className="text-xs text-gray-500">高度 (px)</label>
                        <input type="number" className="w-full px-3 py-2 rounded-lg bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm focus:border-[#007AFF] focus:ring-1 focus:ring-[#007AFF] outline-none dark:text-white" placeholder="1080" />
                    </div>
                </div>
            )}

            <div className="pt-2 border-t border-gray-100 dark:border-white/5 space-y-3">
                <div className="grid grid-cols-2 gap-3">
                     <CustomSelect label="色彩空间" options={['保持原样', 'sRGB', 'P3', 'CMYK']} value={colorSpace} onChange={setColorSpace} />
                     <div className="space-y-2">
                        <label className="text-sm font-medium text-gray-700 dark:text-gray-300">DPI</label>
                        <input type="number" value={dpi} onChange={e => setDpi(Number(e.target.value))} className="w-full px-3 py-2.5 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm focus:border-[#007AFF] focus:ring-1 focus:ring-[#007AFF] outline-none dark:text-white" />
                     </div>
                </div>
                <Switch label="保留元数据 (EXIF)" checked={keepMetadata} onChange={setKeepMetadata} />
            </div>
        </div>
    );
});

const CompressorSettings = memo(() => {
    const [mode, setMode] = useState('标准');
    const [targetSize, setTargetSize] = useState(false);
    const [stripMeta, setStripMeta] = useState(true);
    const [engine, setEngine] = useState('MozJPEG');
    
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

            <CustomSelect label="压缩引擎" options={['MozJPEG (推荐)', 'Guetzli (慢但小)', 'PNGQuant (针对PNG)', 'OxiPNG']} value={engine} onChange={setEngine} />

            <div className="space-y-4 pt-2">
                <Switch label="指定目标大小限制 (KB)" checked={targetSize} onChange={setTargetSize} />
                {targetSize && (
                     <div className="animate-enter">
                        <input type="number" placeholder="例如: 500" className="w-full px-4 py-2.5 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm focus:outline-none focus:ring-2 focus:ring-[#007AFF]/50 focus:border-[#007AFF] dark:text-white transition-all" />
                     </div>
                )}
            </div>

             <div className="pt-2 border-t border-gray-100 dark:border-white/5 space-y-4">
                 <Switch label="移除所有元数据 (隐私保护)" checked={stripMeta} onChange={setStripMeta} />
                 <div className="p-3 rounded-xl bg-green-50 dark:bg-green-500/10 text-xs text-green-700 dark:text-green-400 leading-relaxed border border-green-100 dark:border-green-500/10">
                    {mode === '无损' ? '仅优化文件结构，不损失任何画质。' : `当前使用 ${engine} 智能量化算法，预计减少 40%-80% 体积。`}
                </div>
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

const PdfSettings = memo(() => {
    const [size, setSize] = useState('A4 (210 x 297 mm)');
    const [layout, setLayout] = useState('纵向');
    const [fit, setFit] = useState('适应页面');
    const [compress, setCompress] = useState(false);
    const [ocr, setOcr] = useState(false);

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
                    <input type="text" placeholder="文档标题" className="w-full px-3 py-2 rounded-lg bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm outline-none dark:text-white" />
                    <input type="text" placeholder="作者" className="w-full px-3 py-2 rounded-lg bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm outline-none dark:text-white" />
                 </div>
                <div className="h-px bg-gray-100 dark:bg-white/5" />
                <Switch label="压缩图片流 (减小体积)" checked={compress} onChange={setCompress} />
                <Switch label="OCR 文字识别 (创建可搜索 PDF)" checked={ocr} onChange={setOcr} />
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

const InfoSettings = memo(() => {
    // Mock Data for UI
    const exifData = [
        { label: '尺寸', value: '4032 x 3024' },
        { label: '文件大小', value: '4.2 MB' },
        { label: '颜色空间', value: 'Display P3' },
        { label: '深度', value: '24 bit' },
        { label: 'DPI', value: '72' },
        { label: '设备制造', value: 'Apple' },
        { label: '设备型号', value: 'iPhone 15 Pro' },
        { label: '光圈', value: 'f/1.78' },
        { label: '曝光时间', value: '1/120 s' },
        { label: 'ISO 感光度', value: '80' },
        { label: '焦距', value: '24mm' },
        { label: 'GPS', value: '34.05, -118.24' },
    ];

    return (
        <div className="h-full flex flex-col">
            <div className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-4 flex items-center justify-between">
                <span>元数据预览</span>
                <span className="text-xs text-gray-400 font-normal">sample.jpg</span>
            </div>
            <div className="bg-gray-50 dark:bg-white/5 rounded-xl border border-gray-200 dark:border-white/10 overflow-hidden text-sm flex-1 overflow-y-auto no-scrollbar">
                <table className="w-full">
                    <tbody>
                        {exifData.map((item, i) => (
                            <tr key={i} className="border-b border-gray-100 dark:border-white/5 last:border-0 hover:bg-black/5 dark:hover:bg-white/5 transition-colors">
                                <td className="py-2.5 px-4 text-gray-500 dark:text-gray-400 w-1/3">{item.label}</td>
                                <td className="py-2.5 px-4 text-gray-900 dark:text-white font-mono text-xs">{item.value}</td>
                            </tr>
                        ))}
                    </tbody>
                </table>
            </div>
            <div className="mt-4 flex gap-3 shrink-0">
                 <button className="flex-1 py-2 rounded-lg border border-gray-200 dark:border-white/10 text-sm font-medium text-gray-600 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-white/5 transition-colors">导出 JSON</button>
                 <button className="flex-1 py-2 rounded-lg border border-gray-200 dark:border-white/10 text-sm font-medium text-red-500 hover:bg-red-50 dark:hover:bg-red-500/10 hover:border-red-200 transition-colors">清除隐私信息</button>
            </div>
        </div>
    );
});

const DetailView: React.FC<DetailViewProps> = ({ id, onBack }) => {
    const feature = FEATURES.find(f => f.id === id);
    if (!feature) return null;

    const [dropResult, setDropResult] = useState<ExpandDroppedPathsResult | null>(null);
    const [preserveFolderStructure, setPreserveFolderStructure] = useState(true);
    const [outputDir, setOutputDir] = useState('');
    const [isProcessing, setIsProcessing] = useState(false);
    const [progress, setProgress] = useState(0);
    const [lastMessage, setLastMessage] = useState<string>('');

    const renderSettings = () => {
        switch(id) {
            case 'converter': return <ConverterSettings />;
            case 'compressor': return <CompressorSettings />;
            case 'watermark': return <WatermarkSettings />;
            case 'adjust': return <AdjustSettings />;
            case 'filter': return <FilterSettings />;
            case 'pdf': return <PdfSettings />;
            case 'gif': return <GifSettings />;
            case 'info': return <InfoSettings />;
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
        if (!outDir) {
            setLastMessage('请选择输出目录');
            return;
        }

        setIsProcessing(true);
        try {
            const files = dropResult.files;
            const total = files.length;
            let completed = 0;

            if (id === 'converter') {
                const format = 'jpg'; // In real app, get from settings
                const quality = 90;

                // Process individually to show progress
                for (const f of files) {
                    const rel = preserveFolderStructure && f.is_from_dir_drop ? f.relative_path : basename(f.input_path);
                    const outRel = replaceExt(rel, format);
                    const req = {
                        input_path: f.input_path,
                        output_path: joinPath(outDir, outRel),
                        format,
                        quality,
                        width: 0,
                        height: 0,
                        maintain_ar: true,
                    };
                    
                    try {
                        await window.go.main.App.Convert(req);
                    } catch (err) {
                        console.error(`Failed to convert ${f.input_path}:`, err);
                    }
                    
                    completed++;
                    setProgress((completed / total) * 100);
                }
                setLastMessage(`转换完成：${completed}/${total} 项`);
                return;
            }

            if (id === 'compressor') {
                // Process individually to show progress
                for (const f of files) {
                    const rel = preserveFolderStructure && f.is_from_dir_drop ? f.relative_path : basename(f.input_path);
                    const req = {
                        input_path: f.input_path,
                        output_path: joinPath(outDir, rel),
                        mode: 'smart',
                        quality: 80,
                    };

                    try {
                        await window.go.main.App.Compress(req);
                    } catch (err) {
                        console.error(`Failed to compress ${f.input_path}:`, err);
                    }

                    completed++;
                    setProgress((completed / total) * 100);
                }
                setLastMessage(`压缩完成：${completed}/${total} 项`);
                return;
            }

            setLastMessage('该功能的实际处理链路尚未接入');
        } catch (e: any) {
            console.error(e);
            setLastMessage(typeof e?.message === 'string' ? e.message : '处理失败');
        } finally {
            setIsProcessing(false);
        }
    };

    return (
        <div className="h-full flex flex-col p-1">
            <div className="flex-1 grid grid-cols-1 lg:grid-cols-3 gap-6 min-h-0">
                {/* Left Side: Upload Area (Shared) - Replaced with FileDropZone */}
                <div className="lg:col-span-2 h-full">
                    <FileDropZone 
                        onFilesSelected={handleFilesSelected}
                        onPathsExpanded={(result) => {
                            setDropResult(result);
                            setLastMessage('');
                            setProgress(0);
                        }}
                        acceptedFormats="image/*"
                        allowMultiple={true}
                        title="拖拽文件 / 文件夹到这里"
                        subTitle="或点击选择文件（Wails 下支持拖拽绝对路径）"
                    />
                </div>

                {/* Right Side: Specific Settings Panel (Secondary Menu) */}
                <div className="bg-white dark:bg-[#2C2C2E] rounded-3xl p-5 shadow-sm border border-gray-100 dark:border-white/5 flex flex-col h-full overflow-hidden">
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
                    
                    <div className="flex-1 overflow-y-auto no-scrollbar px-1 pb-2">
                        {renderSettings()}
                    </div>

                    {id !== 'info' && (
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
