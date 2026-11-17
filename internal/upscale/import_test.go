package upscale

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalModelImportCopiesVerifiesAndDoesNotClaimOfficialTrust(t *testing.T) {
	source, root := t.TempDir(), t.TempDir()
	parameter := filepath.Join(source, "local-4x.param")
	if err := os.WriteFile(parameter, []byte("7767517\n1 1\nInput input 0 1 data\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "local-4x.bin"), []byte{1, 2, 3, 4}, 0o600); err != nil {
		t.Fatal(err)
	}
	model, err := ImportLocalModel(context.Background(), root, parameter, 4)
	if err != nil {
		t.Fatal(err)
	}
	if model.Name != "custom-local-4x" || model.Backend != "upscayl" || model.Purpose != "本地自定义" {
		t.Fatalf("本地导入身份错误：%+v", model)
	}
	statuses := ModelStatuses(root)
	found := false
	for _, status := range statuses {
		if status.Name == model.Name {
			found = status.Installed && status.HashOK
		}
	}
	if !found {
		t.Fatal("导入文件未复制到固定目录或未登记摘要")
	}
	if _, err := ImportLocalModel(context.Background(), root, parameter, 4); err != nil {
		t.Fatalf("相同模型重复导入失败：%v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "local-4x.bin"), []byte{5, 6, 7, 8}, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportLocalModel(context.Background(), root, parameter, 4); err == nil {
		t.Fatal("不同内容覆盖了已导入模型")
	}
}

func TestLocalModelImportRejectsInvalidNCNNAndOldModels(t *testing.T) {
	for _, name := range []string{"invalid", "realesrgan-x4plus", "realesr-animevideov3-x2"} {
		source, root := t.TempDir(), t.TempDir()
		parameter := filepath.Join(source, name+".param")
		if err := os.WriteFile(parameter, []byte("不是 NCNN"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(source, name+".bin"), []byte{1}, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := ImportLocalModel(context.Background(), root, parameter, 4); err == nil {
			t.Fatalf("未拒绝不兼容模型：%s", name)
		}
		entries, err := os.ReadDir(root)
		if err != nil || len(entries) != 0 {
			t.Fatalf("拒绝的导入写入模型目录：%v %v", entries, err)
		}
	}
}
