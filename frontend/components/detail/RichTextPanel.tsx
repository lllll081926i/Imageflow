import React, { memo, useCallback, useMemo } from 'react';
import { CustomSelect, StyledSlider, Switch } from '../Controls';
import { ColorField } from '../ColorField';
import { PresetBar } from './PresetBar';
import type { RichTextBlock } from '../hooks/useFeatureParams';

type RichTextPanelProps = {
    blocks: RichTextBlock[];
    setBlocks: (blocks: RichTextBlock[]) => void;
    width: number; setWidth: (v: number) => void;
    padding: number; setPadding: (v: number) => void;
    background: string; setBackground: (v: string) => void;
    textColor: string; setTextColor: (v: string) => void;
    headingColor: string; setHeadingColor: (v: string) => void;
    accentColor: string; setAccentColor: (v: string) => void;
    fontSize: number; setFontSize: (v: number) => void;
    lineHeight: number; setLineHeight: (v: number) => void;
    fontName: string; setFontName: (v: string) => void;
    fileName: string; setFileName: (v: string) => void;
};

const TYPE_LABELS: Record<RichTextBlock['type'], string> = {
    heading: '标题',
    paragraph: '正文',
    list: '列表',
    quote: '引用',
    image: '图片',
};

const FONT_OPTIONS = ['微软雅黑', '黑体', '宋体', '等线', 'Arial'];

/** Built-in style themes applied on top of current blocks. */
export const RICH_THEMES: Array<{
    name: string;
    style: Pick<RichTextPanelProps, 'background' | 'textColor' | 'headingColor' | 'accentColor'> &
        { fontSize?: number; lineHeight?: number; padding?: number };
}> = [
    { name: '简约白', style: { background: '#FFFFFF', textColor: '#1F2937', headingColor: '#111827', accentColor: '#007AFF', padding: 48 } },
    { name: '暗夜', style: { background: '#1B1B1F', textColor: '#D1D5DB', headingColor: '#F9FAFB', accentColor: '#64AFFF', padding: 48 } },
    { name: '米黄手账', style: { background: '#FDF6E3', textColor: '#4B4237', headingColor: '#8B5E34', accentColor: '#D97706', lineHeight: 1.8, padding: 56 } },
    { name: '蓝调商务', style: { background: '#0F2540', textColor: '#CBD5E1', headingColor: '#FFFFFF', accentColor: '#38BDF8', padding: 52 } },
    { name: '樱花粉', style: { background: '#FFF1F2', textColor: '#6B3A45', headingColor: '#BE123C', accentColor: '#FB7185', lineHeight: 1.75 } },
    { name: '墨绿禅意', style: { background: '#0C1F1A', textColor: '#B7CFC5', headingColor: '#ECFDF5', accentColor: '#34D399', lineHeight: 1.7 } },
];

type ThemeChipProps = { onApply: typeof applyThemeDefault };
function applyThemeDefault(_t: unknown) { /* replaced below */ }
void applyThemeDefault;

export const RichTextSettings = memo(({
    blocks, setBlocks,
    width, setWidth,
    padding, setPadding,
    background, setBackground,
    textColor, setTextColor,
    headingColor, setHeadingColor,
    accentColor, setAccentColor,
    fontSize, setFontSize,
    lineHeight, setLineHeight,
    fontName, setFontName,
    fileName, setFileName,
}: RichTextPanelProps) => {
    const payload = useMemo(() => ({
        blocks, width, padding, background, textColor, headingColor, accentColor,
        fontSize, lineHeight, fontName,
    }), [blocks, width, padding, background, textColor, headingColor, accentColor, fontSize, lineHeight, fontName]);

    const apply = useCallback((data: Record<string, unknown>) => {
        if (Array.isArray(data.blocks)) setBlocks(data.blocks as RichTextBlock[]);
        if (typeof data.width === 'number') setWidth(data.width);
        if (typeof data.padding === 'number') setPadding(data.padding);
        if (typeof data.background === 'string') setBackground(data.background);
        if (typeof data.textColor === 'string') setTextColor(data.textColor);
        if (typeof data.headingColor === 'string') setHeadingColor(data.headingColor);
        if (typeof data.accentColor === 'string') setAccentColor(data.accentColor);
        if (typeof data.fontSize === 'number') setFontSize(data.fontSize);
        if (typeof data.lineHeight === 'number') setLineHeight(data.lineHeight);
        if (typeof data.fontName === 'string') setFontName(data.fontName);
    }, [setBlocks, setWidth, setPadding, setBackground, setTextColor, setHeadingColor, setAccentColor, setFontSize, setLineHeight, setFontName]);

    const updateBlock = (index: number, patch: Partial<RichTextBlock>) => {
        setBlocks(blocks.map((b, i) => (i === index ? { ...b, ...patch } : b)));
    };

    const pickImage = async (done: (path: string | null) => void) => {
        try {
            const res = await window.runtime?.OpenFileDialog?.({
                filters: [{ DisplayName: 'Images', Pattern: '*.png;*.jpg;*.jpeg;*.webp;*.bmp' }],
                allowsMultipleSelection: false,
            });
            const path = Array.isArray(res) ? res[0] : (typeof res === 'string' ? res : null);
            done(path || null);
        } catch {
            done(null);
        }
    };

    return (
        <div className="space-y-6">
            <PresetBar scope="richtext" buildPayload={() => payload} applyPayload={apply} />

            <div className="space-y-2">
                <label className="text-sm font-medium text-gray-700 dark:text-gray-300">预设样式</label>
                <div className="flex flex-wrap gap-2">
                    {RICH_THEMES.map((theme) => (
                        <button
                            key={theme.name}
                            type="button"
                            onClick={() => {
                                const s = theme.style;
                                if (s.background) setBackground(s.background as string);
                                if (s.textColor) setTextColor(s.textColor as string);
                                if (s.headingColor) setHeadingColor(s.headingColor as string);
                                if (s.accentColor) setAccentColor(s.accentColor as string);
                                if (typeof s.fontSize === 'number') setFontSize(s.fontSize);
                                if (typeof s.lineHeight === 'number') setLineHeight(s.lineHeight);
                                if (typeof s.padding === 'number') setPadding(s.padding);
                            }}
                            className="spring-tap-sm px-2.5 py-1.5 rounded-xl border text-xs font-medium flex items-center gap-1.5 transition-all duration-200 border-gray-200 dark:border-white/10 text-gray-600 dark:text-gray-300 hover:border-[#007AFF]/50"
                        >
                            <span className="w-3.5 h-3.5 rounded-full border border-black/10 dark:border-white/20 shrink-0" style={{ background: theme.style.background }} />
                            {theme.name}
                        </button>
                    ))}
                </div>
            </div>

            <div className="space-y-3">
                <label className="text-sm font-medium text-gray-700 dark:text-gray-300">内容块</label>
                {blocks.map((block, index) => (
                    <div key={index} className="rounded-2xl border border-gray-200 dark:border-white/10 bg-gray-50 dark:bg-white/5 p-3 space-y-2">
                        <div className="flex items-center gap-1.5">
                            <span className="text-[11px] px-1.5 py-0.5 rounded-md bg-[#007AFF]/10 text-[#007AFF] font-medium shrink-0">
                                {TYPE_LABELS[block.type]}
                            </span>
                            {block.type === 'heading' && (
                                <div className="flex rounded-lg overflow-hidden border border-gray-200 dark:border-white/10">
                                    {[1, 2, 3].map((l) => (
                                        <button
                                            key={l}
                                            type="button"
                                            onClick={() => updateBlock(index, { level: l })}
                                            className={`spring-tap-sm w-7 h-6 text-[11px] font-mono transition-colors ${((block.level ?? 1) === l) ? 'bg-[#007AFF] text-white' : 'bg-white dark:bg-white/10 text-gray-500 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-white/20'}`}
                                        >
                                            H{l}
                                        </button>
                                    ))}
                                </div>
                            )}
                            <div className="flex-1" />
                            <button type="button" onClick={() => moveBlockLocal(blocks, setBlocks, index, -1)} disabled={index === 0}
                                className="spring-tap-sm w-6 h-6 rounded-md text-xs text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 disabled:opacity-30">↑</button>
                            <button type="button" onClick={() => moveBlockLocal(blocks, setBlocks, index, 1)} disabled={index === blocks.length - 1}
                                className="spring-tap-sm w-6 h-6 rounded-md text-xs text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 disabled:opacity-30">↓</button>
                            <button type="button" onClick={() => setBlocks(blocks.filter((_, i) => i !== index))}
                                className="spring-tap-sm w-6 h-6 rounded-md text-xs text-red-400 hover:text-red-600">✕</button>
                        </div>

                        {(block.type === 'heading' || block.type === 'paragraph' || block.type === 'quote') && (
                            <textarea
                                value={block.text ?? ''}
                                rows={block.type === 'heading' ? 1 : 3}
                                onChange={(e) => updateBlock(index, { text: e.target.value })}
                                placeholder="输入内容"
                                className="w-full px-2.5 py-2 rounded-xl bg-white dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm outline-none dark:text-white focus:border-[#007AFF] transition-colors resize-none"
                            />
                        )}

                        {block.type === 'image' && (
                            <div className="space-y-2">
                                <div className="text-[11px] text-gray-500 dark:text-gray-400 break-all font-mono truncate">
                                    {block.path || '未选择图片'}
                                </div>
                                <StyledSlider
                                    label="宽度占比"
                                    value={block.width_percent ?? 100}
                                    min={10}
                                    max={100}
                                    unit="%"
                                    onChange={(v) => updateBlock(index, { width_percent: v })}
                                />
                            </div>
                        )}

                        {block.type === 'list' && (
                            <div className="space-y-2">
                                <Switch
                                    checked={Boolean(block.ordered)}
                                    onChange={(checked) => updateBlock(index, { ordered: checked })}
                                    label="有序列表"
                                />
                                <textarea
                                    value={(block.items ?? []).join('\n')}
                                    rows={Math.max(2, (block.items ?? []).length)}
                                    onChange={(e) => updateBlock(index, { items: e.target.value.split('\n') })}
                                    placeholder={'每行一项'}
                                    className="w-full px-2.5 py-2 rounded-xl bg-white dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm outline-none dark:text-white focus:border-[#007AFF] transition-colors resize-none"
                                />
                            </div>
                        )}
                    </div>
                ))}

                <CustomSelect
                    options={['＋ 标题', '＋ 正文', '＋ 列表', '＋ 引用', '＋ 图片']}
                    value=""
                    onChange={(label) => {
                        if (label.includes('标题')) setBlocks([...blocks, { type: 'heading', text: '新标题', level: 2 }]);
                        else if (label.includes('正文')) setBlocks([...blocks, { type: 'paragraph', text: '' }]);
                        else if (label.includes('列表')) setBlocks([...blocks, { type: 'list', ordered: false, items: ['第一项'] }]);
                        else if (label.includes('引用')) setBlocks([...blocks, { type: 'quote', text: '' }]);
                        else if (label.includes('图片')) void pickImage((path) => {
                            if (path) setBlocks([...blocks, { type: 'image', path, width_percent: 100 }]);
                        });
                    }}
                />
            </div>

            <div className="rounded-2xl bg-gray-50 dark:bg-white/5 border border-gray-200/80 dark:border-white/10 p-4 space-y-4">
                <label className="text-sm font-medium text-gray-700 dark:text-gray-300">整体样式</label>
                <StyledSlider label="画布宽度" value={width} min={300} max={2000} step={20} unit="px" onChange={setWidth} />
                <StyledSlider label="内边距" value={padding} min={16} max={200} unit="px" onChange={setPadding} />
                <StyledSlider label="基础字号" value={fontSize} min={12} max={60} unit="px" onChange={setFontSize} />
                <StyledSlider label="行高" value={lineHeight} min={1.2} max={2.5} step={0.1} onChange={setLineHeight} />
                <div className="grid grid-cols-2 gap-3">
                    <ColorField label="背景" value={background} onChange={setBackground} />
                    <ColorField label="强调色" value={accentColor} onChange={setAccentColor} />
                    <ColorField label="正文色" value={textColor} onChange={setTextColor} />
                    <ColorField label="标题色" value={headingColor} onChange={setHeadingColor} />
                </div>
                <CustomSelect
                    label="字体"
                    options={FONT_OPTIONS}
                    value={fontName}
                    onChange={setFontName}
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
                    <span className="text-xs text-gray-400 shrink-0">.png</span>
                </div>
            </div>
        </div>
    );
});

function moveBlockLocal<T>(arr: T[], setArr: (next: T[]) => void, index: number, delta: -1 | 1) {
    const next = [...arr];
    const target = index + delta;
    if (target < 0 || target >= next.length) return;
    const tmp = next[index];
    next[index] = next[target];
    next[target] = tmp;
    setArr(next);
}
