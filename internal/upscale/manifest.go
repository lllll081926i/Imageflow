package upscale

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/lllll081926i/imageflow-go/internal/output"
)

var modelInstallMu sync.Mutex

func (model Model) engineName() string {
	name := strings.TrimSuffix(model.ParamFile, ".param")
	if strings.HasPrefix(name, "realesr-animevideov3-x") {
		return "realesr-animevideov3"
	}
	return name
}

func readManifest(path string) ([]Model, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() > 1<<20 {
		return nil, fmt.Errorf("模型清单不能超过 1 MiB")
	}
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var models []Model
	if err := decoder.Decode(&models); err != nil {
		return nil, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("模型清单末尾包含额外数据")
	}
	if len(models) == 0 || len(models) > 128 {
		return nil, fmt.Errorf("模型数量必须介于 1 与 128")
	}
	seen := map[string]bool{}
	for _, model := range models {
		if model.Backend != "" && model.Backend != "upscayl" {
			return nil, fmt.Errorf("未知模型后端：%s", model.Backend)
		}
		for _, name := range []string{model.Name, model.ParamFile, model.BinFile} {
			if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\:`) || filepath.Base(name) != name {
				return nil, fmt.Errorf("模型名称和文件名不能包含路径：%q", name)
			}
		}
		hash, err := hex.DecodeString(model.SHA256)
		if err != nil || len(hash) != 32 || model.Scale < 2 || model.Scale > 4 || model.License == "" || model.Purpose == "" {
			return nil, fmt.Errorf("模型 %s 缺少有效哈希、倍率、用途或许可证", model.Name)
		}
		key := strings.ToLower(model.Name)
		if seen[key] {
			return nil, fmt.Errorf("模型名称重复：%s", model.Name)
		}
		seen[key] = true
		if !strings.HasSuffix(model.ParamFile, ".param") || model.BinFile != strings.TrimSuffix(model.ParamFile, ".param")+".bin" {
			return nil, fmt.Errorf("模型参数与权重文件名不配对：%s", model.Name)
		}
		if model.engineName() == "realesr-animevideov3" && model.ParamFile != fmt.Sprintf("realesr-animevideov3-x%d.param", model.Scale) {
			return nil, fmt.Errorf("动漫视频模型文件名与倍率不匹配：%s", model.Name)
		}
	}
	return models, nil
}

func Models(root string) ([]Model, error) {
	models := RegistrySnapshot()
	if strings.TrimSpace(root) == "" {
		return models, nil
	}
	trusted, err := readManifest(filepath.Join(root, "imageflow-models.json"))
	if os.IsNotExist(err) {
		return models, nil
	}
	if err != nil {
		return models, err
	}
	for _, model := range trusted {
		if isLegacyModel(model.Name) {
			continue
		}
		replaced := false
		for index := range models {
			if strings.EqualFold(models[index].Name, model.Name) {
				models[index], replaced = model, true
				break
			}
		}
		if !replaced {
			models = append(models, model)
		}
	}
	return models, nil
}

func ImportManifest(source, root string) error {
	return ImportManifestContext(context.Background(), source, root)
}

func ImportManifestContext(ctx context.Context, source, root string) error {
	if strings.TrimSpace(root) == "" {
		return fmt.Errorf("先选择模型目录")
	}
	models, err := readManifest(source)
	if err != nil {
		return err
	}
	for _, model := range models {
		actual, err := dirHashContext(ctx, model.Dir(root), model.ParamFile, model.BinFile)
		if err != nil {
			return err
		}
		if !strings.EqualFold(actual, model.SHA256) {
			return fmt.Errorf("模型 %s 哈希不匹配，拒绝导入", model.Name)
		}
	}
	modelInstallMu.Lock()
	defer modelInstallMu.Unlock()
	return writeTrustedModels(ctx, root, models)
}

func writeTrustedModels(ctx context.Context, root string, models []Model) error {
	existing, err := readManifest(filepath.Join(root, "imageflow-models.json"))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, model := range models {
		replaced := false
		for index := range existing {
			if strings.EqualFold(existing[index].Name, model.Name) {
				existing[index], replaced = model, true
				break
			}
		}
		if !replaced {
			existing = append(existing, model)
		}
	}
	if len(existing) == 0 || len(existing) > 128 {
		return fmt.Errorf("模型数量必须介于 1 与 128")
	}
	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}
	if len(data) > 1<<20 {
		return fmt.Errorf("模型清单不能超过 1 MiB")
	}
	return output.Atomic(filepath.Join(root, "imageflow-models.json"), func(file *os.File) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		_, err := file.Write(data)
		return err
	})
}
