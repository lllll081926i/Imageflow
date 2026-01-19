import { Feature } from './types';

export const FEATURES: Feature[] = [
    { id: 'converter', title: '格式转换', desc: '支持 13 种格式互转', iconName: 'FileType', color: 'text-blue-500', bg: 'bg-blue-50', darkBg: 'dark:bg-blue-500/10' },
    { id: 'compressor', title: '图片压缩', desc: '智能压缩，极小体积', iconName: 'Layers', color: 'text-green-500', bg: 'bg-green-50', darkBg: 'dark:bg-green-500/10' },
    { id: 'pdf', title: '转 PDF', desc: '批量合并，自由排版', iconName: 'FileJson', color: 'text-orange-500', bg: 'bg-orange-50', darkBg: 'dark:bg-orange-500/10' },
    { id: 'gif', title: 'GIF 拆分', desc: '提取帧，精准控制', iconName: 'Scissors', color: 'text-red-500', bg: 'bg-red-50', darkBg: 'dark:bg-red-500/10' },
    { id: 'info', title: '信息查看', desc: 'EXIF 元数据深度解析', iconName: 'Info', color: 'text-gray-600 dark:text-gray-400', bg: 'bg-gray-50', darkBg: 'dark:bg-gray-500/10' },
    { id: 'watermark', title: '图片水印', desc: '文字/图片，批量添加', iconName: 'Stamp', color: 'text-purple-500', bg: 'bg-purple-50', darkBg: 'dark:bg-purple-500/10' },
    { id: 'adjust', title: '图片调整', desc: '旋转、翻转、色彩', iconName: 'SlidersHorizontal', color: 'text-cyan-500', bg: 'bg-cyan-50', darkBg: 'dark:bg-cyan-500/10' },
    { id: 'filter', title: '图片滤镜', desc: '专业级滤镜效果', iconName: 'Wand2', color: 'text-pink-500', bg: 'bg-pink-50', darkBg: 'dark:bg-pink-500/10' },
];