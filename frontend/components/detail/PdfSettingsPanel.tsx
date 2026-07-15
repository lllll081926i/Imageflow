import React, { memo } from 'react';
import { CustomSelect, SegmentedControl } from '../Controls';

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

export const PdfSettings = memo(({
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
