package upscale

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLegacyMigrationMovesOnlyOldAssetsAndKeepsCustomModels(t *testing.T) {
	root := t.TempDir()
	old := Model{Name: "realesrgan-x4plus", ParamFile: "realesrgan-x4plus.param", BinFile: "realesrgan-x4plus.bin",
		SHA256: strings.Repeat("a", 64), Scale: 4, Purpose: "旧模型", License: "旧许可"}
	custom := old
	custom.Name, custom.ParamFile, custom.BinFile = "custom-photo", "photo.param", "photo.bin"
	for _, model := range []Model{old, custom} {
		if err := os.MkdirAll(model.Dir(root), 0o755); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{model.ParamFile, model.BinFile} {
			if err := os.WriteFile(filepath.Join(model.Dir(root), name), []byte(name), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := os.WriteFile(filepath.Join(old.Dir(root), "用户笔记.txt"), []byte("保留"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "engine"), 0o755); err != nil {
		t.Fatal(err)
	}
	engine := filepath.Join(root, "engine", "realesrgan-ncnn-vulkan.exe")
	if err := os.WriteFile(engine, []byte("旧引擎"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeTrustedModels(context.Background(), root, []Model{old, custom}); err != nil {
		t.Fatal(err)
	}
	if err := MigrateLegacyModels(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{engine, filepath.Join(old.Dir(root), old.ParamFile), filepath.Join(old.Dir(root), old.BinFile)} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("旧资源仍在工作目录：%s，%v", path, err)
		}
	}
	for _, path := range []string{filepath.Join(custom.Dir(root), custom.ParamFile), filepath.Join(custom.Dir(root), custom.BinFile), filepath.Join(old.Dir(root), "用户笔记.txt")} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("不应修改用户资源：%s，%v", path, err)
		}
	}
	archives, _ := filepath.Glob(filepath.Join(root, ".trash", "legacy-*"))
	if len(archives) != 1 {
		t.Fatalf("回收目录数量错误：%v", archives)
	}
	if data, err := os.ReadFile(filepath.Join(archives[0], old.Name, old.BinFile)); err != nil || string(data) != old.BinFile {
		t.Fatalf("旧模型未完整保留在回收目录：%v", err)
	}
	kept, err := readManifest(filepath.Join(root, "imageflow-models.json"))
	if err != nil || len(kept) != 1 || kept[0].Name != custom.Name {
		t.Fatalf("自定义模型记录未保留：%v %v", kept, err)
	}
	if err := MigrateLegacyModels(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	again, _ := filepath.Glob(filepath.Join(root, ".trash", "legacy-*"))
	if len(again) != len(archives) {
		t.Fatal("重复启动重复生成回收目录")
	}
}

func TestLegacyMigrationArchivesManifestWhenOnlyLegacyModelsRemain(t *testing.T) {
	root := t.TempDir()
	model := Model{Name: "realesr-animevideov3", ParamFile: "realesr-animevideov3-x2.param", BinFile: "realesr-animevideov3-x2.bin",
		SHA256: strings.Repeat("a", 64), Scale: 2, Purpose: "旧模型", License: "旧许可"}
	if err := writeTrustedModels(context.Background(), root, []Model{model}); err != nil {
		t.Fatal(err)
	}
	if err := MigrateLegacyModels(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	models, err := Models(root)
	if err != nil || len(models) != 7 {
		t.Fatalf("迁移后没有正常加载七个新模型：%v %v", models, err)
	}
	if _, err := os.Stat(filepath.Join(root, "imageflow-models.json")); !os.IsNotExist(err) {
		t.Fatalf("只含旧模型的清单未移出：%v", err)
	}
}

func TestLegacyMigrationProtectsNonDirectoryAndCancellation(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "realesrgan-x4plus"), []byte("用户文件"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := MigrateLegacyModels(context.Background(), root); err == nil {
		t.Fatal("未拒绝被用户文件占用的目录")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	root = t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "engine"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := MigrateLegacyModels(ctx, root); !errors.Is(err, context.Canceled) {
		t.Fatalf("取消未返回：%v", err)
	}
}

func TestCatalogueDoesNotOfferLegacyModelsEvenWithOldManifest(t *testing.T) {
	root := t.TempDir()
	legacy := Model{Name: "realesrgan-x4plus", ParamFile: "realesrgan-x4plus.param", BinFile: "realesrgan-x4plus.bin",
		SHA256: strings.Repeat("a", 64), Scale: 4, Purpose: "旧模型", License: "旧许可"}
	data, _ := json.Marshal([]Model{legacy})
	if err := os.WriteFile(filepath.Join(root, "imageflow-models.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	models, err := Models(root)
	if err != nil || len(models) != 7 {
		t.Fatalf("目录仍提供旧模型：%v %v", models, err)
	}
	if _, err := DownloadComponent(context.Background(), root, legacy.Name, DownloadOptions{}, nil); err == nil {
		t.Fatal("仍允许下载旧模型")
	}
}
