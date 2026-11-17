package capabilities

import "testing"

// TestSnapshotCoversEveryWritableFormat pins the one invariant the settings page
// depends on: every extension the converter offers has an Encode row, and every
// row without Encode carries a reason instead of silently disappearing.
func TestSnapshotCoversEveryWritableFormat(t *testing.T) {
	for _, f := range Current().Formats {
		if !f.Encode && f.Reason == "" {
			t.Errorf("%s: read-only format must explain why", f.Format)
		}
		if f.Encode && f.Reason != "" {
			t.Errorf("%s: writable format must not carry an unavailability reason", f.Format)
		}
	}
}

func TestEncoderFormatsAreAllWritable(t *testing.T) {
	list := EncoderFormats()
	if len(list) == 0 {
		t.Fatal("no encoder formats")
	}
	for _, ext := range list {
		if !SupportsExt(ext).Encode {
			t.Errorf("%s offered as output but not writable", ext)
		}
	}
}

func TestSupportsExtFoldsSpellings(t *testing.T) {
	cases := map[string]string{
		"jpeg": "jpg", "JPG": "jpg", ".jpg": "jpg",
		"tif": "tiff", "TIFF": "tiff",
		"heic": "heif",
	}
	for in, want := range cases {
		if got := normalizeExt(in); got != want {
			t.Errorf("normalizeExt(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAlphaAndAnimationAreReportedSeparately(t *testing.T) {
	gif := SupportsExt("gif")
	if !gif.AnimatedEncode {
		t.Error("GIF must report animated encode")
	}
	jpg := SupportsExt("jpg")
	if jpg.AnimatedEncode || jpg.Alpha {
		t.Errorf("JPEG must not claim animation or alpha: %+v", jpg)
	}
}

func TestAnimationTargetsStayInsideTheTable(t *testing.T) {
	got := AnimationTargets()
	want := []string{"gif", "apng", "webp"}
	if len(got) != len(want) {
		t.Fatalf("targets = %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("targets = %v", got)
		}
	}
}

func TestSummaryNamesTheAbilities(t *testing.T) {
	if got := SupportsExt("webp").Summary(); got == "不可用" {
		t.Fatalf("WebP summary = %q", got)
	}
	if got := (Format{Format: "X"}).Summary(); got != "不可用" {
		t.Fatalf("empty row summary = %q", got)
	}
}
