# Little CMS 2 WASM

`lcms.wasm` 来自 [lcms-wasm 1.0.5](https://registry.npmjs.org/lcms-wasm/-/lcms-wasm-1.0.5.tgz) 的 `dist/lcms.wasm`，保留原始二进制。
上游包装器：[mattdesl/lcms-wasm](https://github.com/mattdesl/lcms-wasm)。
Little CMS：[mm2/Little-CMS](https://github.com/mm2/Little-CMS)。

SHA-256：`dbcbee3fb3a49459a60bb6b29d6ab25fe7c219b0cc0c2b9579c0cc78f7390ddd`。

包装器与 Little CMS 均采用 MIT 许可，分别见 `LICENSE-WRAPPER` 和 `LICENSE-LCMS`。
Go 通过 wazero 执行模块，在内存中添加 Emscripten 回调桥，不修改内置文件。每次转换使用独立实例，最多同时运行 4 个实例；每实例内存上限 128 MiB，执行超时 2 分钟，不提供宿主文件系统。
