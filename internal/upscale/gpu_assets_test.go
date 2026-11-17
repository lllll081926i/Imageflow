package upscale

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func prepareUpscaylLive(t *testing.T, root string) string {
	t.Helper()
	base := os.Getenv("IMAGEFLOW_UPSCAYL_AUDIT")
	if !filepath.IsAbs(base) {
		t.Fatal("必须指定固定提交资源目录 IMAGEFLOW_UPSCAYL_AUDIT 的绝对路径")
	}
	resources, err := upscaylEngineResources()
	if err != nil {
		t.Fatal(err)
	}
	models := UpscaylModels()
	engine := EnginePathForModel(root, models[0])
	if err := os.MkdirAll(filepath.Dir(engine), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, resource := range resources {
		data, err := os.ReadFile(filepath.Join(base, filepath.FromSlash(resource.path)))
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(data)
		if hex.EncodeToString(digest[:]) != resource.hash {
			t.Fatalf("引擎资源摘要不匹配：%s", resource.path)
		}
		if err := os.WriteFile(filepath.Join(filepath.Dir(engine), filepath.Base(resource.path)), data, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, model := range models {
		if err := os.MkdirAll(model.Dir(root), 0o755); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{model.ParamFile, model.BinFile} {
			data, err := os.ReadFile(filepath.Join(base, "models", name))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(model.Dir(root), name), data, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		actual, err := DirHash(model.Dir(root), model.ParamFile, model.BinFile)
		if err != nil || actual != model.SHA256 {
			t.Fatalf("模型资源摘要不匹配：%s %v", model.Name, err)
		}
	}
	if err := writeTrustedModels(context.Background(), root, models); err != nil {
		t.Fatal(err)
	}
	return engine
}
