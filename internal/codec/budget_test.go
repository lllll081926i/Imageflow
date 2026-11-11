package codec

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHEIFSequenceRejectedBeforeDecode(t *testing.T) {
	for _, brand := range []string{"msf1", "hevc", "hevx"} {
		raw := makeBMFFBox("ftyp", append([]byte(brand+"\x00\x00\x00\x00"), []byte("heic")...))
		if _, err := decodeConfig(bytes.NewReader(raw), "sequence.heic"); err == nil || !strings.Contains(err.Error(), "图像序列") {
			t.Fatalf("%s 序列未在解码前拒绝：%v", brand, err)
		}
	}
}

func TestHDRWithoutICCReadsEncoding(t *testing.T) {
	// BT.2020 原色与 PQ 传递函数不能当作普通 sRGB 像素重新编码。
	colr := makeBMFFBox("colr", []byte{'n', 'c', 'l', 'x', 0, 9, 0, 16, 0, 9, 128})
	raw := makeBMFFBox("meta", append(make([]byte, 4), makeBMFFBox("iprp", makeBMFFBox("ipco", colr))...))
	profile, nclx, err := readBMFFColor(bytes.NewReader(raw), int64(len(raw)))
	if err != nil || len(profile) != 0 || len(nclx) != 7 || binary.BigEndian.Uint16(nclx) != 9 || binary.BigEndian.Uint16(nclx[2:]) != 16 {
		t.Fatalf("未读取 HDR 色彩描述：%v %x", err, nclx)
	}
}

func TestOversizedHeaderRejectedBeforeDecode(t *testing.T) {
	header := make([]byte, 13)
	binary.BigEndian.PutUint32(header, 20000)
	binary.BigEndian.PutUint32(header[4:], 20000)
	header[8], header[9] = 8, 6
	raw := append([]byte("\x89PNG\r\n\x1a\n"), metadataPNGChunk("IHDR", header)...)
	// IHDR 是有效的；缺少像素数据，用于确认尺寸检查先于图像解码。
	if crc32.ChecksumIEEE(raw[12:29]) != binary.BigEndian.Uint32(raw[29:33]) {
		t.Fatal("测试头部校验错误")
	}
	path := filepath.Join(t.TempDir(), "huge.png")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := Probe(path); err == nil || !strings.Contains(err.Error(), "超出") {
		t.Fatalf("探测未拦截巨大尺寸：%v", err)
	}
	if _, err := Open(path); err == nil || !strings.Contains(err.Error(), "超出") {
		t.Fatalf("解码未拦截巨大尺寸：%v", err)
	}
}
