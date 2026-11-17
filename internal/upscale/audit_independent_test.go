package upscale

import (
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func init() {
	mode := os.Getenv("IMAGEFLOW_REVIEW_UPSCALE_HELPER")
	if mode == "" {
		return
	}
	args := os.Args[1:]
	value := func(flag string) string {
		for index := 0; index+1 < len(args); index++ {
			if args[index] == flag {
				return args[index+1]
			}
		}
		return ""
	}
	target := value("-o")
	if mode == "hang" {
		os.WriteFile(value("-m")+".started", []byte("started"), 0600)
		time.Sleep(30 * time.Second)
		os.Exit(25)
	}
	if mode == "model-name" && strings.HasSuffix(value("-n"), ".param") {
		os.Stderr.WriteString("模型名称不应包含 .param 扩展名")
		os.Exit(23)
	}
	if mode == "failed-write" {
		os.WriteFile(target, []byte("partial output"), 0600)
		os.Exit(9)
	}
	if mode == "corrupt-output" {
		os.WriteFile(target, []byte("not an image"), 0600)
		os.Exit(0)
	}
	output, err := os.Create(target)
	if err != nil {
		os.Exit(24)
	}
	width := 8
	if mode == "wrong-size" {
		width = 7
	}
	img := image.NewNRGBA(image.Rect(0, 0, width, 8))
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	if mode == "wrong-format" {
		jpeg.Encode(output, img, nil)
	} else {
		png.Encode(output, img)
	}
	output.Close()
	os.Exit(0)
}

func auditUpscaleRequest(t *testing.T, mode string) Request {
	t.Helper()
	t.Setenv("IMAGEFLOW_REVIEW_UPSCALE_HELPER", mode)
	original := RegistrySnapshot()
	t.Cleanup(func() { SetRegistry(original) })
	root := t.TempDir()
	model := Model{Name: "audit-model", ParamFile: "audit-model.param", BinFile: "audit-model.bin", Scale: 4}
	if err := os.MkdirAll(model.Dir(root), 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{model.ParamFile, model.BinFile} {
		if err := os.WriteFile(filepath.Join(model.Dir(root), name), []byte("audit"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	hash, err := DirHash(model.Dir(root), model.ParamFile, model.BinFile)
	if err != nil {
		t.Fatal(err)
	}
	model.SHA256 = hash
	SetRegistry([]Model{model})
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(root, "input.png")
	output, err := os.Create(input)
	if err != nil {
		t.Fatal(err)
	}
	png.Encode(output, image.NewNRGBA(image.Rect(0, 0, 2, 2)))
	output.Close()
	return Request{EnginePath: executable, ModelsRoot: root, Model: model.Name, InputPath: input, OutputPath: filepath.Join(root, "out.png"), Scale: 4, GPU: -1}
}

func TestAuditUpscaleModelName(t *testing.T) {
	req := auditUpscaleRequest(t, "model-name")
	if _, err := Run(context.Background(), req); err != nil {
		t.Fatalf("模型名参数不符合引擎协议：%v", err)
	}
}

func TestAuditUpscaleRejectsCorruptOutput(t *testing.T) {
	req := auditUpscaleRequest(t, "corrupt-output")
	if report, err := Run(context.Background(), req); err == nil {
		t.Fatalf("损坏输出被报告成功：%+v", report)
	}
}

func TestAuditUpscaleFailurePreservesTarget(t *testing.T) {
	req := auditUpscaleRequest(t, "failed-write")
	before := []byte("previous approved output")
	if err := os.WriteFile(req.OutputPath, before, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(context.Background(), req); err == nil {
		t.Fatal("预期引擎失败")
	}
	after, err := os.ReadFile(req.OutputPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("失败推理覆盖了已有目标：%q", after)
	}
}

func TestAuditUpscaleRequestedScale(t *testing.T) {
	req := auditUpscaleRequest(t, "valid-output")
	req.Scale = 2
	report, err := Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(req.OutputPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	img, err := png.Decode(file)
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 4 || img.Bounds().Dy() != 4 || report.AppliedScale != 2 {
		t.Fatalf("请求 2 倍，实际 %v、AppliedScale=%d", img.Bounds(), report.AppliedScale)
	}
}

func TestUpscaleRejectsWrongSizeAndFormat(t *testing.T) {
	for _, mode := range []string{"wrong-size", "wrong-format"} {
		t.Run(mode, func(t *testing.T) {
			req := auditUpscaleRequest(t, mode)
			before := []byte("保留旧目标")
			if err := os.WriteFile(req.OutputPath, before, 0600); err != nil {
				t.Fatal(err)
			}
			_, err := Run(context.Background(), req)
			if failure, ok := err.(*Error); !ok || failure.Code != CodeVerifyFailed {
				t.Fatalf("输出未被正确拒绝：%v", err)
			}
			after, err := os.ReadFile(req.OutputPath)
			if err != nil || string(after) != string(before) {
				t.Fatalf("旧目标被修改：%q %v", after, err)
			}
			assertNoUpscaleTemporaries(t, req.OutputPath)
		})
	}
}

func TestUpscaleCancelsRunningChild(t *testing.T) {
	req := auditUpscaleRequest(t, "hang")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan error, 1)
	go func() { _, err := Run(ctx, req); finished <- err }()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(req.ModelsRoot, req.Model) + ".started"); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("辅助进程未启动")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-finished:
		if failure, ok := err.(*Error); !ok || failure.Code != CodeCancelled {
			t.Fatalf("取消结果错误：%v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("取消后辅助进程仍未退出")
	}
	if _, err := os.Stat(req.OutputPath); !os.IsNotExist(err) {
		t.Fatalf("取消仍提交了输出：%v", err)
	}
	assertNoUpscaleTemporaries(t, req.OutputPath)
}

func assertNoUpscaleTemporaries(t *testing.T, target string) {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(filepath.Dir(target), ".imageflow-*"))
	if err != nil || len(paths) != 0 {
		t.Fatalf("临时文件未清理：%v %v", paths, err)
	}
}
