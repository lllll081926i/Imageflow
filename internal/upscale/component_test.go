package upscale

import (
	"archive/zip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func componentArchive(t *testing.T, names []string) string {
	t.Helper()
	archivePath := filepath.Join(t.TempDir(), "component.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	for _, name := range names {
		entry, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte("组件测试内容")); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return archivePath
}

func TestComponentInstallationIsAtomicAndPreservesExistingFiles(t *testing.T) {
	names := []string{"realesrgan-ncnn-vulkan.exe", "vcomp140.dll", "vcomp140d.dll"}
	archive := componentArchive(t, append(names, "../outside.exe"))
	root := t.TempDir()
	for attempt := 0; attempt < 2; attempt++ {
		if err := installEngineArchive(context.Background(), archive, root); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(filepath.Dir(ComponentEnginePath(root)))
	if err != nil || len(entries) != 3 {
		t.Fatalf("组件文件不完整：%v %v", entries, err)
	}
	if err := os.WriteFile(ComponentEnginePath(root), []byte("已有内容"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := installEngineArchive(context.Background(), archive, root); err == nil {
		t.Fatal("覆盖了已有不同内容")
	}
	data, err := os.ReadFile(ComponentEnginePath(root))
	if err != nil || string(data) != "已有内容" {
		t.Fatal("原有组件被破坏")
	}
	staged, _ := filepath.Glob(filepath.Join(root, ".imageflow-engine-*"))
	if len(staged) != 0 {
		t.Fatal("组件暂存未清理")
	}
}

func TestComponentRejectsIncompleteDuplicateAndCancelledArchives(t *testing.T) {
	for _, names := range [][]string{
		{"realesrgan-ncnn-vulkan.exe"},
		{"realesrgan-ncnn-vulkan.exe", "realesrgan-ncnn-vulkan.exe", "vcomp140.dll", "vcomp140d.dll"},
	} {
		root := t.TempDir()
		if err := installEngineArchive(context.Background(), componentArchive(t, names), root); err == nil {
			t.Fatal("接受了不完整或重复组件")
		}
		if _, err := os.Stat(ComponentEnginePath(root)); !os.IsNotExist(err) {
			t.Fatal("失败安装残留组件")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := installEngineArchive(ctx, "", t.TempDir()); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestComponentLiveDownload(t *testing.T) {
	if os.Getenv("IMAGEFLOW_COMPONENT_LIVE") != "1" {
		t.Skip("需显式启用组件联网验收")
	}
	root := os.Getenv("IMAGEFLOW_COMPONENT_LIVE_DIRECTORY")
	if root == "" {
		t.Fatal("必须指定独立验收目录")
	}
	root = filepath.Join(root, "models")
	model, err := DownloadComponent(context.Background(), root, DownloadableModels()[0].Name, DownloadOptions{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	environment := Detect(EnginePathForModel(root, model), root)
	if !environment.Available {
		t.Fatal(environment.Reason)
	}
	verified := false
	for _, status := range environment.Models {
		if status.Name == model.Name && status.HashOK && status.Installed {
			verified = true
		}
	}
	if !verified {
		t.Fatal("下载的模型未通过校验")
	}
	input := os.Getenv("IMAGEFLOW_COMPONENT_LIVE_INPUT")
	if input == "" {
		t.Fatal("必须指定推理验收输入")
	}
	report, err := Run(context.Background(), Request{
		EnginePath: EnginePathForModel(root, model), ModelsRoot: root, Model: model.Name,
		InputPath: input, OutputPath: filepath.Join(root, "acceptance-output.png"),
		Scale: 2, Tile: 64, GPU: 0, Format: "png",
	})
	if err != nil || report.Width <= 0 || report.Height <= 0 {
		t.Fatalf("自动安装后的真实推理失败：%v", err)
	}
	t.Logf("真实 GPU 输出：%d×%d", report.Width, report.Height)
	t.Logf("组件下载、引擎启动和模型校验通过：%s", root)
}
