import React, { memo, useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import Icon from '../Icon';
import { Switch, StyledSlider, CustomSelect, SegmentedControl, PositionGrid } from '../Controls';

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

export const WatermarkSettings = memo(({
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
                        Pattern: "*.jpg;*.jpeg;*.png;*.webp;*.gif;*.bmp;*.tiff;*.tif"
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
                         <button
                             type="button"
                             onClick={handleSelectImage}
                             className="w-full h-28 rounded-xl bg-gray-50 dark:bg-white/5 border-2 border-dashed border-gray-200 dark:border-white/10 flex flex-col items-center justify-center text-sm text-gray-500 cursor-pointer hover:bg-gray-100 dark:hover:bg-white/10 transition-colors animate-enter"
                         >
                             <Icon name="Upload" size={20} className="mb-2 opacity-50" />
                             <span className="text-xs text-gray-500">{imageLabel}</span>
                        </button>
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
