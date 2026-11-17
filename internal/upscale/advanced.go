package upscale

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/lllll081926i/imageflow-go/internal/codec"
	"github.com/lllll081926i/imageflow-go/internal/output"
)

func targetSize(width, height, scale, customWidth int) (int, int, error) {
	if customWidth < 0 || customWidth > 32768 {
		return 0, 0, newError(CodeBadRequest, "自定义宽度必须介于 1 与 32768")
	}
	targetWidth, targetHeight := width*scale, height*scale
	if customWidth > 0 {
		targetWidth = customWidth
		targetHeight = max(1, int(math.Round(float64(height)*float64(customWidth)/float64(width))))
	}
	if err := output.CheckSize(targetWidth, targetHeight); err != nil {
		return 0, 0, newError(CodeBadRequest, "目标输出超限：%v", err)
	}
	return targetWidth, targetHeight, nil
}

func runDouble(ctx context.Context, req Request) (*Report, error) {
	if err := ctx.Err(); err != nil {
		return nil, newError(CodeCancelled, "已取消")
	}
	width, height, _, err := codec.Probe(req.InputPath)
	if err != nil {
		return nil, newError(CodeBadRequest, "输入图像无效：%v", err)
	}
	scale := req.Scale
	if scale == 0 {
		scale = 4
	}
	if scale < 1 || scale > 16 {
		return nil, newError(CodeBadRequest, "倍率必须介于 1 与 16")
	}
	if _, _, err := targetSize(width*scale, height*scale, scale, req.CustomWidth); err != nil {
		return nil, err
	}
	models, err := Models(req.ModelsRoot)
	if err != nil {
		return nil, newError(CodeModelMissing, "模型清单无效：%v", err)
	}
	found := false
	for _, model := range models {
		if model.Name == req.Model {
			found = true
			if err := output.CheckSize(width*scale*model.Scale, height*scale*model.Scale); err != nil {
				return nil, newError(CodeBadRequest, "第二次推理原生输出超限：%v", err)
			}
		}
	}
	if !found {
		return nil, newError(CodeModelMissing, "未登记的模型：%s", req.Model)
	}
	if err := os.MkdirAll(filepath.Dir(req.OutputPath), 0o755); err != nil {
		return nil, err
	}
	stage, err := os.MkdirTemp(filepath.Dir(req.OutputPath), ".imageflow-double-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stage)
	started := time.Now()
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	first := req
	if req.Progress != nil {
		first.Progress = func(percent int, line string) {
			if percent >= 0 {
				percent /= 2
			}
			req.Progress(percent, line)
		}
	}
	first.Double, first.TTA, first.CustomWidth, first.Compression = false, false, 0, 0
	first.OutputPath, first.Format = filepath.Join(stage, "first.png"), "png"
	if _, err := Run(ctx, first); err != nil {
		return nil, err
	}
	second := req
	if req.Progress != nil {
		second.Progress = func(percent int, line string) {
			if percent >= 0 {
				percent = 50 + percent/2
			}
			req.Progress(percent, line)
		}
	}
	second.Double, second.InputPath = false, first.OutputPath
	report, err := Run(ctx, second)
	if err != nil {
		return nil, err
	}
	report.AppliedScale = scale * scale
	report.Elapsed = time.Since(started)
	report.Warnings = append(report.Warnings, fmt.Sprintf("已执行两次推理，倍率 %d× → %d×", scale, scale*scale))
	return report, nil
}
