const GIF_ERROR_MESSAGES: Record<string, string> = {
    GIF_BAD_REQUEST: '请求参数不完整，请检查输入',
    GIF_UNSUPPORTED_ACTION: '不支持的 GIF 操作类型',
    GIF_INPUT_NOT_FOUND: '输入文件不存在',
    GIF_UNSUPPORTED_IMAGE: '输入文件不是可处理的 GIF',
    GIF_EXPORT_UNSUPPORTED_FORMAT: '导出格式不支持',
    GIF_EXPORT_EMPTY_SELECTION: '没有可导出的帧',
    GIF_SPEED_CHANGE_FAILED: 'GIF 帧率修改失败',
    GIF_REVERSE_FAILED: 'GIF 倒放失败',
    GIF_COMPRESS_FAILED: 'GIF 压缩失败',
    GIF_RESIZE_INVALID_SIZE: '缩放尺寸无效，请至少填写宽或高',
    GIF_RESIZE_FAILED: 'GIF 缩放失败',
    GIF_BUILD_NO_INPUT: '没有可用于合成的输入图片',
    GIF_BUILD_FAILED: 'GIF 合成失败',
    ANIMATED_CONVERT_BAD_FORMAT: '互转目标格式不支持',
    ANIMATED_CONVERT_BAD_INPUT: '输入不是可互转的动图或参数无效',
    ANIMATED_CONVERT_FAILED: '动图互转失败',
    GIF_MEMORY_LIMIT: 'GIF 体积过大，超出安全处理上限',
    GIF_INTERNAL_ERROR: 'GIF 处理发生内部错误',
    GIF_INVALID_JSON: '请求数据格式错误',
};

export function resolveGifErrorMessage(errorCode?: string, fallbackError?: string): string {
    const code = (errorCode || '').trim().toUpperCase();
    if (code && GIF_ERROR_MESSAGES[code]) {
        return GIF_ERROR_MESSAGES[code];
    }
    if (typeof fallbackError === 'string' && fallbackError.trim()) {
        return fallbackError.trim();
    }
    return 'GIF 处理失败';
}
