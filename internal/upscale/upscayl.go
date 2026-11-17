package upscale

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const UpscaylCommit = "4f39acfc6f88260d105920a64deff8431d5e1544"
const UpscaylSource = "https://github.com/upscayl/upscayl/tree/" + UpscaylCommit

func UpscaylModels() []Model {
	entries := []struct{ name, purpose, hash string }{
		{"upscayl-standard-4x", "照片 / 通用", "e0adbaf6c011eabe0e70b7da569482465bae42eba7a86ef58e23d0917f5c5edc"},
		{"upscayl-lite-4x", "照片 / 低显存快速处理", "b9e1813ba3b431cbefc7133ad7545fbe7c49b0565a31ae4374d4744f2ec01663"},
		{"hfa2k-4x", "照片 / 保留细节", "04ddd4a430cb2f711a9bd061e6d0efa6c3a1982cccca025b20fcbdb817127d39"},
		{"remacri-4x", "照片 / 纹理细节", "76a1a799e8dcb0250ae9e24e75285fa74954627f086389a9486efeebb6bb5524"},
		{"ultramix-balanced-4x", "照片 / 均衡增强", "4daabd74bda53766e3a6b09e4ee83ed3d17c30848a29478a07dd020438153aff"},
		{"ultrasharp-4x", "照片 / 锐化细节", "bee191e98f26485093466b41ebd06dcb2700e58e26750dbb19e339040a7c9c44"},
		{"digital-art-4x", "插画 / 动漫", "3946c072129fa71e38c2d2f2a63bb4db67d3981622d693de24e59de5e5cf2445"},
	}
	models := make([]Model, 0, len(entries))
	for _, entry := range entries {
		models = append(models, Model{Name: entry.name, ParamFile: entry.name + ".param", BinFile: entry.name + ".bin",
			Scale: 4, SHA256: entry.hash, Purpose: entry.purpose, Backend: "upscayl",
			License: "来源：Upscayl v2.15.0；组件 AGPL-3.0；权重许可见上游模型说明"})
	}
	return models
}

func EnginePathForModel(root string, model Model) string {
	if model.Backend != "upscayl" {
		return ComponentEnginePath(root)
	}
	name := "upscayl-bin"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(root, "upscayl-"+UpscaylCommit[:12], name)
}

type pinnedResource struct {
	path string
	hash string
}

func upscaylEngineResources() ([]pinnedResource, error) {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "windows/amd64":
		return []pinnedResource{
			{"win/bin/upscayl-bin.exe", "704fd622984220c8c646a8dff4c7eba1cc62fbb8c47383996f38571f76b73fbf"},
			{"win/bin/vcomp140.dll", "d49b394de1154176b39611c37c669ebff50aa5a818dbd5ff3d2214a299368ddd"},
			{"win/bin/vcomp140d.dll", "879ff638ac1c12a436ad794fb4bbbaf06ac4b43a980ebe5509f56acde2f3e5ab"},
		}, nil
	case "linux/amd64":
		return []pinnedResource{{"linux/bin/upscayl-bin", "a7aad1d831d1130762101be3cba8cefe2ceb2d1d010cce4d876cfc022226dab4"}}, nil
	case "darwin/amd64", "darwin/arm64":
		return []pinnedResource{{"mac/bin/upscayl-bin", "6cd3485f42e7561305670d7a7134c8364070c8c5b51effb766801f3734ddcd25"}}, nil
	default:
		return nil, fmt.Errorf("Upscayl 暂无当前平台的已校验组件：%s/%s", runtime.GOOS, runtime.GOARCH)
	}
}

func resourceURL(path string, options DownloadOptions) (string, error) {
	address := "https://github.com/upscayl/upscayl/raw/" + UpscaylCommit + "/resources/" + path
	if options.Source == "" || options.Source == "github" {
		return address, nil
	}
	if options.Source != "mirror" {
		return "", fmt.Errorf("未知的模型下载源：%s", options.Source)
	}
	mirror := options.Mirror
	if mirror == "" {
		mirror = DefaultModelMirror
	}
	parsed, err := url.Parse(mirror)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("加速地址必须为不带账户、查询参数或片段的 HTTPS 前缀")
	}
	return strings.TrimRight(mirror, "/") + "/" + address, nil
}

func downloadResource(ctx context.Context, client *http.Client, target string, resource pinnedResource, options DownloadOptions, progress func(int64, int64)) error {
	address, err := resourceURL(resource.path, options)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", "ImageFlow-component-downloader")
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("组件下载失败（未绕过代理）：%w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.ContentLength > maxModelBundleBytes {
		return fmt.Errorf("组件下载响应无效：HTTP %d", response.StatusCode)
	}
	file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o700)
	if err != nil {
		return err
	}
	hash := sha256.New()
	buffer := make([]byte, 64<<10)
	var received int64
	reader := io.LimitReader(contextReader{ctx, response.Body}, maxModelBundleBytes+1)
	for {
		count, readErr := reader.Read(buffer)
		if count > 0 {
			received += int64(count)
			if received > maxModelBundleBytes {
				file.Close()
				return fmt.Errorf("组件文件超过 128 MiB")
			}
			if _, err := file.Write(buffer[:count]); err != nil {
				file.Close()
				return err
			}
			hash.Write(buffer[:count])
			if progress != nil {
				progress(received, response.ContentLength)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			file.Close()
			return readErr
		}
	}
	if err := file.Close(); err != nil {
		return err
	}
	if resource.hash != "" && hex.EncodeToString(hash.Sum(nil)) != resource.hash {
		return fmt.Errorf("组件 SHA256 不匹配：%s", resource.path)
	}
	return ctx.Err()
}

func installPinnedDirectory(ctx context.Context, stage, target string, names []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if info, err := os.Lstat(target); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("组件目录不是普通目录：%s", target)
		}
		for _, name := range names {
			info, err := os.Lstat(filepath.Join(target, name))
			if err != nil || !info.Mode().IsRegular() || info.Size() > maxModelBundleBytes {
				return fmt.Errorf("已有组件无效，未覆盖：%s", name)
			}
		}
		actual, err := dirHashContext(ctx, target, names...)
		if err != nil {
			return err
		}
		expected, err := dirHashContext(ctx, stage, names...)
		if err != nil || actual != expected {
			return fmt.Errorf("已有组件内容不匹配，未覆盖：%s", target)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.Rename(stage, target)
}

func downloadUpscayl(ctx context.Context, root string, model Model, options DownloadOptions, progress func(int64, int64), engine bool) (Model, error) {
	resources, err := upscaylEngineResources()
	if err != nil && engine {
		return model, err
	}
	if strings.TrimSpace(root) == "" {
		return model, fmt.Errorf("模型目录不能为空")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return model, fmt.Errorf("模型目录不可写：%w", err)
	}
	client, err := downloadClient(options)
	if err != nil {
		return model, err
	}
	defer client.CloseIdleConnections()
	stage, err := os.MkdirTemp(root, ".imageflow-upscayl-*")
	if err != nil {
		return model, err
	}
	defer os.RemoveAll(stage)
	engineStage, modelStage := filepath.Join(stage, "engine"), filepath.Join(stage, "model")
	for _, directory := range []string{engineStage, modelStage} {
		if err := os.Mkdir(directory, 0o755); err != nil {
			return model, err
		}
	}
	var names []string
	if engine {
		for _, resource := range resources {
			name := filepath.Base(resource.path)
			if err := downloadResource(ctx, client, filepath.Join(engineStage, name), resource, options, progress); err != nil {
				return model, err
			}
			names = append(names, name)
		}
	}
	modelNames := []string{model.ParamFile, model.BinFile}
	for _, name := range modelNames {
		if err := downloadResource(ctx, client, filepath.Join(modelStage, name), pinnedResource{path: "models/" + name}, options, progress); err != nil {
			return model, err
		}
	}
	actual, err := dirHashContext(ctx, modelStage, modelNames...)
	if err != nil || actual != model.SHA256 {
		return model, fmt.Errorf("模型 SHA256 不匹配，拒绝安装")
	}
	modelInstallMu.Lock()
	defer modelInstallMu.Unlock()
	if engine {
		if err := installPinnedDirectory(ctx, engineStage, filepath.Dir(EnginePathForModel(root, model)), names); err != nil {
			return model, err
		}
	}
	if err := installPinnedDirectory(ctx, modelStage, model.Dir(root), modelNames); err != nil {
		return model, err
	}
	return model, writeTrustedModels(ctx, root, []Model{model})
}
