package utils

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/imageflow/backend/models"
	"github.com/kanrichan/resvg-go"
)

func isSVG(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".svg")
}

func RasterizeSVGToTempPNG(req models.ConvertRequest) (string, func(), error) {
	if !isSVG(req.InputPath) {
		return "", func() {}, fmt.Errorf("not an svg input: %s", req.InputPath)
	}

	data, err := os.ReadFile(req.InputPath)
	if err != nil {
		return "", func() {}, err
	}

	baseW, baseH := parseSVGIntrinsicSize(data)
	if baseW <= 0 || baseH <= 0 {
		baseW, baseH = 1024, 1024
	}

	targetW, targetH := computeTargetSize(baseW, baseH, req)

	ctx, err := resvg.NewContext(context.Background())
	if err != nil {
		return "", func() {}, err
	}
	defer func() { _ = ctx.Close() }()

	renderer, err := ctx.NewRenderer()
	if err != nil {
		return "", func() {}, err
	}
	defer func() { _ = renderer.Close() }()

	pngBytes, err := renderer.RenderWithSize(data, uint32(targetW), uint32(targetH))
	if err != nil {
		return "", func() {}, err
	}

	outPath := filepath.Join(os.TempDir(), fmt.Sprintf("imageflow-svg-%s.png", uuid.NewString()))
	if err := os.WriteFile(outPath, pngBytes, 0o644); err != nil {
		_ = os.Remove(outPath)
		return "", func() {}, err
	}

	cleanup := func() {
		_ = os.Remove(outPath)
	}
	return outPath, cleanup, nil
}

var svgNumber = regexp.MustCompile(`^\s*([0-9]*\.?[0-9]+)`)

func parseSVGNumber(v string) float64 {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	m := svgNumber.FindStringSubmatch(v)
	if len(m) != 2 {
		return 0
	}
	f, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0
	}
	return f
}

func parseSVGIntrinsicSize(data []byte) (int, int) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := decoder.Token()
		if err != nil {
			return 0, 0
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if !strings.EqualFold(start.Name.Local, "svg") {
			continue
		}

		var w, h float64
		var viewBox string
		for _, a := range start.Attr {
			switch strings.ToLower(a.Name.Local) {
			case "width":
				w = parseSVGNumber(a.Value)
			case "height":
				h = parseSVGNumber(a.Value)
			case "viewbox":
				viewBox = a.Value
			}
		}

		if w > 0 && h > 0 {
			return int(w + 0.5), int(h + 0.5)
		}

		if viewBox != "" {
			parts := strings.FieldsFunc(strings.TrimSpace(viewBox), func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r' })
			if len(parts) == 4 {
				vbw, err1 := strconv.ParseFloat(parts[2], 64)
				vbh, err2 := strconv.ParseFloat(parts[3], 64)
				if err1 == nil && err2 == nil && vbw > 0 && vbh > 0 {
					return int(vbw + 0.5), int(vbh + 0.5)
				}
			}
		}

		return 0, 0
	}
}

func computeTargetSize(baseW, baseH int, req models.ConvertRequest) (int, int) {
	targetW, targetH := baseW, baseH
	mode := strings.ToLower(strings.TrimSpace(req.ResizeMode))

	if mode == "percent" && req.ScalePercent > 0 {
		pct := req.ScalePercent
		if pct < 1 {
			pct = 1
		}
		targetW = maxInt(1, int(float64(baseW)*float64(pct)/100.0))
		targetH = maxInt(1, int(float64(baseH)*float64(pct)/100.0))
	} else if mode == "long_edge" && req.LongEdge > 0 {
		le := req.LongEdge
		if le < 1 {
			le = 1
		}
		scale := float64(le) / float64(maxInt(baseW, baseH))
		targetW = maxInt(1, int(float64(baseW)*scale))
		targetH = maxInt(1, int(float64(baseH)*scale))
	} else if mode == "fixed" && (req.Width > 0 || req.Height > 0) {
		w := req.Width
		h := req.Height
		if req.MaintainAR {
			if w > 0 && h == 0 {
				scale := float64(w) / float64(baseW)
				targetW = w
				targetH = maxInt(1, int(float64(baseH)*scale))
			} else if h > 0 && w == 0 {
				scale := float64(h) / float64(baseH)
				targetH = h
				targetW = maxInt(1, int(float64(baseW)*scale))
			} else if w > 0 && h > 0 {
				scale := minFloat(float64(w)/float64(baseW), float64(h)/float64(baseH))
				targetW = maxInt(1, int(float64(baseW)*scale))
				targetH = maxInt(1, int(float64(baseH)*scale))
			}
		} else {
			if w > 0 {
				targetW = w
			}
			if h > 0 {
				targetH = h
			}
		}
	} else if req.Width > 0 || req.Height > 0 {
		w := req.Width
		h := req.Height
		if req.MaintainAR {
			if w > 0 && h == 0 {
				scale := float64(w) / float64(baseW)
				targetW = w
				targetH = maxInt(1, int(float64(baseH)*scale))
			} else if h > 0 && w == 0 {
				scale := float64(h) / float64(baseH)
				targetH = h
				targetW = maxInt(1, int(float64(baseW)*scale))
			} else if w > 0 && h > 0 {
				scale := minFloat(float64(w)/float64(baseW), float64(h)/float64(baseH))
				targetW = maxInt(1, int(float64(baseW)*scale))
				targetH = maxInt(1, int(float64(baseH)*scale))
			}
		} else {
			if w > 0 {
				targetW = w
			}
			if h > 0 {
				targetH = h
			}
		}
	}

	if strings.EqualFold(strings.TrimSpace(req.Format), "ico") && len(req.ICOSizes) > 0 {
		maxSize := 0
		for _, s := range req.ICOSizes {
			if s > maxSize {
				maxSize = s
			}
		}
		edge := maxInt(maxInt(targetW, targetH), maxSize)
		targetW, targetH = edge, edge
	}

	return maxInt(1, targetW), maxInt(1, targetH)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
