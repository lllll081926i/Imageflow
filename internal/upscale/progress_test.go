package upscale

import (
	"strings"
	"testing"
)

func TestInferenceProgressParsesPartialLinesAndBoundsLogs(t *testing.T) {
	var percent int
	log := &inferenceLog{notify: func(value int, line string) {
		if value >= 0 {
			percent = value
		}
	}}
	log.Write([]byte("GPU device\n25."))
	log.Write([]byte("00%\r\n"))
	if percent != 25 {
		t.Fatalf("进度未解析：%d", percent)
	}
	log.Write([]byte(strings.Repeat("a", 100000)))
	if len(log.tail) > 16<<10 || len(log.pending) > 4096 {
		t.Fatal("日志未限制内存")
	}
}

func TestGPUIdsValidation(t *testing.T) {
	for _, value := range []string{"", "auto", "AUTO", "自动", "-1", "0", "0,1", "0, 2"} {
		if _, err := validateGPUIds(value); err != nil {
			t.Fatalf("有效 GPU ID 被拒绝：%s %v", value, err)
		}
	}
	for _, value := range []string{"-2", "0,0", "0,", "abc", "32", "0;1"} {
		if _, err := validateGPUIds(value); err == nil {
			t.Fatalf("无效 GPU ID 未被拒绝：%s", value)
		}
	}
}
