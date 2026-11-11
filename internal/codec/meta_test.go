package codec_test

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/lllll081926i/imageflow-go/internal/codec"
)

// jpegWithEXIF builds a JPEG whose EXIF block records a non-upright orientation,
// which is what a phone writes when it turns the sensor.
func jpegWithEXIF(t *testing.T, dir string, orientation uint16) string {
	t.Helper()
	src := filepath.Join(dir, "src.jpg")
	img := image.NewNRGBA(image.Rect(0, 0, 40, 20))
	for y := 0; y < 20; y++ {
		for x := 0; x < 40; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 6), G: uint8(y * 12), B: 90, A: 255})
		}
	}
	if err := codec.Save(img, src, codec.SaveOptions{Quality: 92}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	segment := exifSegment(t, orientation)
	out := make([]byte, 0, len(raw)+len(segment))
	out = append(out, raw[:2]...)
	out = append(out, segment...)
	out = append(out, raw[2:]...)
	if err := os.WriteFile(src, out, 0o644); err != nil {
		t.Fatal(err)
	}
	return src
}

// exifSegment builds a complete APP1 segment carrying one orientation tag.
func exifSegment(t *testing.T, orientation uint16) []byte {
	t.Helper()
	// Little-endian TIFF: header, one IFD entry, next-IFD offset.
	body := &bytes.Buffer{}
	body.WriteString("Exif\x00\x00")
	body.WriteString("II")
	var buf [4]byte
	binary.LittleEndian.PutUint16(buf[:2], 42)
	body.Write(buf[:2])
	binary.LittleEndian.PutUint32(buf[:4], 8) // first IFD offset
	body.Write(buf[:4])
	binary.LittleEndian.PutUint16(buf[:2], 1) // one entry
	body.Write(buf[:2])
	binary.LittleEndian.PutUint16(buf[:2], 0x0112)
	body.Write(buf[:2])
	binary.LittleEndian.PutUint16(buf[:2], 3) // SHORT
	body.Write(buf[:2])
	binary.LittleEndian.PutUint32(buf[:4], 1) // count
	body.Write(buf[:4])
	// A SHORT value lives in the first two bytes of the 4-byte value field.
	// Writing orientation into buf[:2] and then zeroing the same two bytes
	// used to leave every fixture at orientation 0, so the "6 → 1" reset was
	// never actually tested.
	binary.LittleEndian.PutUint16(buf[:2], orientation)
	binary.LittleEndian.PutUint16(buf[2:4], 0)
	body.Write(buf[:4])
	binary.LittleEndian.PutUint32(buf[:4], 0) // no next IFD
	body.Write(buf[:4])

	payload := body.Bytes()
	segment := &bytes.Buffer{}
	segment.Write([]byte{0xFF, 0xE1})
	var length [2]byte
	binary.BigEndian.PutUint16(length[:], uint16(len(payload)+2))
	segment.Write(length[:])
	segment.Write(payload)
	if segment.Len()+4 > 0xFFFF {
		t.Fatal("exif segment too large")
	}
	return segment.Bytes()
}

// TestKeepMetadataCopiesEXIF pins the converter's "保留 EXIF 元数据" switch: the
// segments must reach the output, and the orientation tag must be reset because
// the decoder already rotated the pixels.
func TestKeepMetadataCopiesEXIF(t *testing.T) {
	dir := t.TempDir()
	src := jpegWithEXIF(t, dir, 6)
	// Guard the fixture itself: if the helper ever stops writing orientation 6,
	// the assertion below would pass vacuously (the converter writes 1 anyway).
	srcRaw, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if got := exifOrientation(t, srcRaw); got != 6 {
		t.Fatalf("fixture orientation = %d, want 6", got)
	}

	kept := filepath.Join(dir, "kept.jpg")
	if err := codec.Save(mustOpen(t, src), kept, codec.SaveOptions{
		Quality: 92, KeepMetadata: true, MetadataSource: src,
	}); err != nil {
		t.Fatal(err)
	}
	dropped := filepath.Join(dir, "dropped.jpg")
	if err := codec.Save(mustOpen(t, src), dropped, codec.SaveOptions{Quality: 92}); err != nil {
		t.Fatal(err)
	}

	keptRaw, err := os.ReadFile(kept)
	if err != nil {
		t.Fatal(err)
	}
	droppedRaw, err := os.ReadFile(dropped)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(keptRaw, []byte("Exif\x00\x00")) {
		t.Fatal("kept output carries no EXIF segment")
	}
	if bytes.Contains(droppedRaw, []byte("Exif\x00\x00")) {
		t.Fatal("default output must not carry EXIF")
	}
	if got := exifOrientation(t, keptRaw); got != 1 {
		t.Errorf("orientation = %d, want 1 (pixels are already rotated)", got)
	}
}

// TestMetadataSkipsNonJPEG pins that a source without an EXIF block does not fail
// the conversion: the caller's switch is a hint, not a requirement.
func TestMetadataSkipsNonJPEG(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.png")
	img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	if err := codec.Save(img, src, codec.SaveOptions{}); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out.jpg")
	if err := codec.Save(mustOpen(t, src), out, codec.SaveOptions{
		Quality: 90, KeepMetadata: true, MetadataSource: src,
	}); err != nil {
		t.Fatalf("keep-metadata from a PNG must still write a JPEG: %v", err)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(raw, []byte{0xFF, 0xD8}) {
		t.Fatal("output is not a JPEG")
	}
}

func mustOpen(t *testing.T, path string) image.Image {
	t.Helper()
	opened, err := codec.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return opened.Image
}

// exifOrientation reads the orientation tag back out of an encoded JPEG.
func exifOrientation(t *testing.T, raw []byte) uint16 {
	t.Helper()
	i := bytes.Index(raw, []byte("Exif\x00\x00"))
	if i < 0 {
		t.Fatal("no EXIF")
	}
	tiff := raw[i+6:]
	if len(tiff) < 8 || string(tiff[:2]) != "II" {
		t.Fatalf("unexpected TIFF header %q", tiff[:2])
	}
	ifd := int(binary.LittleEndian.Uint32(tiff[4:8]))
	count := int(binary.LittleEndian.Uint16(tiff[ifd : ifd+2]))
	for k := 0; k < count; k++ {
		entry := ifd + 2 + k*12
		if binary.LittleEndian.Uint16(tiff[entry:entry+2]) == 0x0112 {
			return binary.LittleEndian.Uint16(tiff[entry+8 : entry+10])
		}
	}
	t.Fatal("orientation tag missing")
	return 0
}
