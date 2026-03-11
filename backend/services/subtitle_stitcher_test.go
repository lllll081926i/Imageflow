package services

import (
	"testing"

	"github.com/imageflow/backend/models"
)

func TestSubtitleStitchService_PassesPayload(t *testing.T) {
	logger := newTestLogger(t)
	defer logger.Close()

	runner := &mockPythonRunner{
		executeAndParseFn: func(scriptName string, input interface{}, result interface{}) error {
			if scriptName != "subtitle_stitcher.py" {
				t.Fatalf("expected script subtitle_stitcher.py, got %s", scriptName)
			}
			payload, ok := input.(map[string]interface{})
			if !ok {
				t.Fatalf("expected payload map, got %T", input)
			}
			if payload["action"] != "subtitle_stitch" {
				t.Fatalf("expected action subtitle_stitch, got %#v", payload["action"])
			}
			if payload["output_path"] != "out.png" {
				t.Fatalf("expected output_path out.png, got %#v", payload["output_path"])
			}
			if payload["subtitle_crop_ratio"] != 0.22 {
				t.Fatalf("expected subtitle_crop_ratio 0.22, got %#v", payload["subtitle_crop_ratio"])
			}
			if payload["dedup_enabled"] != true {
				t.Fatalf("expected dedup_enabled true, got %#v", payload["dedup_enabled"])
			}
			if payload["dedup_threshold"] != 2 {
				t.Fatalf("expected dedup_threshold 2, got %#v", payload["dedup_threshold"])
			}
			res := result.(*models.SubtitleStitchResult)
			*res = models.SubtitleStitchResult{Success: true, OutputPath: "out.png", KeptCount: 3}
			return nil
		},
	}

	service := NewSubtitleStitchService(runner, logger)
	res, err := service.Generate(models.SubtitleStitchRequest{
		InputPaths:        []string{"a.png", "b.png", "c.png"},
		OutputPath:        "out.png",
		SubtitleCropRatio: 0.22,
		HeaderKeepFull:    true,
		DedupEnabled:      true,
		DedupThreshold:    2,
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !res.Success || res.OutputPath != "out.png" {
		t.Fatalf("unexpected result: %+v", res)
	}
}
