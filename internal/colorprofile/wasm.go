package colorprofile

import (
	"context"
	_ "embed"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

//go:embed wasm/lcms.wasm
var lcmsWasm []byte

var (
	initializeOnce sync.Once
	lcmsRuntime    wazero.Runtime
	lcmsCompiled   wazero.CompiledModule
	lcmsError      error
	lcmsSlots      = make(chan struct{}, 4)
)

func initializeLCMS() {
	ctx := context.Background()
	lcmsRuntime = wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().WithMemoryLimitPages(2048).WithCloseOnContextDone(true))
	data, err := callbackModule(lcmsWasm)
	if err != nil {
		lcmsError = err
		return
	}
	lcmsCompiled, lcmsError = lcmsRuntime.CompileModule(ctx, data)
	if lcmsError != nil {
		return
	}
	builder := lcmsRuntime.NewHostModuleBuilder("env")
	wasi := lcmsRuntime.NewHostModuleBuilder("wasi_snapshot_preview1")
	for _, definition := range lcmsCompiled.ImportedFunctions() {
		module, name, _ := definition.Import()
		if module != "env" && module != "wasi_snapshot_preview1" {
			continue
		}
		target := builder
		if module == "wasi_snapshot_preview1" {
			target = wasi
		}
		target.NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
			switch {
			case strings.HasPrefix(name, "invoke_"):
				args := stack[:len(definition.ParamTypes())]
				result, err := m.ExportedFunction("imageflow_"+name).Call(ctx, args...)
				if err != nil {
					panic(err)
				}
				copy(stack, result)
			case name == "emscripten_memcpy_js":
				data, ok := m.Memory().Read(uint32(stack[1]), uint32(stack[2]))
				if !ok || !m.Memory().Write(uint32(stack[0]), data) {
					panic("WASM 内存复制越界")
				}
			case name == "emscripten_date_now":
				stack[0] = math.Float64bits(float64(time.Now().UnixMilli()))
			case name == "emscripten_resize_heap":
				pages := (stack[0] + 65535) / 65536
				current, _ := m.Memory().Grow(0)
				ok := true
				if pages > uint64(current) {
					_, ok = m.Memory().Grow(uint32(pages) - current)
				}
				stack[0] = 0
				if ok {
					stack[0] = 1
				}
			case strings.HasPrefix(name, "__syscall_"):
				// 模块没有宿主文件系统权限，返回 POSIX ENOSYS。
				stack[0] = uint64(uint32(0xffffffda))
			case name == "fd_read" || name == "fd_close":
				stack[0] = 8
			case name == "fd_seek":
				stack[0] = 29
			case name == "fd_write":
				if stack[0] != 1 && stack[0] != 2 {
					stack[0] = 8
					return
				}
				total := uint32(0)
				for i := uint32(0); i < uint32(stack[2]); i++ {
					n, ok := m.Memory().ReadUint32Le(uint32(stack[1]) + i*8 + 4)
					if !ok {
						panic("WASM 日志缓冲区越界")
					}
					total += n
				}
				if !m.Memory().WriteUint32Le(uint32(stack[3]), total) {
					panic("WASM 日志计数越界")
				}
				stack[0] = 0
			case name == "_tzset_js":
				m.Memory().WriteUint32Le(uint32(stack[0]), 0)
				m.Memory().WriteUint32Le(uint32(stack[1]), 0)
				m.Memory().Write(uint32(stack[2]), []byte("UTC\x00"))
			case name == "_gmtime_js":
				t := time.Unix(int64(uint64(uint32(stack[0]))|uint64(uint32(stack[1]))<<32), 0).UTC()
				for i, n := range []int{t.Second(), t.Minute(), t.Hour(), t.Day(), int(t.Month()) - 1, t.Year() - 1900, int(t.Weekday()), t.YearDay() - 1, 0} {
					m.Memory().WriteUint32Le(uint32(stack[2])+uint32(i*4), uint32(n))
				}
			default:
				panic("Little CMS 执行异常：" + name)
			}
		}), definition.ParamTypes(), definition.ResultTypes()).Export(name)
	}
	_, lcmsError = builder.Instantiate(ctx)
	if lcmsError == nil {
		_, lcmsError = wasi.Instantiate(ctx)
	}
}

func newLCMS() (context.Context, api.Module, func(), error) {
	initializeOnce.Do(initializeLCMS)
	if lcmsError != nil {
		return nil, nil, nil, lcmsError
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	select {
	case lcmsSlots <- struct{}{}:
	case <-ctx.Done():
		cancel()
		return nil, nil, nil, ctx.Err()
	}
	m, err := lcmsRuntime.InstantiateModule(ctx, lcmsCompiled, wazero.NewModuleConfig().WithName("").WithStartFunctions("__wasm_call_ctors"))
	cleanup := func() {
		if m != nil {
			m.Close(context.Background())
		}
		<-lcmsSlots
		cancel()
	}
	if err != nil {
		cleanup()
		return nil, nil, nil, fmt.Errorf("初始化 Little CMS 失败：%w", err)
	}
	return ctx, m, cleanup, nil
}
