// Package pngopt 调用 OxiPNG 执行无损 PNG 优化。
package pngopt

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

// Optimize 使用单线程，避免与批处理并发叠加后占满 CPU。
func Optimize(data []byte) ([]byte, error) {
	result, err := optimize("-", data)
	if err != nil {
		return nil, err
	}
	if len(result) >= len(data) {
		return data, nil
	}
	return result, nil
}

// OptimizeFile 直接优化源 PNG，保留色彩描述和其他元数据。
func OptimizeFile(path string) ([]byte, error) { return optimize(path, nil) }

func optimize(input string, data []byte) ([]byte, error) {
	path, err := executable()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "-o", "2", "--threads", "1", "--timeout", "60", "--stdout", "--", input)
	configure(cmd)
	cmd.Stdin = bytes.NewReader(data)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	result, err := cmd.Output()
	if ctx.Err() != nil {
		return nil, fmt.Errorf("OxiPNG 压缩超时：%w", ctx.Err())
	}
	if err != nil {
		return nil, fmt.Errorf("OxiPNG 压缩失败：%w；%s", err, stderr.String())
	}
	if len(result) < 8 || !bytes.Equal(result[:8], []byte("\x89PNG\r\n\x1a\n")) {
		return nil, fmt.Errorf("OxiPNG 未返回有效的 PNG")
	}
	return result, nil
}
