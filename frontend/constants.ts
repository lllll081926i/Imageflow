import { Feature } from './types';

export const FEATURES: Feature[] = [
    { id: 'converter', title: '格式转换', desc: '支持 13 种格式互转', iconName: 'FileType', color: 'text-blue-500', bg: 'bg-blue-50', darkBg: 'dark:bg-blue-500/10' },
    { id: 'compressor', title: '图片压缩', desc: '智能压缩，极小体积', iconName: 'Layers', color: 'text-green-500', bg: 'bg-green-50', darkBg: 'dark:bg-green-500/10' },
    { id: 'resize', title: '尺寸调整', desc: '比例 / 长边 / 短边', iconName: 'Expand', color: 'text-cyan-500', bg: 'bg-cyan-50', darkBg: 'dark:bg-cyan-500/10' },
    { id: 'grid', title: '宫格切图', desc: '四 / 九 / 十六宫格', iconName: 'Grid', color: 'text-purple-500', bg: 'bg-purple-50', darkBg: 'dark:bg-purple-500/10' },
    { id: 'split', title: '图片布局', desc: '按行列自定义分割', iconName: 'SplitGrid', color: 'text-violet-500', bg: 'bg-violet-50', darkBg: 'dark:bg-violet-500/10' },
    { id: 'stitch', title: '长图拼接', desc: '多图合一 / 加边框', iconName: 'Stitch', color: 'text-teal-500', bg: 'bg-teal-50', darkBg: 'dark:bg-teal-500/10' },
    { id: 'watermark', title: '批量水印', desc: '丰富样式，实时预览', iconName: 'Stamp', color: 'text-pink-500', bg: 'bg-pink-50', darkBg: 'dark:bg-pink-500/10' },
    { id: 'palette', title: '色彩提取', desc: '主题色与十色色板', iconName: 'Palette', color: 'text-orange-500', bg: 'bg-orange-50', darkBg: 'dark:bg-orange-500/10' },
    { id: 'pdf', title: '转 PDF', desc: '批量合并，自由排版', iconName: 'FileJson', color: 'text-amber-600', bg: 'bg-amber-50', darkBg: 'dark:bg-amber-500/10' },
    { id: 'gif', title: 'GIF 工具', desc: '导出、倒放、调帧率', iconName: 'Scissors', color: 'text-red-500', bg: 'bg-red-50', darkBg: 'dark:bg-red-500/10' },
    { id: 'richtext', title: '富文本制图', desc: '文字转图片，可存模板', iconName: 'Type', color: 'text-indigo-500', bg: 'bg-indigo-50', darkBg: 'dark:bg-indigo-500/10' },
];
