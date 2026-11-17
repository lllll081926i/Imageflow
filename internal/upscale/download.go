package upscale

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/net/http/httpproxy"
)

const ModelBundleURL = "https://github.com/xinntao/Real-ESRGAN/releases/download/v0.2.5.0/realesrgan-ncnn-vulkan-20220424-windows.zip"
const ModelBundleSHA256 = "abc02804e17982a3be33675e4d471e91ea374e65b70167abc09e31acb412802d"
const DefaultModelMirror = "https://ghfast.top/"
const DefaultModelProxy = "http://127.0.0.1:7890"
const maxModelBundleBytes int64 = 128 << 20

type DownloadOptions struct {
	Source    string
	Mirror    string
	ProxyMode string
	ProxyURL  string
}

func DownloadURL(source, mirror string) (string, error) {
	switch source {
	case "", "github":
		return ModelBundleURL, nil
	case "mirror":
		if mirror == "" {
			mirror = DefaultModelMirror
		}
		parsed, err := url.Parse(mirror)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return "", fmt.Errorf("加速地址必须为不带账户、查询参数或片段的 HTTPS 前缀")
		}
		return strings.TrimRight(mirror, "/") + "/" + ModelBundleURL, nil
	default:
		return "", fmt.Errorf("未知的模型下载源：%s", source)
	}
}

func parseProxy(value string) (*url.URL, error) {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.Opaque != "" || parsed.Path != "" && parsed.Path != "/" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("代理地址无效")
	}
	switch parsed.Scheme {
	case "http", "https", "socks5", "socks5h":
	default:
		return nil, fmt.Errorf("代理必须使用 http、https 或 socks5 协议")
	}
	return parsed, nil
}

func downloadClient(options DownloadOptions) (*http.Client, error) {
	mode := options.ProxyMode
	if mode == "" {
		mode = "system"
	}
	endpoint := options.ProxyURL
	if endpoint == "" {
		endpoint = DefaultModelProxy
	}
	fallback, err := parseProxy(endpoint)
	if err != nil && mode != "direct" {
		return nil, err
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	switch mode {
	case "direct":
		transport.Proxy = nil
	case "custom":
		transport.Proxy = http.ProxyURL(fallback)
	case "system":
		environment := httpproxy.FromEnvironment().ProxyFunc()
		transport.Proxy = func(request *http.Request) (*url.URL, error) {
			if configured, err := environment(request.URL); err != nil || configured != nil {
				return configured, err
			}
			if configured, err := platformProxy(request.URL); err != nil || configured != nil {
				return configured, err
			}
			return fallback, nil
		}
	default:
		return nil, fmt.Errorf("未知代理模式：%s", mode)
	}
	return &http.Client{Transport: transport, Timeout: 15 * time.Minute,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("下载重定向次数过多")
			}
			if request.URL.Scheme != "https" {
				return fmt.Errorf("拒绝下载跳转到非 HTTPS 地址")
			}
			return nil
		}}, nil
}

func DownloadableModels() []Model {
	models := defaultRegistry()
	hashes := []string{
		"e0adbaf6c011eabe0e70b7da569482465bae42eba7a86ef58e23d0917f5c5edc",
		"3946c072129fa71e38c2d2f2a63bb4db67d3981622d693de24e59de5e5cf2445",
		"ea8b9ba4d93a0cc2bba7bacc9c4e3a20c618a76917a5560cbc1360c16e212280",
	}
	for index := range models {
		models[index].SHA256 = hashes[index]
	}
	return models
}

func DownloadModel(ctx context.Context, root, name string, options DownloadOptions, progress func(int64, int64)) (Model, error) {
	var model Model
	for _, candidate := range DownloadableModels() {
		if candidate.Name == name {
			model = candidate
			break
		}
	}
	if model.Name == "" {
		return model, fmt.Errorf("该模型没有内置的可信下载记录：%s", name)
	}
	if strings.TrimSpace(root) == "" {
		return model, fmt.Errorf("先选择模型目录")
	}
	address, err := DownloadURL(options.Source, options.Mirror)
	if err != nil {
		return model, err
	}
	client, err := downloadClient(options)
	if err != nil {
		return model, err
	}
	defer client.CloseIdleConnections()
	if err := ctx.Err(); err != nil {
		return model, err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return model, fmt.Errorf("模型目录不可写：%w", err)
	}
	archive, err := os.CreateTemp(root, ".imageflow-model-bundle-*.zip")
	if err != nil {
		return model, fmt.Errorf("模型目录不可写：%w", err)
	}
	archivePath := archive.Name()
	defer os.Remove(archivePath)
	defer archive.Close()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return model, err
	}
	request.Header.Set("User-Agent", "ImageFlow-model-downloader")
	response, err := client.Do(request)
	if err != nil {
		return model, fmt.Errorf("模型下载失败（未自动绕过代理）：%w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return model, fmt.Errorf("模型下载 HTTP %d", response.StatusCode)
	}
	if response.ContentLength > maxModelBundleBytes {
		return model, fmt.Errorf("模型压缩包超过 128 MiB")
	}
	hash := sha256.New()
	reader := io.LimitReader(contextReader{ctx, response.Body}, maxModelBundleBytes+1)
	buffer := make([]byte, 64<<10)
	received := int64(0)
	lastNotice := time.Time{}
	for {
		count, readErr := reader.Read(buffer)
		if count > 0 {
			received += int64(count)
			if received > maxModelBundleBytes {
				archive.Close()
				return model, fmt.Errorf("模型压缩包超过 128 MiB")
			}
			if _, err := archive.Write(buffer[:count]); err != nil {
				archive.Close()
				return model, err
			}
			hash.Write(buffer[:count])
			if progress != nil && time.Since(lastNotice) >= 100*time.Millisecond {
				progress(received, response.ContentLength)
				lastNotice = time.Now()
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			archive.Close()
			return model, readErr
		}
	}
	if err := archive.Close(); err != nil {
		return model, err
	}
	if !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), ModelBundleSHA256) {
		return model, fmt.Errorf("模型压缩包 SHA256 不匹配，拒绝安装")
	}
	if progress != nil {
		progress(received, response.ContentLength)
	}
	return model, installModelArchive(ctx, archivePath, root, model)
}

func installModelArchive(ctx context.Context, archivePath, root string, model Model) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer archive.Close()
	stage, err := os.MkdirTemp(root, ".imageflow-model-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	for _, name := range []string{model.ParamFile, model.BinFile} {
		var entry *zip.File
		for _, candidate := range archive.File {
			if candidate.Name == "models/"+name {
				if entry != nil {
					return fmt.Errorf("压缩包模型文件重复：%s", name)
				}
				entry = candidate
			}
		}
		if entry == nil || entry.UncompressedSize64 > uint64(maxModelBundleBytes) {
			return fmt.Errorf("压缩包模型文件缺失或超限：%s", name)
		}
		input, err := entry.Open()
		if err != nil {
			return err
		}
		file, err := os.OpenFile(filepath.Join(stage, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			input.Close()
			return err
		}
		count, copyErr := io.Copy(file, io.LimitReader(contextReader{ctx, input}, maxModelBundleBytes+1))
		input.Close()
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if count > maxModelBundleBytes {
			return fmt.Errorf("模型文件超过 128 MiB")
		}
	}
	actual, err := dirHashContext(ctx, stage, model.ParamFile, model.BinFile)
	if err != nil {
		return err
	}
	if !strings.EqualFold(actual, model.SHA256) {
		return fmt.Errorf("模型文件 SHA256 不匹配，拒绝安装")
	}
	modelInstallMu.Lock()
	defer modelInstallMu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	target := model.Dir(root)
	if _, err := os.Lstat(target); err == nil {
		actual, err := dirHashContext(ctx, target, model.ParamFile, model.BinFile)
		if err != nil || !strings.EqualFold(actual, model.SHA256) {
			return fmt.Errorf("模型目录已存在且内容不匹配，未覆盖：%s", target)
		}
	} else if !os.IsNotExist(err) {
		return err
	} else if err := os.Rename(stage, target); err != nil {
		return err
	}
	return writeTrustedModels(ctx, root, []Model{model})
}
