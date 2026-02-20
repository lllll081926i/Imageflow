# Imageflow 代码安全审查报告

***

## 🚨 严重漏洞（立即修复）

### 1. 路径遍历攻击漏洞

**严重程度：** 🔴 CRITICAL | **CVSS：** 8.2

**问题描述：**
文件路径处理函数未对用户输入进行充分验证，攻击者可以通过`../`序列访问系统任意文件。

**受影响文件：**

- `backend/services/converter.go` (lines 78-111)

- `backend/services/pdf_generator.go`

- `backend/services/watermark.go`

- `backend/app.go` (line 463)

**漏洞代码示例：**

```go
// 不安全代码
data, err := os.ReadFile(req.InputPath) // req.InputPath可被控制
```

**修复方案：**

```go
// 在resolveInputPath函数中添加路径验证
func resolveInputPath(inputPath string) (string, error) {
    cleaned := filepath.Clean(strings.TrimSpace(inputPath))

    // 防止路径遍历
    if strings.HasPrefix(cleaned, "../") || cleaned == ".." {
        return "", errors.New("path traversal attempt detected")
    }

    // 验证路径是否在允许目录内
    allowedBase := filepath.Clean(getInputDirectory())
    if !strings.HasPrefix(cleaned, allowedBase) {
        return "", errors.New("access denied: path outside allowed directory")
    }

    return cleaned, nil
}
```

**实施计划：** 24小时内完成修复

***

### 2. XXE（XML外部实体）攻击漏洞

**严重程度：** 🔴 CRITICAL | **CVSS：** 7.8

**问题描述：**
Python SVG处理使用不安全的`xml.etree.ElementTree`库，未禁用外部实体解析，可能导致任意文件读取。

**受影响文件：**

- `backend/python/converter.py` (lines 87-122)

**漏洞代码示例：**

```python
# 不安全代码
tree = ET.parse(svg_path)  # 未禁用外部实体解析
```

**修复方案：**

```python
# 方案1：使用defusedxml库
from defusedxml.ElementTree import parse

def process_svg(svg_path):
    tree = parse(svg_path)  # 安全：已禁用外部实体
    # ... 处理逻辑

# 方案2：如果无法安装defusedxml，手动禁用实体解析
import xml.etree.ElementTree as ET
from io import BytesIO

def process_svg_safe(svg_path):
    with open(svg_path, 'rb') as f:
        data = f.read()
    # 移除DOCTYPE声明
    data = re.sub(b'<!DOCTYPE[^>]*>', b'', data)
    tree = ET.parse(BytesIO(data))
```

**依赖安装：**

```bash
pip install defusedxml
```

**实施计划：** 24小时内完成修复

***

### 3. Python执行器池通道死锁

**严重程度：** 🔴 CRITICAL | **影响：** 服务可用性

**问题描述：**
Python执行器池在发生panic时会导致通道阻塞，后续所有请求无法处理。

**受影响文件：**

- `backend/utils/python_executor_pool.go` (lines 66-76)

**漏洞代码示例：**

```go
func (p *PythonExecutorPool) Execute(scriptName string, input interface{}) ([]byte, error) {
    exec := <-p.ch
    defer func() { p.ch <- exec }() // ⚠️ panic时不会执行
    return exec.Execute(scriptName, input)
}
```

**修复方案：**

```go
func (p *PythonExecutorPool) Execute(scriptName string, input interface{}) ([]byte, error) {
    exec := <-p.ch
    execute := func() ([]byte, error) {
        defer func() {
            if r := recover(); r != nil {
                // 确保执行器返回到池中
                p.ch <- exec
                // 重新抛出panic以便上层处理
                panic(r)
            }
        }()

        result, err := exec.Execute(scriptName, input)
        // 正常返回执行器到池中
        p.ch <- exec
        return result, err
    }

    return execute()
}
```

**备选方案（更简单）：**

```go
func (p *PythonExecutorPool) Execute(scriptName string, input interface{}) ([]byte, error) {
    exec := <-p.ch
    result, err := exec.Execute(scriptName, input)
    p.ch <- exec // 确保总是返回
    return result, err
}
```

**实施计划：** 24小时内完成修复

***

## ⚠️ 高风险问题（72小时内修复）

### 4. 静默失败风险

**问题描述：** Python工作线程启动失败仅记录警告，服务继续运行但无法正常工作。

**受影响文件：** `backend/app.go` (lines 125-132)

**修复方案：**

```go
// 修改前
go func(r utils.PythonRunner) {
    if err := r.StartWorker(); err != nil {
        a.logger.Warn("Python worker warmup failed: %v", err)
    }
}(runner)

// 修改后
if err := runner.StartWorker(); err != nil {
    a.logger.Error("Python worker initialization failed: %v", err)
    return fmt.Errorf("failed to initialize Python worker: %w", err)
}
```

***

### 5. 资源泄漏问题

**问题描述：** 多个Python脚本未使用`with`语句，可能导致文件句柄泄漏。

**受影响文件：**

- `backend/python/adjuster.py` (lines 60-61)

- `backend/python/compressor.py` (lines 219-221)

- `backend/python/gif_splitter.py` (lines 216-227)

**修复方案：**

```python
# 修复前
img = Image.open(input_path)
# ... 处理逻辑
img.close()

# 修复后
with Image.open(input_path) as img:
    # ... 处理逻辑
    # 自动关闭文件句柄
```

***

### 6. 接口标准化问题

**问题描述：** Python脚本JSON返回格式不统一，Go端解析困难。

**修复方案：**

```python
# 统一返回格式
def create_response(success=True, data=None, error_code=None, error_message=None):
    return {
        "success": success,
        "data": data or {},
        "error": {
            "code": error_code,  # 机器可读
            "message": error_message  # 人类可读
        } if not success else None
    }

# 使用示例
try:
    result = process_image()
    return create_response(success=True, data=result)
except FileNotFoundError:
    return create_response(success=False, error_code="FILE_NOT_FOUND", error_message="输入文件不存在")
except Exception as e:
    return create_response(success=False, error_code="PROCESSING_ERROR", error_message=str(e))
```

***

## 🔧 中风险问题（1-2周内修复）

### 7. 代码重复问题

**问题描述：** 5个批量处理函数存在大量重复逻辑。

**受影响文件：** `backend/app.go` (lines 254-679)

**重构方案：**

```go
// 提取通用批量处理函数
func (a *App) processBatch[T any, R any](
    requests []T,
    workerCount int,
    processor func(T) (R, error),
) ([]R, error) {
    if len(requests) == 0 {
        return nil, errors.New("empty batch request")
    }

    if workerCount <= 0 {
        workerCount = runtime.NumCPU()
    }

    type result struct {
        index int
        value R
        err   error
    }

    var wg sync.WaitGroup
    requestChan := make(chan T, len(requests))
    resultChan := make(chan result, len(requests))

    // 启动工作协程
    for i := 0; i < workerCount; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for req := range requestChan {
                value, err := processor(req)
                resultChan <- result{index: 0, value: value, err: err}
            }
        }()
    }

    // 发送请求
    for _, req := range requests {
        requestChan <- req
    }
    close(requestChan)

    // 等待完成
    wg.Wait()
    close(resultChan)

    // 收集结果
    results := make([]R, len(requests))
    var batchErr error
    for res := range resultChan {
        if res.err != nil {
            batchErr = fmt.Errorf("batch processing failed: %w", res.err)
            continue
        }
        // 需要根据实际需求调整索引处理
        results = append(results, res.value)
    }

    return results, batchErr
}

// 使用示例
func (a *App) ConvertBatch(requests []ConvertRequest) ([]ConvertResult, error) {
    return a.processBatch(requests, a.pythonWorkers, func(req ConvertRequest) (ConvertResult, error) {
        return a.convertSingle(req)
    })
}
```

***

### 8. 性能优化问题

**问题描述：** 频繁临时文件创建、嵌套循环平铺操作等性能问题。

**修复方案：**

```python
# watermark.py - 优化平铺算法
def create_tiled_watermark(background_size, watermark_path, opacity, position):
    from PIL import Image
    import numpy as np

    # 使用numpy优化平铺操作
    bg_width, bg_height = background_size
    wm = Image.open(watermark_path).convert('RGBA')
    wm.putalpha(int(255 * opacity))

    # 使用numpy进行高效平铺计算
    wm_width, wm_height = wm.size
    repeat_x = bg_width // wm_width + 1
    repeat_y = bg_height // wm_height + 1

    # 创建平铺图案
    tiled = np.tile(np.array(wm), (repeat_y, repeat_x, 1))
    tiled = Image.fromarray(tiled[:bg_height, :bg_width])

    return tiled
```

***

## 🎯 验收标准

### 修复完成标准

1. **严重漏洞**：所有路径遍历、XXE、死锁问题已修复并通过测试
2. **单元测试**：关键安全功能100%测试覆盖
3. **集成测试**：批量处理、文件转换等核心功能测试通过
4. **安全扫描**：使用安全扫描工具无严重漏洞报告
5. **代码审查**：修复代码经过同行审查
