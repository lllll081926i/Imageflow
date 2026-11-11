package engine

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/lllll081926i/imageflow-go/internal/codec"
	"github.com/lllll081926i/imageflow-go/internal/output"
	"github.com/lllll081926i/imageflow-go/internal/pngopt"
)

// CompressRequest 定义静态图片压缩；Format 为空或 auto 时保持原格式。
type CompressRequest struct {
	InputPath  string `json:"input_path"`
	OutputPath string `json:"output_path"`
	Format     string `json:"format"`
	Quality    int    `json:"quality"`
	Lossless   bool   `json:"lossless"`
}

// CompressionFormat 返回压缩工具支持的输出格式。
func CompressionFormat(input, format string) string {
	if format == "" || strings.EqualFold(format, "auto") {
		format = strings.TrimPrefix(filepath.Ext(input), ".")
	}
	format = normalizeFormat(format)
	switch format {
	case "jpg", "png", "webp", "avif":
		return format
	}
	return ""
}

// Compress 保持像素尺寸，通过比较实际编码大小避免同格式输出膨胀。
func Compress(req CompressRequest) Result {
	if strings.TrimSpace(req.InputPath) == "" || strings.TrimSpace(req.OutputPath) == "" {
		return Fail("BAD_INPUT", "缺少输入或输出路径")
	}
	source := CompressionFormat(req.InputPath, "auto")
	format := CompressionFormat(req.InputPath, req.Format)
	if source == "" || format == "" {
		return Fail("BAD_INPUT", "图片压缩支持 JPG、PNG、WebP、AVIF；动图请使用 GIF 工具")
	}
	if req.Quality < 0 || req.Quality > 100 {
		return Fail("BAD_INPUT", "质量必须在 1–100 之间，0 表示默认值 80")
	}
	if req.Lossless && format != "png" && format != "webp" {
		return Fail("BAD_INPUT", "无损压缩仅支持 PNG 和 WebP 输出")
	}
	target := ensureExtension(req.OutputPath, format)
	inAbs, err := filepath.Abs(req.InputPath)
	if err != nil {
		return Fail("BAD_INPUT", err.Error())
	}
	outAbs, err := filepath.Abs(target)
	if err != nil {
		return Fail("BAD_INPUT", err.Error())
	}
	inStat, err := os.Stat(req.InputPath)
	if err != nil {
		return Fail("READ_FAILED", err.Error())
	}
	outStat, _ := os.Stat(target)
	if strings.EqualFold(inAbs, outAbs) || (outStat != nil && os.SameFile(inStat, outStat)) {
		return Fail("BAD_INPUT", "压缩输出不能覆盖输入原图，请选择其他文件名或输出目录")
	}
	f, err := os.Open(req.InputPath)
	if err != nil {
		return Fail("READ_FAILED", err.Error())
	}
	if err = checkStill(f, source, inStat.Size()); err != nil {
		f.Close()
		return Fail("BAD_INPUT", err.Error())
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		f.Close()
		return Fail("READ_FAILED", err.Error())
	}
	cfg, _, err := image.DecodeConfig(f)
	f.Close()
	if err != nil {
		return Fail("READ_FAILED", err.Error())
	}
	if err = output.CheckSize(cfg.Width, cfg.Height); err != nil {
		return Fail("OUTPUT_TOO_LARGE", err.Error())
	}
	quality := req.Quality
	if quality == 0 {
		quality = 80
	}
	var data []byte
	width, height := cfg.Width, cfg.Height
	if source == "png" && format == "png" {
		data, err = pngopt.OptimizeFile(inAbs)
	} else {
		opened, openErr := codec.Open(req.InputPath)
		if openErr != nil {
			return Fail("READ_FAILED", openErr.Error())
		}
		width, height = opened.Width, opened.Height
		data, err = codec.CompressBytes(opened.Image, format, quality, req.Lossless)
		if err == nil {
			data, err = codec.PreserveMetadata(data, format, req.InputPath, false)
		}
	}
	if err != nil {
		return Fail("ENCODE_FAILED", err.Error())
	}
	retained := source == format && int64(len(data)) >= inStat.Size()
	err = output.Atomic(target, func(dst *os.File) error {
		if retained {
			src, err := os.Open(req.InputPath)
			if err != nil {
				return err
			}
			defer src.Close()
			_, err = io.Copy(dst, src)
			return err
		}
		_, err := dst.Write(data)
		return err
	})
	if err != nil {
		return Fail("WRITE_FAILED", err.Error())
	}
	r := withSizes(OK(), req.InputPath, target)
	r.OutputPath, r.Width, r.Height = target, width, height
	if retained {
		r.Message = "未获得更小文件，已保留原始数据"
	} else {
		r.Message = fmt.Sprintf("压缩完成：%d → %d 字节", r.InputSize, r.OutputSize)
	}
	return r
}

// checkStill 只扫描容器块头，避免为了识别动图解码全部帧。
func checkStill(f *os.File, format string, size int64) error {
	if format == "jpg" {
		return nil
	}
	if format == "avif" {
		var head [32]byte
		n, err := f.Read(head[:])
		if err != nil && err != io.EOF {
			return err
		}
		if n < 16 || string(head[4:8]) != "ftyp" {
			return fmt.Errorf("无效的 AVIF 文件头")
		}
		length := int64(binary.BigEndian.Uint32(head[:4]))
		if length < 16 || length > size || length > 4096 {
			return fmt.Errorf("无效的 AVIF 类型块")
		}
		if _, err := f.Seek(8, io.SeekStart); err != nil {
			return err
		}
		brands := make([]byte, length-8)
		if _, err := io.ReadFull(f, brands); err != nil {
			return err
		}
		if bytes.Contains(brands, []byte("avis")) {
			return fmt.Errorf("不支持 AVIF 图像序列，请使用静态图片")
		}
		return nil
	}
	start := int64(8)
	if format == "webp" {
		start = 12
	}
	for pos := start; pos+8 <= size; {
		if _, err := f.Seek(pos, io.SeekStart); err != nil {
			return err
		}
		var head [8]byte
		if _, err := io.ReadFull(f, head[:]); err != nil {
			return err
		}
		var length int64
		var kind string
		if format == "png" {
			length = int64(binary.BigEndian.Uint32(head[:4])) + 12
			kind = string(head[4:])
		} else {
			length = int64(binary.LittleEndian.Uint32(head[4:]))
			length += length%2 + 8
			kind = string(head[:4])
		}
		if length < 8 || length > size-pos {
			return fmt.Errorf("无效的图片容器块")
		}
		if kind == "acTL" || kind == "ANIM" || kind == "ANMF" {
			return fmt.Errorf("检测到动图，请使用 GIF 工具压缩以保留全部帧")
		}
		pos += length
	}
	return nil
}
