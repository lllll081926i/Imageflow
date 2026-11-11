package codec

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func colorBox(p, tr uint16) []byte {
	b := []byte{'n', 'c', 'l', 'x', 0, 0, 0, 0, 0, 9, 128}
	binary.BigEndian.PutUint16(b[4:], p)
	binary.BigEndian.PutUint16(b[6:], tr)
	return makeBMFFBox("colr", b)
}

func TestBMFFPrimaryColorAssociations(t *testing.T) {
	for _, wide := range []bool{false, true} {
		primary := uint32(1)
		pitm := []byte{0, 0, 0, 0, 0, 1}
		ipma := []byte{0, 0, 0, 0, 0, 0, 0, 1, 0, 1, 1, 0x81}
		if wide {
			primary = 70000
			pitm = make([]byte, 8)
			pitm[0] = 1
			binary.BigEndian.PutUint32(pitm[4:], primary)
			ipma = []byte{1, 0, 0, 1, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0x80, 129}
			binary.BigEndian.PutUint32(ipma[8:], primary)
		}
		var properties []byte
		if wide {
			for range 128 {
				properties = append(properties, makeBMFFBox("free", nil)...)
			}
		}
		properties = append(properties, append(colorBox(9, 16), colorBox(1, 13)...)...)
		ipco := makeBMFFBox("ipco", properties)
		iprp := makeBMFFBox("iprp", append(ipco, makeBMFFBox("ipma", ipma)...))
		data := makeBMFFBox("meta", append(append([]byte{0, 0, 0, 0}, makeBMFFBox("pitm", pitm)...), iprp...))
		_, nclx, err := readBMFFColor(bytes.NewReader(data), int64(len(data)))
		if err != nil || len(nclx) != 7 || binary.BigEndian.Uint16(nclx) != 9 || binary.BigEndian.Uint16(nclx[2:]) != 16 {
			t.Fatalf("主图色彩读取错误 wide=%v：%x %v", wide, nclx, err)
		}
	}
}

func TestBMFFDerivedColorAndInvalidAssociations(t *testing.T) {
	for _, test := range []struct {
		name  string
		index byte
		cycle bool
		fail  bool
	}{
		{"继承图块", 1, false, false}, {"索引越界", 3, false, true}, {"循环引用", 0, true, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			pitm := makeBMFFBox("pitm", []byte{0, 0, 0, 0, 0, 1})
			ipma := makeBMFFBox("ipma", []byte{0, 0, 0, 0, 0, 0, 0, 1, 0, 2, 1, test.index})
			iprp := makeBMFFBox("iprp", append(makeBMFFBox("ipco", colorBox(9, 18)), ipma...))
			refs := makeBMFFBox("dimg", []byte{0, 1, 0, 1, 0, 2})
			if test.cycle {
				refs = append(refs, makeBMFFBox("dimg", []byte{0, 2, 0, 1, 0, 1})...)
			}
			iref := makeBMFFBox("iref", append([]byte{0, 0, 0, 0}, refs...))
			data := makeBMFFBox("meta", append(append(append([]byte{0, 0, 0, 0}, pitm...), iprp...), iref...))
			_, nclx, err := readBMFFColor(bytes.NewReader(data), int64(len(data)))
			if test.fail {
				if err == nil {
					t.Fatal("无效关联未拒绝")
				}
				return
			}
			if err != nil || len(nclx) != 7 || binary.BigEndian.Uint16(nclx[2:]) != 18 {
				t.Fatalf("未继承 HLG 图块色彩：%x %v", nclx, err)
			}
		})
	}
}
