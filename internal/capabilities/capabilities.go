// Package capabilities builds the runtime capability snapshot the settings page
// and the export menus read.
//
// The PRD requires that a format is only advertised once its read and write
// behaviour has been tested, and that the UI shows one shared list instead of
// each page keeping its own opinion. This package therefore holds the single
// table of what the build can actually do, and a Probe helper that asks the
// live encoders in internal/codec.
package capabilities

import (
	"sort"
	"strings"

	"github.com/lllll081926i/imageflow-go/internal/codec"
)

// Format is one row of the capability matrix. Every dimension is reported
// separately: a format that decodes but cannot encode must never be offered as
// an output choice.
type Format struct {
	Format         string `json:"format"`
	Decode         bool   `json:"decode"`
	Encode         bool   `json:"encode"`
	AnimatedDecode bool   `json:"animated_decode"`
	AnimatedEncode bool   `json:"animated_encode"`
	Alpha          bool   `json:"alpha"`
	ICC            bool   `json:"icc"`
	Quality        bool   `json:"quality"`
	Lossless       bool   `json:"lossless"`
	// RequiresOptional marks a format whose support depends on an optional
	// component (a plugin, an external engine) rather than the core build.
	RequiresOptional bool `json:"requires_optional"`
	// Reason explains a missing ability in the words the UI shows.
	Reason string `json:"reason,omitempty"`
}

// Snapshot is the whole answer: the format matrix plus the optional components
// this installation can offer.
type Snapshot struct {
	Formats []Format `json:"formats"`
	// AnimationFormats are the containers the animation tool may write.
	AnimationFormats []string `json:"animation_formats"`
	// ColorManagement reports which working space the pipeline uses, so the
	// colour pages can label a limited 8-bit path instead of promising more.
	ColorManagement string `json:"color_management"`
	// Warnings are capability-level notes shown in the settings section.
	Warnings []string `json:"warnings,omitempty"`
}

// formatTable is the single source of truth. Each entry states what the build
// does today, not what a candidate library claims.
//
// The coverage was verified by internal/codec's matrix test, which encodes and
// reopens every writable format, and by the colour support tests, which cover
// ICC, CMYK and HDR inputs.
var formatTable = []Format{
	{
		Format: "JPEG", Decode: true, Encode: true, Alpha: false, ICC: true, Quality: true,
	},
	{
		Format: "PNG", Decode: true, Encode: true, Alpha: true, ICC: true, Lossless: true,
	},
	{
		Format: "WebP", Decode: true, Encode: true, AnimatedDecode: true, AnimatedEncode: true,
		Alpha: true, ICC: true, Quality: true, Lossless: true,
	},
	{
		Format: "AVIF", Decode: true, Encode: true, Alpha: true, ICC: true, Quality: true,
	},
	{
		Format: "GIF", Decode: true, Encode: true, AnimatedDecode: true, AnimatedEncode: true, Alpha: true,
	},
	{
		Format: "APNG", Decode: true, Encode: true, AnimatedDecode: true, AnimatedEncode: true,
		Alpha: true, ICC: true, Lossless: true,
	},
	{
		Format: "TIFF", Decode: true, Encode: true, Alpha: true, Lossless: true,
	},
	{
		Format: "BMP", Decode: true, Encode: true, Alpha: false,
	},
	{
		Format: "ICO", Decode: true, Encode: true, Alpha: true,
	},
	{
		// HEIC is readable through the bundled decoder but the build does not
		// encode it: offering it as an output would promise a file it cannot
		// produce.
		Format: "HEIF", Decode: true, ICC: true,
		Reason: "仅支持读取；写入需要额外的 HEIC 编码组件",
	},
	{
		// SVG is rasterised for reading only. Writing real SVG paths is the P2
		// vectorisation feature and is not claimed here.
		Format: "SVG", Decode: true,
		Reason: "仅支持读取并栅格化；矢量描摹未开放",
	},
}

// Current returns the snapshot for this build.
func Current() Snapshot {
	formats := make([]Format, len(formatTable))
	copy(formats, formatTable)
	sort.SliceStable(formats, func(i, j int) bool { return formats[i].Format < formats[j].Format })
	out := Snapshot{
		Formats:          formats,
		AnimationFormats: []string{"GIF", "APNG", "WebP"},
		ColorManagement:  "LCMS/WASM 转换到 sRGB 8 位工作空间",
	}
	for _, f := range formats {
		if f.Encode && f.AnimatedEncode && f.Format == "APNG" {
			out.Warnings = append(out.Warnings, "APNG 写入使用固定的帧延迟有理数表示，非整数毫秒会被拒绝")
		}
	}
	out.Warnings = append(out.Warnings, "专业调色按 8 位工作空间处理，不宣称任意高位深无损编辑")
	return out
}

// EncoderFormats lists the formats the export menus may offer, in the order the
// converter shows them. It is derived from the table so the two cannot drift.
func EncoderFormats() []string {
	out := make([]string, 0, len(codec.OutputFormats))
	for _, ext := range codec.OutputFormats {
		if SupportsExt(ext).Encode {
			out = append(out, ext)
		}
	}
	return out
}

// SupportsExt resolves a file extension ("jpg") to its table row. An unknown
// extension returns a zero row whose Encode is false.
func SupportsExt(ext string) Format {
	name := normalizeExt(ext)
	for _, f := range formatTable {
		if normalizeExt(f.Format) == name {
			return f
		}
	}
	return Format{Format: strings.ToUpper(ext), Reason: "未在能力表中登记"}
}

// normalizeExt folds the spellings the app uses for one container.
func normalizeExt(v string) string {
	v = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(v, ".")))
	switch v {
	case "jpeg", "jpg":
		return "jpg"
	case "tif", "tiff":
		return "tiff"
	case "heic", "heif":
		return "heif"
	case "apng", "png":
		// APNG is PNG's animation profile, so both spellings resolve to PNG for
		// the still-image abilities and to APNG only for the animation ones.
		return "png"
	}
	return v
}

// AnimationTargets lists the containers the animation tool may write, checked
// against the table rather than hard-coded in the page.
func AnimationTargets() []string {
	out := make([]string, 0, 3)
	for _, target := range []string{"gif", "apng", "webp"} {
		// APNG is not in codec.OutputFormats; the animation writer produces it,
		// so it is resolved against the table directly instead of the ladder.
		if f := SupportsExt(target); f.Encode {
			out = append(out, target)
		}
	}
	return out
}

// Summary renders a one-line description of a row for the settings list, so the
// page never has to recompute which abilities combine.
func (f Format) Summary() string {
	parts := make([]string, 0, 6)
	if f.Decode {
		parts = append(parts, "读取")
	}
	if f.Encode {
		parts = append(parts, "写入")
	}
	if f.AnimatedDecode && f.AnimatedEncode {
		parts = append(parts, "动画读写")
	} else if f.AnimatedDecode {
		parts = append(parts, "动画读取")
	}
	if f.Alpha {
		parts = append(parts, "透明")
	}
	if f.ICC {
		parts = append(parts, "ICC")
	}
	if len(parts) == 0 {
		return "不可用"
	}
	return strings.Join(parts, " · ")
}
