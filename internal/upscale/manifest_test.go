package upscale

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManifestImportPersistsAndRejectsTampering(t *testing.T) {
	req := auditUpscaleRequest(t, "valid-output")
	model := RegistrySnapshot()[0]
	data, err := json.Marshal([]Model{{Name: model.Name, ParamFile: model.ParamFile, BinFile: model.BinFile,
		SHA256: model.SHA256, Scale: model.Scale, Purpose: "测试图片", License: "测试许可"}})
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "models.json")
	if err := os.WriteFile(source, data, 0600); err != nil {
		t.Fatal(err)
	}
	model.SHA256 = ""
	SetRegistry([]Model{model})
	if err := ImportManifest(source, req.ModelsRoot); err != nil {
		t.Fatal(err)
	}
	loaded, err := Models(req.ModelsRoot)
	if err != nil || len(loaded) != 1 || loaded[0].SHA256 == "" {
		t.Fatalf("可信清单未持久化：%v %v", loaded, err)
	}
	if statuses := ModelStatuses(req.ModelsRoot); len(statuses) != 1 || !statuses[0].HashOK {
		t.Fatalf("模型未通过校验：%v", statuses)
	}
	before, err := os.ReadFile(filepath.Join(req.ModelsRoot, "imageflow-models.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(model.Dir(req.ModelsRoot), model.BinFile), []byte("篡改"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := ImportManifest(source, req.ModelsRoot); err == nil {
		t.Fatal("篡改权重仍被信任")
	}
	after, err := os.ReadFile(filepath.Join(req.ModelsRoot, "imageflow-models.json"))
	if err != nil || string(before) != string(after) {
		t.Fatal("失败导入修改了可信清单")
	}
	if ModelStatuses(req.ModelsRoot)[0].HashOK {
		t.Fatal("篡改后仍通过校验")
	}
}

func TestManifestRejectsUnsafeAndInvalidEntries(t *testing.T) {
	valid := Model{Name: "model", ParamFile: "model.param", BinFile: "model.bin", Scale: 4,
		SHA256: strings.Repeat("a", 64), Purpose: "测试", License: "测试"}
	for _, change := range []func(*Model){
		func(model *Model) { model.Name = "../outside" },
		func(model *Model) { model.ParamFile = "../outside.param" },
		func(model *Model) { model.SHA256 = "" },
		func(model *Model) { model.Scale = 8 },
		func(model *Model) { model.BinFile = "different.bin" },
	} {
		model := valid
		change(&model)
		data, err := json.Marshal([]Model{model})
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "models.json")
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := readManifest(path); err == nil {
			t.Fatalf("无效清单未被拒绝：%s", data)
		}
	}
}

func TestAnimeVideoModelUsesNativeFileSuffix(t *testing.T) {
	model := defaultRegistry()[2]
	if model.engineName() != "realesr-animevideov3" || model.ParamFile != "realesr-animevideov3-x2.param" || model.BinFile != "realesr-animevideov3-x2.bin" {
		t.Fatalf("动漫视频模型协议不正确：%+v", model)
	}
}

func TestManifestMergeRejectsOversizedRegistry(t *testing.T) {
	root := t.TempDir()
	models := make([]Model, 128)
	for index := range models {
		models[index] = Model{Name: fmt.Sprintf("model-%d", index), ParamFile: "model.param", BinFile: "model.bin",
			SHA256: strings.Repeat("a", 64), Scale: 4, Purpose: "测试", License: "测试"}
	}
	if err := writeTrustedModels(context.Background(), root, models); err != nil {
		t.Fatal(err)
	}
	additional := models[0]
	additional.Name = "additional"
	if err := writeTrustedModels(context.Background(), root, []Model{additional}); err == nil {
		t.Fatal("合并后超限的清单未被拒绝")
	}
	loaded, err := readManifest(filepath.Join(root, "imageflow-models.json"))
	if err != nil || len(loaded) != 128 {
		t.Fatalf("失败合并破坏已有清单：%d %v", len(loaded), err)
	}
}
