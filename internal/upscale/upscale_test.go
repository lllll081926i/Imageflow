package upscale

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRunRefusesToOverwriteInput(t *testing.T) {
	input := filepath.Join(t.TempDir(), "source.png")
	before := []byte("原图必须保留")
	if err := os.WriteFile(input, before, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, double := range []bool{false, true} {
		_, err := Run(context.Background(), Request{InputPath: input, OutputPath: input, Double: double})
		if failure, ok := err.(*Error); !ok || failure.Code != CodeBadRequest || !strings.Contains(failure.Message, "覆盖原图") {
			t.Fatalf("未拒绝原图覆盖：%v", err)
		}
	}
	after, err := os.ReadFile(input)
	if err != nil || string(after) != string(before) {
		t.Fatal("原图被更改")
	}
}

// TestDetectWithoutEngineReportsTheMissingComponent is E2E-13's precondition: with
// no engine configured the package must answer with a reason, not panic and not
// claim an acceleration it does not have.
func TestDetectWithoutEngineReportsTheMissingComponent(t *testing.T) {
	env := Detect("", t.TempDir())
	if env.Available {
		t.Fatal("an unconfigured engine must not report availability")
	}
	if !strings.Contains(env.Reason, "未配置") {
		t.Fatalf("reason = %q", env.Reason)
	}
}

func TestDetectWithMissingPathExplainsWhy(t *testing.T) {
	env := Detect(filepath.Join(t.TempDir(), "nope.exe"), t.TempDir())
	if env.Available {
		t.Fatal("a missing binary must not report availability")
	}
	if !strings.Contains(env.Reason, "不存在") {
		t.Fatalf("reason = %q", env.Reason)
	}
}

func TestRunRejectsAnUnregisteredModel(t *testing.T) {
	dir := t.TempDir()
	engine := writeScript(t, dir, "engine", "@echo -i -g -s\n")
	_, err := Run(context.Background(), Request{
		EnginePath: engine, ModelsRoot: dir, Model: "not-a-model",
		InputPath: "in.png", OutputPath: filepath.Join(dir, "out.png"),
	})
	if err == nil {
		t.Fatal("an unregistered model must be refused")
	}
	if e, ok := err.(*Error); !ok || e.Code != CodeModelMissing {
		t.Fatalf("err = %v", err)
	}
}

func TestRunRefusesAModelWithoutARecordedHash(t *testing.T) {
	dir := t.TempDir()
	models := dir + "/models"
	model := RegistrySnapshot()[0]
	model.SHA256 = ""
	SetRegistry([]Model{model})
	t.Cleanup(func() { SetRegistry(defaultRegistry()) })
	if err := os.MkdirAll(model.Dir(models), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{model.ParamFile, model.BinFile} {
		if err := os.WriteFile(filepath.Join(model.Dir(models), name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_, err := Run(context.Background(), Request{
		EnginePath: writeScript(t, dir, "engine", "@echo -i\n"),
		ModelsRoot: models, Model: model.Name,
		InputPath: "in.png", OutputPath: filepath.Join(dir, "out.png"),
	})
	if err == nil || !strings.Contains(err.Error(), "未登记哈希") {
		t.Fatalf("err = %v, want a hash refusal", err)
	}
}

func TestRunRefusesATamperedModel(t *testing.T) {
	dir := t.TempDir()
	models := dir + "/models"
	base := RegistrySnapshot()[0]
	model := base
	model.SHA256 = strings.Repeat("0", 64)
	SetRegistry([]Model{model})
	t.Cleanup(func() { SetRegistry(defaultRegistry()) })
	if err := os.MkdirAll(model.Dir(models), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{model.ParamFile, model.BinFile} {
		if err := os.WriteFile(filepath.Join(model.Dir(models), name), []byte("payload"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_, err := Run(context.Background(), Request{
		EnginePath: writeScript(t, dir, "engine", "@echo -i\n"),
		ModelsRoot: models, Model: model.Name,
		InputPath: "in.png", OutputPath: filepath.Join(dir, "out.png"),
	})
	if err == nil || !strings.Contains(err.Error(), "哈希不匹配") {
		t.Fatalf("err = %v, want a hash mismatch", err)
	}
}

func TestDirHashIsOrderSensitive(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a", "b"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ab, err := DirHash(dir, "a", "b")
	if err != nil {
		t.Fatal(err)
	}
	ba, err := DirHash(dir, "b", "a")
	if err != nil {
		t.Fatal(err)
	}
	if ab == ba {
		t.Fatal("the hash must follow the declared file order")
	}
	h := sha256.Sum256([]byte("ab"))
	if ab != hex.EncodeToString(h[:]) {
		t.Fatal("the hash is not the sha256 of the concatenated files")
	}
}

// TestRunRejectsAnUnsupportedOutputFormat keeps the format list honest: the engine
// writes png, jpg or webp only.
func TestRunRejectsAnUnsupportedOutputFormat(t *testing.T) {
	dir := t.TempDir()
	_, err := Run(context.Background(), Request{
		EnginePath: writeScript(t, dir, "engine", "@echo -i\n"),
		ModelsRoot: dir, Model: "realesrgan-x4plus",
		InputPath: "in.png", OutputPath: filepath.Join(dir, "out.avif"), Format: "avif",
	})
	if e, ok := err.(*Error); !ok || e.Code != CodeBadRequest {
		t.Fatalf("err = %v", err)
	}
}

func TestLooksLikeOOMRecognisesTheEngineSpelling(t *testing.T) {
	for _, text := range []string{"vkAllocateMemory failed", "out of memory", "Vulkan failed to allocate"} {
		if !looksLikeOOM(text) {
			t.Errorf("%q not recognised", text)
		}
	}
	if looksLikeOOM("file not found") {
		t.Error("an unrelated failure was classified as OOM")
	}
}

func TestRegistryEntriesCarryLicenceAndPurpose(t *testing.T) {
	for _, m := range RegistrySnapshot() {
		if m.License == "" || m.Purpose == "" {
			t.Errorf("%s: missing licence or purpose", m.Name)
		}
		if m.Scale < 2 {
			t.Errorf("%s: implausible native scale %d", m.Name, m.Scale)
		}
		if len(m.SHA256) != 64 || m.Backend != "upscayl" {
			t.Errorf("%s：缺少固定摘要或 Upscayl 后端", m.Name)
		}
	}
}

// writeScript writes a tiny stand-in for the engine binary. The real engine is an
// optional component, so the tests exercise the argument build and the refusal
// paths with a script that echoes its arguments.
func writeScript(t *testing.T, dir, name, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		path := filepath.Join(dir, name+".bat")
		content := "@echo off\r\n" + strings.TrimPrefix(body, "@echo ") + "\r\n"
		if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
		return path
	}
	path := filepath.Join(dir, name+".sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+strings.TrimPrefix(body, "@echo ")), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDetectOffersAModelList(t *testing.T) {
	env := Detect("", t.TempDir())
	if len(env.Models) == 0 {
		t.Fatal("the page needs the catalogue even when no engine is installed")
	}
	for _, st := range env.Models {
		if st.Installed && st.HashOK {
			t.Fatalf("%s: nothing is installed in an empty root", st.Name)
		}
	}
}

// TestCancelIsReportedAsCancellation pins the cancellation contract: the caller
// must be able to tell a cancellation from an engine failure.
func TestCancelIsReportedAsCancellation(t *testing.T) {
	dir := t.TempDir()
	models := dir + "/models"
	base := RegistrySnapshot()[0]
	model := base
	if err := os.MkdirAll(model.Dir(models), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{model.ParamFile, model.BinFile} {
		if err := os.WriteFile(filepath.Join(model.Dir(models), name), []byte("p"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	hash, err := DirHash(model.Dir(models), model.ParamFile, model.BinFile)
	if err != nil {
		t.Fatal(err)
	}
	model.SHA256 = hash
	SetRegistry([]Model{model})
	t.Cleanup(func() { SetRegistry(defaultRegistry()) })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, runErr := Run(ctx, Request{
		EnginePath: writeScript(t, dir, "engine", "@echo -i -g\n"),
		ModelsRoot: models, Model: model.Name,
		InputPath: "in.png", OutputPath: filepath.Join(dir, "out.png"),
		Timeout: 2 * time.Second,
	})
	if e, ok := runErr.(*Error); !ok || e.Code != CodeCancelled {
		t.Fatalf("err = %v, want CANCELLED", runErr)
	}
}
