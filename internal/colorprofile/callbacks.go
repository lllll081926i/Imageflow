package colorprofile

import (
	"encoding/binary"
	"fmt"
)

// callbackModule 为 Emscripten 的 invoke 导入添加调用同一函数表的导出桥。
// WASM 原文件保持不变；仅处理随程序内置的模块。
func callbackModule(wasm []byte) ([]byte, error) {
	type section struct {
		id   byte
		data []byte
	}
	read := func(data []byte, pos *int) uint32 {
		v, n := binary.Uvarint(data[*pos:])
		if n < 1 || v > 0xffffffff {
			panic("无效的内置 WASM 整数")
		}
		*pos += n
		return uint32(v)
	}
	put := func(v uint32) []byte { return binary.AppendUvarint(nil, uint64(v)) }
	var sections []section
	for p := 8; p < len(wasm); {
		id := wasm[p]
		p++
		n := int(read(wasm, &p))
		if n > len(wasm)-p {
			return nil, fmt.Errorf("内置 WASM 已截断")
		}
		sections = append(sections, section{id, wasm[p : p+n]})
		p += n
	}
	find := func(id byte) *section {
		for i := range sections {
			if sections[i].id == id {
				return &sections[i]
			}
		}
		return nil
	}
	imports := find(2)
	if imports == nil {
		return nil, fmt.Errorf("内置 WASM 缺少导入")
	}
	p := 0
	importCount := read(imports.data, &p)
	for i := uint32(0); i < importCount; i++ {
		for j := 0; j < 2; j++ {
			n := int(read(imports.data, &p))
			p += n
		}
		if imports.data[p] != 0 {
			return nil, fmt.Errorf("内置 WASM 包含非函数导入")
		}
		p++
		read(imports.data, &p)
	}
	types, funcs, exports, code := find(1), find(3), find(7), find(10)
	if types == nil || funcs == nil || exports == nil || code == nil {
		return nil, fmt.Errorf("内置 WASM 缺少函数段")
	}
	split := func(s *section) (uint32, []byte) {
		p := 0
		n := read(s.data, &p)
		return n, append([]byte(nil), s.data[p:]...)
	}
	nt, tb := split(types)
	nf, fb := split(funcs)
	ne, eb := split(exports)
	nc, cb := split(code)
	for _, signature := range []string{"v", "vi", "vii", "viii", "viiii", "ii", "iii"} {
		params := len(signature)
		for _, count := range []int{params, params - 1} {
			tb = append(tb, 0x60)
			tb = append(tb, put(uint32(count))...)
			for j := 0; j < count; j++ {
				tb = append(tb, 0x7f)
			}
			if signature[0] == 'v' {
				tb = append(tb, 0)
			} else {
				tb = append(tb, 1, 0x7f)
			}
		}
		fb = append(fb, put(nt)...)
		name := "imageflow_invoke_" + signature
		eb = append(eb, put(uint32(len(name)))...)
		eb = append(eb, name...)
		eb = append(eb, 0)
		eb = append(eb, put(importCount+nf)...)
		body := []byte{0}
		for j := 1; j < params; j++ {
			body = append(body, 0x20)
			body = append(body, put(uint32(j))...)
		}
		body = append(body, 0x20, 0, 0x11)
		body = append(body, put(nt+1)...)
		body = append(body, 0, 0x0b)
		cb = append(cb, put(uint32(len(body)))...)
		cb = append(cb, body...)
		nt += 2
		nf++
		ne++
		nc++
	}
	types.data = append(put(nt), tb...)
	funcs.data = append(put(nf), fb...)
	exports.data = append(put(ne), eb...)
	code.data = append(put(nc), cb...)
	out := append([]byte(nil), wasm[:8]...)
	for _, s := range sections {
		out = append(out, s.id)
		out = append(out, put(uint32(len(s.data)))...)
		out = append(out, s.data...)
	}
	return out, nil
}
