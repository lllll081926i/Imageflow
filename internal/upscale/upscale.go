// Package upscale is the optional AI super-resolution adapter.
//
// The design follows the PRD's boundary exactly: the core build never needs an
// inference engine, and this package is the only place that talks to one. It
// speaks the Real-ESRGAN-ncnn-vulkan command line through an absolute path and an
// explicit argument list — never a shell string — and it treats the engine as
// untrusted: the output is verified by decoding it before the result is accepted.
//
// Everything here is inert until a user points the app at an engine binary and a
// model directory. There is no download, no auto-install and no bundled weight.
package upscale

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/disintegration/imaging"
	"github.com/lllll081926i/imageflow-go/internal/codec"
	"github.com/lllll081926i/imageflow-go/internal/output"
)

// Error codes the UI maps to its recovery hints.
const (
	CodeEngineMissing  = "ENGINE_MISSING"
	CodeModelMissing   = "MODEL_MISSING"
	CodeGPUUnavailable = "GPU_UNAVAILABLE"
	CodeEngineFailed   = "ENGINE_FAILED"
	CodeCancelled      = "CANCELLED"
	CodeVerifyFailed   = "VERIFY_FAILED"
	CodeBadRequest     = "BAD_REQUEST"
)

// Error is one adapter failure with a code the page can act on.
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Message }

func newError(code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// Model is one registered weight set.
//
// A model is only usable when its SHA256 matches the recorded hash: the PRD
// requires that a corrupt or swapped weight file is refused rather than loaded.
type Model struct {
	// Name is the label the page shows.
	Name string `json:"name"`
	// ParamFile is the .param file inside Dir.
	ParamFile string `json:"param_file"`
	// BinFile is the .bin file inside Dir.
	BinFile string `json:"bin_file"`
	// SHA256 is the expected hash of the directory's files joined in name order.
	// An empty hash means "not recorded yet": the model lists as unverified.
	SHA256 string `json:"sha256,omitempty"`
	// Scale is the model's native upscale factor.
	Scale int `json:"scale"`
	// Purpose is the picture family the model is trained for.
	Purpose string `json:"purpose"`
	// License names the weight licence and where it came from.
	License string `json:"license"`
}

// Dir resolves the model's directory inside modelsRoot.
func (m Model) Dir(modelsRoot string) string { return filepath.Join(modelsRoot, m.Name) }

// Installed reports whether the model's files exist.
func (m Model) Installed(modelsRoot string) bool {
	dir := m.Dir(modelsRoot)
	for _, name := range []string{m.ParamFile, m.BinFile} {
		if name == "" {
			return false
		}
		if st, err := os.Stat(filepath.Join(dir, name)); err != nil || st.IsDir() {
			return false
		}
	}
	return true
}

// ModelStatus is what the page shows for one registered model.
type ModelStatus struct {
	Model
	Installed  bool   `json:"installed"`
	HashOK     bool   `json:"hash_ok"`
	ActualHash string `json:"actual_hash,omitempty"`
	Note       string `json:"note,omitempty"`
}

// Environment is the detected state of the optional component.
type Environment struct {
	// EnginePath is the executable in use, empty when not configured.
	EnginePath string `json:"engine_path"`
	// Version is the engine's reported version, when it reports one.
	Version string `json:"version,omitempty"`
	// Available is true only when the executable exists and answers a probe.
	Available bool `json:"available"`
	// GPUDevices lists the Vulkan devices the engine printed, if any.
	GPUDevices []string `json:"gpu_devices,omitempty"`
	// Reason explains an unavailable environment in the page's words.
	Reason string `json:"reason,omitempty"`
	// OS is the platform, which the page shows next to the version.
	OS         string        `json:"os"`
	ModelsRoot string        `json:"models_root"`
	Models     []ModelStatus `json:"models,omitempty"`
}

// Detect probes the configured engine.
//
// The probe runs `<engine> -h` with a short timeout: Real-ESRGAN-ncnn-vulkan
// prints its usage and exits non-zero, so a non-zero status with usage text is a
// success here, while a missing file or a spawn error is not.
func Detect(enginePath, modelsRoot string) Environment {
	return DetectContext(context.Background(), enginePath, modelsRoot)
}

func DetectContext(parent context.Context, enginePath, modelsRoot string) Environment {
	env := Environment{
		EnginePath: strings.TrimSpace(enginePath),
		OS:         runtime.GOOS + "/" + runtime.GOARCH,
		ModelsRoot: modelsRoot,
	}
	env.Models = modelStatusesContext(parent, modelsRoot)
	if parent.Err() != nil {
		env.Reason = "组件探测已取消"
		return env
	}
	if env.EnginePath == "" {
		env.Reason = "未配置本地推理引擎；AI 高清放大需要单独安装可选的 Real-ESRGAN-ncnn-vulkan 组件"
		return env
	}
	info, err := os.Stat(env.EnginePath)
	if err != nil || info.IsDir() {
		env.Reason = "引擎路径不存在或不是可执行文件：" + env.EnginePath
		return env
	}
	ctx, cancel := context.WithTimeout(parent, 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, env.EnginePath, "-h")
	hideWindow(cmd)
	cmd.Cancel = func() error { killTree(cmd); return nil }
	cmd.WaitDelay = 3 * time.Second
	out, runErr := cmd.CombinedOutput()
	text := string(out)
	if strings.Contains(text, "vulkan") || strings.Contains(text, "-i") || strings.Contains(text, "-g") {
		env.Available = true
		env.Version = firstLine(text)
		env.GPUDevices = gpuLines(text)
		return env
	}
	if runErr != nil {
		env.Reason = "引擎无法运行：" + strings.TrimSpace(firstLine(text))
		if env.Reason == "引擎无法运行：" {
			env.Reason = "引擎无法运行：" + runErr.Error()
		}
		return env
	}
	env.Reason = "引擎未返回可识别的参数说明，可能不是 Real-ESRGAN-ncnn-vulkan"
	return env
}

// firstLine returns the first non-empty line of text.
func firstLine(text string) string {
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			return line
		}
	}
	return ""
}

// gpuLines picks the device lines out of the engine's usage text.
func gpuLines(text string) []string {
	var out []string
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		lower := strings.ToLower(line)
		if strings.Contains(lower, "gpu") && (strings.Contains(lower, "id") || strings.Contains(lower, "device")) {
			out = append(out, line)
		}
	}
	return out
}

// ModelStatuses resolves every registered model against the models root.
func ModelStatuses(modelsRoot string) []ModelStatus {
	return modelStatusesContext(context.Background(), modelsRoot)
}

func modelStatusesContext(ctx context.Context, modelsRoot string) []ModelStatus {
	models, manifestErr := Models(modelsRoot)
	out := make([]ModelStatus, 0, len(models))
	for _, m := range models {
		st := ModelStatus{Model: m, Installed: m.Installed(modelsRoot)}
		if manifestErr != nil {
			st.Note = manifestErr.Error()
			out = append(out, st)
			continue
		}
		if st.Installed {
			hash, err := dirHashContext(ctx, m.Dir(modelsRoot), m.ParamFile, m.BinFile)
			if err != nil {
				st.Note = "无法计算哈希：" + err.Error()
				out = append(out, st)
				continue
			}
			st.ActualHash = hash
			st.HashOK = m.SHA256 != "" && strings.EqualFold(m.SHA256, hash)
			if m.SHA256 == "" {
				st.Note = "未登记哈希，不能启用"
			} else if !st.HashOK {
				st.Note = "哈希不匹配，拒绝加载"
			}
		}
		out = append(out, st)
	}
	return out
}

// DirHash hashes the given files of a directory in first,second order.
func DirHash(dir string, names ...string) (string, error) {
	return dirHashContext(context.Background(), dir, names...)
}

func dirHashContext(ctx context.Context, dir string, names ...string) (string, error) {
	h := sha256.New()
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(h, contextReader{ctx, f}); err != nil {
			f.Close()
			return "", err
		}
		f.Close()
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader contextReader) Read(data []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	return reader.reader.Read(data)
}

// Request is one upscale run.
type Request struct {
	EnginePath string `json:"engine_path"`
	ModelsRoot string `json:"models_root"`
	Model      string `json:"model"`
	InputPath  string `json:"input_path"`
	OutputPath string `json:"output_path"`

	// Scale is the requested factor. The engine only performs its model's native
	// factor, so a different target is reached by a Go resize afterwards, and the
	// result records that.
	Scale int `json:"scale"`
	// Tile is 0 for the engine's automatic choice.
	Tile int `json:"tile"`
	// TTA asks the engine for test-time augmentation.
	TTA bool `json:"tta"`
	// GPU is the device id, -1 for automatic.
	GPU int `json:"gpu"`
	// Format is the engine's output format: png, jpg or webp.
	Format string `json:"format"`

	// Timeout bounds the run. Zero means five minutes.
	Timeout time.Duration `json:"-"`
}

// Report is the accepted result of one run.
type Report struct {
	OutputPath  string
	Model       string
	NativeScale int
	// AppliedScale is the factor the output really has, which may differ from the
	// model's native one.
	AppliedScale  int
	Width, Height int
	Elapsed       time.Duration
	// Warnings carries the honest caveats: a post-resize, a forced tile size.
	Warnings []string
}

// Run executes one upscale through the engine.
//
// Cancellation kills the child process; the caller's temporary directory is
// removed by the caller, so a cancelled run leaves nothing behind.
func Run(ctx context.Context, req Request) (*Report, error) {
	if ctx.Err() != nil {
		return nil, newError(CodeCancelled, "已取消")
	}
	if strings.TrimSpace(req.InputPath) == "" || strings.TrimSpace(req.OutputPath) == "" {
		return nil, newError(CodeBadRequest, "缺少输入或输出路径")
	}
	if strings.TrimSpace(req.EnginePath) == "" {
		return nil, newError(CodeEngineMissing, "未配置本地推理引擎")
	}
	format := strings.ToLower(strings.TrimSpace(req.Format))
	switch format {
	case "", "png":
		format = "png"
	case "jpg", "jpeg":
		format = "jpg"
	case "webp":
		format = "webp"
	default:
		return nil, newError(CodeBadRequest, "引擎不支持的输出格式：%s", req.Format)
	}
	if info, err := os.Stat(req.EnginePath); err != nil || info.IsDir() {
		return nil, newError(CodeEngineMissing, "引擎路径不存在：%s", req.EnginePath)
	}
	var model Model
	found := false
	models, err := Models(req.ModelsRoot)
	if err != nil {
		return nil, newError(CodeModelMissing, "模型清单无效：%v", err)
	}
	for _, m := range models {
		if strings.EqualFold(m.Name, req.Model) {
			model, found = m, true
			break
		}
	}
	if !found {
		return nil, newError(CodeModelMissing, "未登记的模型：%s", req.Model)
	}
	if !model.Installed(req.ModelsRoot) {
		return nil, newError(CodeModelMissing, "模型文件缺失：%s", model.Dir(req.ModelsRoot))
	}
	if model.SHA256 == "" {
		return nil, newError(CodeModelMissing, "模型未登记哈希，不能启用：%s", model.Name)
	}
	actual, err := dirHashContext(ctx, model.Dir(req.ModelsRoot), model.ParamFile, model.BinFile)
	if err != nil {
		if ctx.Err() != nil {
			return nil, newError(CodeCancelled, "已取消")
		}
		return nil, newError(CodeModelMissing, "模型哈希读取失败：%v", err)
	}
	if !strings.EqualFold(actual, model.SHA256) {
		return nil, newError(CodeModelMissing, "模型哈希不匹配，拒绝加载：%s", model.Name)
	}
	source, err := codec.OpenSRGB(req.InputPath)
	if err != nil {
		return nil, newError(CodeBadRequest, "输入图像无效：%v", err)
	}
	inputWidth, inputHeight := source.Image.Bounds().Dx(), source.Image.Bounds().Dy()
	scale := req.Scale
	if scale == 0 {
		scale = model.Scale
	}
	if scale < 2 || scale > 4 || model.Scale < 2 || model.Scale > 4 {
		return nil, newError(CodeBadRequest, "倍率必须介于 2 与 4")
	}
	if err := output.CheckSize(inputWidth*model.Scale, inputHeight*model.Scale); err != nil {
		return nil, newError(CodeBadRequest, "原生输出超限：%v", err)
	}
	if err := output.CheckSize(inputWidth*scale, inputHeight*scale); err != nil {
		return nil, newError(CodeBadRequest, "目标输出超限：%v", err)
	}
	if err := os.MkdirAll(filepath.Dir(req.OutputPath), 0o755); err != nil {
		return nil, newError(CodeBadRequest, "输出目录不可用：%v", err)
	}
	inputPath, err := writeInferenceInput(ctx, source.Image, filepath.Dir(req.OutputPath))
	if err != nil {
		if ctx.Err() != nil {
			return nil, newError(CodeCancelled, "已取消")
		}
		return nil, newError(CodeBadRequest, "准备推理输入失败：%v", err)
	}
	defer os.Remove(inputPath)
	temporary, err := os.CreateTemp(filepath.Dir(req.OutputPath), ".imageflow-upscale-*."+format)
	if err != nil {
		return nil, newError(CodeEngineFailed, "创建临时输出失败：%v", err)
	}
	temporaryPath := temporary.Name()
	if err := temporary.Close(); err != nil {
		os.Remove(temporaryPath)
		return nil, err
	}
	defer os.Remove(temporaryPath)
	tile := req.Tile
	if tile < 0 {
		tile = 0
	}
	if tile > 0 && tile < 32 {
		return nil, newError(CodeBadRequest, "Tile 必须为 0 或不小于 32")
	}
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := []string{
		"-i", inputPath,
		"-o", temporaryPath,
		"-m", model.Dir(req.ModelsRoot),
		"-n", model.engineName(),
		"-f", format,
	}
	// The scale flag is the model's native factor: passing anything else makes the
	// engine fail or silently use its own, so it is never mapped from the UI.
	args = append(args, "-s", fmt.Sprint(model.Scale))
	if tile > 0 {
		args = append(args, "-t", fmt.Sprint(tile))
	}
	if req.TTA {
		args = append(args, "-x")
	}
	if req.GPU >= 0 {
		args = append(args, "-g", fmt.Sprint(req.GPU))
	}
	cmd := exec.CommandContext(runCtx, req.EnginePath, args...)
	hideWindow(cmd)
	// Kill the whole tree on cancellation: a launcher process may spawn the real
	// inference process, and killing only the parent leaves the GPU busy.
	cmd.Cancel = func() error {
		killTree(cmd)
		return nil
	}
	cmd.WaitDelay = 3 * time.Second
	var stderr strings.Builder
	cmd.Stderr = &stderr
	started := time.Now()
	runErr := cmd.Run()
	elapsed := time.Since(started)
	if runCtx.Err() != nil {
		return nil, newError(CodeCancelled, "已取消")
	}
	if runErr != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = runErr.Error()
		}
		if looksLikeOOM(detail) {
			return nil, newError(CodeGPUUnavailable, "显存不足或 GPU 不可用，请降低 Tile 或关闭 TTA：%s", firstLine(detail))
		}
		return nil, newError(CodeEngineFailed, "推理进程失败：%s", firstLine(detail))
	}
	width, height, actualFormat, err := codec.Probe(temporaryPath)
	if err == nil {
		file, openErr := os.Open(temporaryPath)
		if openErr != nil {
			err = openErr
		} else {
			_, actualFormat, err = image.DecodeConfig(file)
			file.Close()
			if actualFormat == "apng" {
				actualFormat = "png"
			}
		}
	}
	if err != nil || width != inputWidth*model.Scale || height != inputHeight*model.Scale ||
		!strings.EqualFold(strings.TrimPrefix(actualFormat, "."), format) && !(format == "jpg" && strings.EqualFold(actualFormat, "jpeg")) {
		return nil, newError(CodeVerifyFailed, "引擎输出格式或尺寸不正确：%dx%d %s（%v）", width, height, actualFormat, err)
	}
	decoded, err := codec.Open(temporaryPath)
	if err != nil {
		return nil, newError(CodeVerifyFailed, "引擎输出解码失败：%v", err)
	}
	var finalImage image.Image = decoded.Image
	alphaRestored := false
	if format != "jpg" {
		finalImage, alphaRestored, err = restoreInputAlpha(runCtx, finalImage, source.Image)
		if err != nil {
			return nil, newError(CodeCancelled, "已取消")
		}
	}
	if scale != model.Scale {
		finalImage = imaging.Resize(finalImage, inputWidth*scale, inputHeight*scale, imaging.Lanczos)
	}
	if scale != model.Scale || alphaRestored {
		if err := codec.SaveContext(runCtx, finalImage, temporaryPath, codec.SaveOptions{Quality: 95, SRGB: true}); err != nil {
			if runCtx.Err() != nil {
				return nil, newError(CodeCancelled, "已取消")
			}
			return nil, newError(CodeVerifyFailed, "目标缩放写入失败：%v", err)
		}
	}
	if runCtx.Err() != nil {
		return nil, newError(CodeCancelled, "已取消")
	}
	if err := output.Atomic(req.OutputPath, func(file *os.File) error {
		if runCtx.Err() != nil {
			return runCtx.Err()
		}
		input, err := os.Open(temporaryPath)
		if err != nil {
			return err
		}
		defer input.Close()
		_, err = io.Copy(file, input)
		if err == nil {
			err = runCtx.Err()
		}
		return err
	}); err != nil {
		if runCtx.Err() != nil {
			return nil, newError(CodeCancelled, "已取消")
		}
		return nil, newError(CodeVerifyFailed, "提交输出失败：%v", err)
	}
	report := &Report{
		OutputPath:   req.OutputPath,
		Model:        model.Name,
		NativeScale:  model.Scale,
		AppliedScale: scale,
		Width:        inputWidth * scale, Height: inputHeight * scale,
		Elapsed: elapsed,
	}
	if req.Scale > 0 && req.Scale != model.Scale {
		report.Warnings = append(report.Warnings,
			fmt.Sprintf("模型原生倍率为 %d×，已在推理后缩放到 %d×", model.Scale, req.Scale))
	}
	if tile == 0 {
		report.Warnings = append(report.Warnings, "Tile 由引擎自动选择，显存不足时请手动指定更小的值")
	}
	return report, nil
}

// looksLikeOOM recognises the engine's out-of-memory spelling.
func looksLikeOOM(detail string) bool {
	lower := strings.ToLower(detail)
	return strings.Contains(lower, "out of memory") || strings.Contains(lower, "vkallocatememory") ||
		strings.Contains(lower, "vulkan") && strings.Contains(lower, "fail")
}

// Registry is the model catalogue this build knows about.
//
// Every entry names where its weights come from and under which licence. A model
// without a recorded SHA256 lists in the page as "未登记哈希" and cannot run, which
// is how the build refuses to preload an unapproved weight set.
var (
	modelMu  sync.RWMutex
	Registry = defaultRegistry()
)

// defaultRegistry is the shipping catalogue. The hashes are intentionally empty:
// the app ships no weights, and a user who installs a set must record its hash in
// the settings before the page will run it.
func defaultRegistry() []Model {
	return []Model{
		{
			Name: "realesrgan-x4plus", ParamFile: "realesrgan-x4plus.param",
			BinFile: "realesrgan-x4plus.bin", Scale: 4, Purpose: "照片",
			License: "上游项目 BSD-3-Clause；引擎 MIT；权重授权请核对上游",
		},
		{
			Name: "realesrgan-x4plus-anime", ParamFile: "realesrgan-x4plus-anime.param",
			BinFile: "realesrgan-x4plus-anime.bin", Scale: 4, Purpose: "插画 / 动漫",
			License: "上游项目 BSD-3-Clause；引擎 MIT；权重授权请核对上游",
		},
		{
			Name: "realesr-animevideov3", ParamFile: "realesr-animevideov3-x2.param",
			BinFile: "realesr-animevideov3-x2.bin", Scale: 2, Purpose: "动漫视频帧",
			License: "上游项目 BSD-3-Clause；引擎 MIT；权重授权请核对上游",
		},
	}
}

// SetRegistry replaces the catalogue, for a caller that recorded verified hashes.
func SetRegistry(models []Model) {
	modelMu.Lock()
	defer modelMu.Unlock()
	Registry = append([]Model(nil), models...)
}

// RegistrySnapshot copies the catalogue for a reader that is not the UI thread.
func RegistrySnapshot() []Model {
	modelMu.RLock()
	defer modelMu.RUnlock()
	return append([]Model(nil), Registry...)
}

// ErrNoEngine is the sentinel a caller compares against.
var ErrNoEngine = errors.New("没有可用的本地推理引擎")
