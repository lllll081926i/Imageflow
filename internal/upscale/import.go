package upscale

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func ImportLocalModel(ctx context.Context, root, parameter string, scale int) (Model, error) {
	if scale < 2 || scale > 4 {
		return Model{}, fmt.Errorf("模型原生倍率必须为 2、3 或 4")
	}
	if strings.TrimSpace(root) == "" || !strings.HasSuffix(parameter, ".param") {
		return Model{}, fmt.Errorf("请选择 NCNN .param 文件")
	}
	base := strings.TrimSuffix(filepath.Base(parameter), ".param")
	if len(base) == 0 || len(base) > 100 || strings.ContainsAny(base, `/\:`) || isLegacyModel(base) || strings.HasPrefix(base, "realesr-animevideov3-") {
		return Model{}, fmt.Errorf("模型名称无效或属于已停用的旧模型")
	}
	weights := strings.TrimSuffix(parameter, ".param") + ".bin"
	for index, path := range []string{parameter, weights} {
		info, err := os.Lstat(path)
		limit := maxModelBundleBytes
		if index == 0 {
			limit = 1 << 20
		}
		if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > limit {
			return Model{}, fmt.Errorf("模型文件缺失、不是普通文件或超限：%s", filepath.Base(path))
		}
	}
	file, err := os.Open(parameter)
	if err != nil {
		return Model{}, err
	}
	var magic, layers, blobs int
	_, scanErr := fmt.Fscan(bufio.NewReader(file), &magic, &layers, &blobs)
	file.Close()
	if scanErr != nil || magic != 7767517 || layers < 1 || layers > 100000 || blobs < 1 || blobs > 100000 {
		return Model{}, fmt.Errorf("不是有效的 NCNN 参数文件")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return Model{}, fmt.Errorf("模型目录不可写：%w", err)
	}
	stage, err := os.MkdirTemp(root, ".imageflow-custom-*")
	if err != nil {
		return Model{}, err
	}
	defer os.RemoveAll(stage)
	model := Model{Name: "custom-" + base, ParamFile: base + ".param", BinFile: base + ".bin", Scale: scale,
		Backend: "upscayl", Purpose: "本地自定义", License: "用户本地导入；来源与使用许可由用户确认，非官方模型"}
	for index, path := range []string{parameter, weights} {
		limit := maxModelBundleBytes
		if index == 0 {
			limit = 1 << 20
		}
		input, err := os.Open(path)
		if err != nil {
			return Model{}, err
		}
		output, err := os.OpenFile(filepath.Join(stage, filepath.Base(path)), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			input.Close()
			return Model{}, err
		}
		count, copyErr := io.Copy(output, io.LimitReader(contextReader{ctx, input}, limit+1))
		input.Close()
		closeErr := output.Close()
		if copyErr != nil {
			return Model{}, copyErr
		}
		if closeErr != nil {
			return Model{}, closeErr
		}
		if count == 0 || count > limit {
			return Model{}, fmt.Errorf("模型文件为空或超限")
		}
	}
	model.SHA256, err = dirHashContext(ctx, stage, model.ParamFile, model.BinFile)
	if err != nil {
		return Model{}, err
	}
	modelInstallMu.Lock()
	defer modelInstallMu.Unlock()
	if err := installPinnedDirectory(ctx, stage, model.Dir(root), []string{model.ParamFile, model.BinFile}); err != nil {
		return Model{}, err
	}
	return model, writeTrustedModels(ctx, root, []Model{model})
}
