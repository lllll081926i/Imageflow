package upscale

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestModelDownloadAddressesAndProxyModes(t *testing.T) {
	ordinary, err := DownloadURL("github", "")
	if err != nil || ordinary != ModelBundleURL {
		t.Fatalf("GitHub 地址错误：%s %v", ordinary, err)
	}
	accelerated, err := DownloadURL("mirror", "")
	if err != nil || accelerated != DefaultModelMirror+ModelBundleURL {
		t.Fatalf("加速地址错误：%s %v", accelerated, err)
	}
	for _, prefix := range []string{"http://mirror.test/", "https://user@mirror.test/", "https://mirror.test/?key=1"} {
		if _, err := DownloadURL("mirror", prefix); err == nil {
			t.Fatalf("接受了无效前缀：%s", prefix)
		}
	}
	request, err := http.NewRequest(http.MethodGet, ModelBundleURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"custom", "direct"} {
		client, err := downloadClient(DownloadOptions{ProxyMode: mode, ProxyURL: "http://127.0.0.1:7891"})
		if err != nil {
			t.Fatal(err)
		}
		transport := client.Transport.(*http.Transport)
		if mode == "direct" {
			if transport.Proxy != nil {
				t.Fatal("直连仍使用代理")
			}
			continue
		}
		proxy, err := transport.Proxy(request)
		if err != nil || proxy.String() != "http://127.0.0.1:7891" {
			t.Fatalf("代理配置错误：%v %v", proxy, err)
		}
	}
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:7892")
	t.Setenv("NO_PROXY", "")
	client, err := downloadClient(DownloadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	proxy, err := client.Transport.(*http.Transport).Proxy(request)
	if err != nil || proxy.String() != "http://127.0.0.1:7892" {
		t.Fatalf("系统模式未采用环境代理：%v %v", proxy, err)
	}
}

func TestModelDownloadUsesProxyAndDoesNotBypassFailure(t *testing.T) {
	var connections atomic.Int64
	proxy := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodConnect {
			connections.Add(1)
		}
		writer.WriteHeader(http.StatusBadGateway)
	}))
	defer proxy.Close()
	_, err := DownloadModel(context.Background(), t.TempDir(), DownloadableModels()[0].Name,
		DownloadOptions{Source: "github", ProxyMode: "custom", ProxyURL: proxy.URL}, nil)
	if err == nil || connections.Load() != 1 {
		t.Fatalf("代理失败未明确返回：%v，连接次数 %d", err, connections.Load())
	}
}

func modelArchiveFixture(t *testing.T, name string) (string, Model) {
	t.Helper()
	model := Model{Name: name, ParamFile: name + ".param", BinFile: name + ".bin", Scale: 4, License: "测试许可证", Purpose: "测试模型"}
	parameter, weights := []byte("测试参数"), []byte("测试权重")
	hash := sha256.Sum256(append(append([]byte{}, parameter...), weights...))
	model.SHA256 = hex.EncodeToString(hash[:])
	path := filepath.Join(t.TempDir(), "models.zip")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	for _, entry := range []struct {
		name string
		data []byte
	}{{"models/" + model.ParamFile, parameter}, {"models/" + model.BinFile, weights}, {"../outside.exe", []byte("不得提取")}, {"realesrgan.exe", []byte("不得提取")}} {
		writer, err := archive.Create(entry.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(entry.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path, model
}

func TestModelArchiveInstallsOnlyVerifiedWeightsAndMergesManifest(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"first", "second"} {
		archive, model := modelArchiveFixture(t, name)
		if err := installModelArchive(context.Background(), archive, root, model); err != nil {
			t.Fatal(err)
		}
		if err := installModelArchive(context.Background(), archive, root, model); err != nil {
			t.Fatal("重复安装相同可信文件失败：", err)
		}
		actual, err := DirHash(model.Dir(root), model.ParamFile, model.BinFile)
		if err != nil || actual != model.SHA256 {
			t.Fatalf("安装模型校验失败：%s %v", actual, err)
		}
	}
	models, err := readManifest(filepath.Join(root, "imageflow-models.json"))
	if err != nil || len(models) != 2 {
		t.Fatalf("清单未合并：%v %v", models, err)
	}
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(path, ".exe") || strings.HasPrefix(entry.Name(), ".imageflow-model-") {
			return fmt.Errorf("不应安装的文件：%s", path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestModelArchiveRejectsWrongHashAndPreservesExistingFiles(t *testing.T) {
	archive, model := modelArchiveFixture(t, "test-model")
	root := t.TempDir()
	invalid := model
	invalid.SHA256 = strings.Repeat("0", 64)
	if err := installModelArchive(context.Background(), archive, root, invalid); err == nil {
		t.Fatal("错误哈希仍被安装")
	}
	if _, err := os.Stat(model.Dir(root)); !os.IsNotExist(err) {
		t.Fatal("错误哈希创建了正式模型目录")
	}
	if err := os.MkdirAll(model.Dir(root), 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(model.Dir(root), model.ParamFile)
	if err := os.WriteFile(path, []byte("保留用户文件"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := installModelArchive(context.Background(), archive, root, model); err == nil {
		t.Fatal("覆盖了用户原有目录")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "保留用户文件" {
		t.Fatal("用户文件被修改")
	}
}

func routeDownloadToTLSFixture(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	transport := server.Client().Transport.(*http.Transport).Clone()
	transport.TLSClientConfig.ServerName = "example.com"
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	previous := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = previous; transport.CloseIdleConnections(); server.Close() })
	return server
}

func TestDownloadRefusesUntrustedPayload(t *testing.T) {
	routeDownloadToTLSFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) { writer.Write([]byte("不是可信压缩包")) }))
	root := t.TempDir()
	_, err := DownloadModel(context.Background(), root, DownloadableModels()[0].Name, DownloadOptions{ProxyMode: "direct"}, nil)
	if err == nil || !strings.Contains(err.Error(), "SHA256") {
		t.Fatalf("未拒绝不可信下载：%v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("不可信下载写入模型目录：%v %v", entries, err)
	}
}

func TestDownloadCancellationStopsHTTPBody(t *testing.T) {
	started := make(chan struct{})
	routeDownloadToTLSFixture(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Write([]byte("partial"))
		writer.(http.Flusher).Flush()
		close(started)
		<-request.Context().Done()
	}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	root := t.TempDir()
	finished := make(chan error, 1)
	go func() {
		_, err := DownloadModel(ctx, root, DownloadableModels()[0].Name, DownloadOptions{ProxyMode: "direct"}, nil)
		finished <- err
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("下载未启动")
	}
	cancel()
	select {
	case err := <-finished:
		if err == nil {
			t.Fatal("取消仍报告成功")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("HTTP 下载未停止")
	}
}

func TestModelDownloadLiveSources(t *testing.T) {
	if os.Getenv("IMAGEFLOW_MODEL_DOWNLOAD_LIVE") != "1" {
		t.Skip("需要显式启用联网下载验证")
	}
	for index, source := range []string{"github", "mirror"} {
		t.Run(source, func(t *testing.T) {
			root := t.TempDir()
			model, err := DownloadModel(context.Background(), root, DownloadableModels()[index].Name, DownloadOptions{Source: source}, nil)
			if err != nil {
				t.Fatal(err)
			}
			actual, err := DirHash(model.Dir(root), model.ParamFile, model.BinFile)
			if err != nil || actual != model.SHA256 {
				t.Fatalf("联网模型校验失败：%s %v", actual, err)
			}
			t.Logf("%s：%s，SHA256=%s", source, model.Name, actual)
		})
	}
}
