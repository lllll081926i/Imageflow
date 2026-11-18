package ui

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lllll081926i/imageflow-go/internal/upscale"
)

func TestUpscalePrimaryActionFollowsReadiness(t *testing.T) {
	app := newTestApp(t)
	tool := &UpscaleTool{models: t.TempDir(), downloadModel: upscale.UpscaylModels()[0].Name}
	check := func(want string, disabled bool) {
		t.Helper()
		label, _, actualDisabled, _ := tool.primaryAction(app)
		if label != want || actualDisabled != disabled {
			t.Fatalf("主操作状态不符：%q，禁用 %t", label, actualDisabled)
		}
	}
	check("正在检测 AI 组件…", true)
	tool.detected = true
	if upscale.ComponentDownloadSupported() {
		check("下载 AI 组件", false)
	} else {
		check("重新检测 AI 组件", false)
	}
	tool.env.Available = true
	tool.env.Models = []upscale.ModelStatus{{Model: upscale.Model{Name: tool.downloadModel}, Installed: true}}
	check("重新检测 AI 组件", false)
	tool.env.Models[0].HashOK = true
	check("选择要放大的图片", false)
	seedInputs(t, app, 1)
	check("放大图片", false)
}

func TestUpscaleCancellationWaitsForComponentWork(t *testing.T) {
	app := newTestApp(t)
	tool := &UpscaleTool{}
	started, release := make(chan struct{}), make(chan struct{})
	defer func() { close(release) }()
	tool.modelDownload.request(app, "download", func(ctx context.Context) (upscale.Model, error) {
		close(started)
		<-ctx.Done()
		<-release
		return upscale.Model{}, ctx.Err()
	})
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("下载任务未启动")
	}
	tool.modelDownload.cancelRequest()
	if _, busy, err := tool.modelDownload.snapshot(); !busy || !errors.Is(err, context.Canceled) {
		t.Fatalf("清理完成前不应该解除互斥：%t，%v", busy, err)
	}
	if label, _, disabled, _ := tool.primaryAction(app); label != "正在取消模型下载…" || !disabled {
		t.Fatalf("取消期间的主操作不正确：%q / %t", label, disabled)
	}
}
