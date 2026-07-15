import React, { memo, useState } from 'react';
import Icon from '../Icon';

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

export const InfoSettings = memo(({
    filePath,
    info,
    onExportJSON,
    onClearPrivacy,
    onEditMetadata,
}: InfoSettingsProps) => {
    const [editingKey, setEditingKey] = useState<string | null>(null);
    const [editingValue, setEditingValue] = useState('');

    const buildRowsFromFields = (fields?: any[]) => {
        if (!Array.isArray(fields)) return [];
        return fields
            .map((field) => {
                const source = typeof field?.source === 'string' ? field.source : '';
                const rawLabel = typeof field?.label === 'string' ? field.label : '';
                const displayLabel = source === 'container' ? rawLabel : translateMetaLabel(rawLabel);
                return {
                    rawKey: typeof field?.key === 'string' ? field.key : rawLabel,
                    label: displayLabel || rawLabel || String(field?.key || ''),
                    value: String(field?.value ?? ''),
                    editKey: source === 'piexif' ? rawLabel : '',
                    editable: Boolean(field?.editable) && source === 'piexif' && !rawLabel.toLowerCase().includes('thumbnail'),
                };
            })
            .filter((row) => row.value !== '' && row.value !== 'undefined' && row.value !== 'null');
    };

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
    const structuredRows = buildRowsFromFields(info?.fields);
    const flatMeta = structuredRows.length === 0 ? buildFlatMetadata() : null;
    let rows = structuredRows.length > 0 ? structuredRows : buildRowsFromMap(flatMeta || {}, editableKeys);
    if (!info?.success && info?.error) {
        rows = [{ rawKey: '错误', label: '错误', value: String(info.error), editKey: '', editable: false }];
    }
    const warnings = Array.isArray(info?.warnings)
        ? info.warnings.filter((item) => item && (item.message || item.code))
        : [];

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
            {warnings.length > 0 && (
                <div className="mb-4 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-xs text-amber-800 dark:border-amber-500/20 dark:bg-amber-500/10 dark:text-amber-200">
                    {warnings.map((item: any, index: number) => {
                        const code = typeof item?.code === 'string' ? item.code.trim() : '';
                        const message = typeof item?.message === 'string' ? item.message.trim() : '';
                        const text = [code, message].filter(Boolean).join('：');
                        return (
                            <div key={`${code || 'warning'}-${index}`} className={index > 0 ? 'mt-1' : ''}>
                                {text || '读取过程中出现告警'}
                            </div>
                        );
                    })}
                </div>
            )}
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
