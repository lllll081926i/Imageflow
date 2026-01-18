# Wails v3 React + Go 拖拽文件交互技术方案

## 已收集资源

### 1. 官方文档
- Wails v2 拖拽文档: https://wails.io/docs/reference/runtime/draganddrop/
- Wails v3 事件系统: https://v3alpha.wails.io/features/events/system/
- Wails v3 官方示例: https://pkg.go.dev/github.com/wailsapp/wails/v3/examples/drag-n-drop

### 2. 核心概念

#### 拖拽文件的基本流程
1. 在 Go 应用配置中启用文件拖拽: `EnableFileDrop: true`
2. 在 HTML 中标记拖拽目标元素
3. 监听拖拽事件
4. 获取文件路径并处理

#### 关键 API

**Go 后端:**
- `OnFileDrop(ctx context.Context, callback func(x, y int, paths []string))` - 处理文件拖拽
- `OnFileDropOff(ctx context.Context)` - 移除拖拽监听
- `app.Event.Emit("event-name", data)` - 向前端发送事件
- `app.Event.On("event-name", handler)` - 监听前端事件

**JavaScript/React 前端:**
- `OnFileDrop(callback, useDropTarget)` - 监听文件拖拽
- `OnFileDropOff()` - 移除拖拽监听
- `Emit("event-name", data)` - 向后端发送事件
- `OnEvent("event-name", handler)` - 监听后端事件

### 3. 浏览器限制绕过

Wails 框架的核心优势在于：
- 前端运行在 WebKit 中，而非真正的浏览器
- Go 后端直接访问文件系统，无浏览器安全限制
- 通过 Wails 运行时 API，前端可以安全地获取完整文件路径
- 拖拽事件直接返回绝对文件路径数组，不受 CORS 或文件系统限制

## 实现流程

### 后端 (Go)

1. **启用文件拖拽**
```go
app := application.New(application.Options{
    EnableFileDrop: true,
})
```

2. **监听拖拽事件**
```go
app.Event.OnApplicationEvent(events.Common.WindowFilesDropped, func(e *application.ApplicationEvent) {
    files := e.Context().DroppedFiles()
    details := e.Context().DropTargetDetails()
    // 处理文件
})
```

3. **处理文件路径**
- 获取的 `paths []string` 包含完整的绝对路径
- 可以直接在 Go 中使用 `os` 包操作这些文件
- 支持文件和文件夹

### 前端 (React)

1. **导入运行时 API**
```javascript
import { OnEvent, Emit } from '@wailsio/runtime'
```

2. **监听后端事件**
```javascript
OnEvent("files-dropped", (data) => {
    const { files, details } = data
    // 处理文件列表
})
```

3. **标记拖拽目标**
```html
<div data-file-drop-target>
    拖拽文件到这里
</div>
```

## 关键特性

| 特性 | 说明 |
|------|------|
| 文件路径 | 获取完整绝对路径，无浏览器限制 |
| 文件夹支持 | 支持拖拽文件夹，获取文件夹路径 |
| 坐标信息 | 获取拖拽释放的坐标位置 |
| 目标识别 | 可识别拖拽到的具体 DOM 元素 |
| 样式反馈 | 自动添加 `file-drop-target-active` 类 |
| 跨平台 | Mac、Windows、Linux 都支持 |

## 参数传递方式

### 方式 1: 事件系统 (推荐)
- 后端监听拖拽事件，获取文件路径
- 后端处理文件，通过 `app.Event.Emit()` 发送结果给前端
- 前端通过 `OnEvent()` 接收处理结果

### 方式 2: 方法绑定
- 前端通过拖拽事件获取文件信息
- 调用绑定的 Go 方法，传递文件路径
- Go 方法直接处理文件，返回结果

### 方式 3: 混合方式
- 前端监听拖拽事件
- 调用 Go 方法处理文件（获取完整路径）
- Go 方法返回处理结果
- 前端更新 UI

## 优势总结

1. **无浏览器限制**: 直接访问文件系统，获取完整路径
2. **类型安全**: Go 类型系统确保数据安全
3. **高性能**: 本地文件操作，无网络延迟
4. **跨平台**: 一套代码支持多个平台
5. **易于集成**: Wails 提供统一的事件和方法绑定系统


## 完整实现示例

### 前端 React 实现

```javascript
import { Events } from '@wailsio/runtime';

export function FileDropZone() {
  const [files, setFiles] = React.useState([]);

  React.useEffect(() => {
    // 监听文件拖拽事件
    const unsubscribe = Events.On('common:WindowFilesDropped', (event) => {
      const droppedFiles = event.data.files;
      
      console.log('Files dropped:', droppedFiles);
      
      // 处理拖拽的文件
      droppedFiles.forEach(file => {
        console.log('File dropped:', file);
        // 可以在这里处理文件，例如上传到后端
        handleFileUpload(file);
      });
      
      setFiles(droppedFiles);
    });

    // 清理监听器
    return () => {
      unsubscribe();
    };
  }, []);

  return (
    <div style={{ 
      border: '2px dashed #ccc', 
      padding: '20px',
      textAlign: 'center'
    }}>
      <h2>拖拽文件到这里</h2>
      {files.length > 0 && (
        <ul>
          {files.map((file, index) => (
            <li key={index}>{file}</li>
          ))}
        </ul>
      )}
    </div>
  );
}
```

### 后端 Go 实现

```go
package main

import (
  "github.com/wailsapp/wails/v3/pkg/application"
  "github.com/wailsapp/wails/v3/pkg/events"
  "log"
  "os"
)

func main() {
  // 创建应用，启用文件拖拽
  app := application.New(application.Options{
    Name:        "File Drop Example",
    Description: "A simple file drop example",
    EnableFileDrop: true,  // 重要：启用文件拖拽
  })

  // 监听文件拖拽事件
  app.Event.OnApplicationEvent(events.Common.WindowFilesDropped, func(e *application.ApplicationEvent) {
    // 获取拖拽的文件列表
    files := e.Context().DroppedFiles()
    
    // 获取拖拽的位置信息
    details := e.Context().DropTargetDetails()
    
    app.Logger.Info("Files dropped", 
      "count", len(files),
      "x", details.X,
      "y", details.Y,
    )
    
    // 处理每个文件
    for _, filePath := range files {
      processFile(filePath)
    }
    
    // 向前端发送处理结果
    app.Event.Emit("files-processed", map[string]interface{}{
      "count": len(files),
      "files": files,
    })
  })

  // 运行应用
  if err := app.Run(); err != nil {
    log.Fatal(err)
  }
}

func processFile(filePath string) {
  // 获取文件信息
  fileInfo, err := os.Stat(filePath)
  if err != nil {
    log.Printf("Error getting file info: %v", err)
    return
  }
  
  if fileInfo.IsDir() {
    log.Printf("Dropped item is a directory: %s", filePath)
    // 处理文件夹
  } else {
    log.Printf("Dropped file: %s (size: %d bytes)", filePath, fileInfo.Size())
    // 处理文件
  }
}
```

### wails.json 配置

```json
{
  "name": "file-drop-app",
  "frontend": {
    "dir": "frontend",
    "build": "npm run build"
  },
  "options": {
    "enableFileDrop": true
  }
}
```

## 高级用法

### 1. 使用方法绑定处理文件

```go
// Go 服务
type FileService struct {
  app *application.Application
}

func (fs *FileService) ProcessFile(filePath string) (string, error) {
  // 直接处理文件
  data, err := os.ReadFile(filePath)
  if err != nil {
    return "", err
  }
  
  // 返回处理结果
  return fmt.Sprintf("Processed %d bytes", len(data)), nil
}

// 在应用中注册服务
app := application.New(application.Options{
  Services: []application.Service{
    application.NewService(&FileService{app: app}),
  },
})
```

```javascript
// React 前端
import { FileService } from './bindings/FileService';

async function handleFileUpload(filePath) {
  try {
    const result = await FileService.ProcessFile(filePath);
    console.log('Processing result:', result);
  } catch (error) {
    console.error('Error processing file:', error);
  }
}
```

### 2. 支持多个拖拽目标

```html
<!-- 在 HTML 中标记多个拖拽目标 -->
<div data-file-drop-target id="drop-zone-1">
  拖拽文件到这里
</div>

<div data-file-drop-target id="drop-zone-2">
  拖拽文件夹到这里
</div>
```

```javascript
// 前端可以根据 drop-target-details 识别是哪个元素
Events.On('common:WindowFilesDropped', (event) => {
  const details = event.data.details;
  const targetId = details.ElementID;
  
  if (targetId === 'drop-zone-1') {
    // 处理文件
  } else if (targetId === 'drop-zone-2') {
    // 处理文件夹
  }
});
```

### 3. 样式反馈

```css
/* 当文件被拖拽到目标元素上时，自动添加此类 */
[data-file-drop-target].file-drop-target-active {
  border-color: #4a9eff;
  background: rgba(74, 158, 255, 0.1);
  transform: scale(1.02);
  transition: all 0.2s ease;
}
```

## 常见问题

### Q: 如何获取文件的完整路径？
A: Wails 直接返回绝对路径字符串数组，无需额外处理。这是 Wails 的核心优势，绕过了浏览器的文件系统限制。

### Q: 支持拖拽文件夹吗？
A: 支持。拖拽文件夹时，会获取文件夹的路径。可以使用 `os.Stat()` 判断是文件还是文件夹。

### Q: 如何处理大文件？
A: 建议在 Go 后端处理大文件，使用流式读取而不是一次性加载到内存。

### Q: 如何在多个窗口中使用拖拽？
A: 每个窗口都可以独立监听拖拽事件。事件会发送到接收拖拽的窗口。

### Q: Windows 上拖拽不工作？
A: 确保使用最新的 Wails v3 alpha 版本（alpha 32 或更新）。之前的版本有已修复的 bug。

## 浏览器限制绕过原理

Wails 框架的核心优势在于：

1. **WebKit 而非浏览器**: 前端运行在 WebKit 中，而非真正的浏览器，因此不受浏览器同源策略限制

2. **本地 IPC 通信**: 前后端通过本地进程间通信（IPC）连接，而非网络请求

3. **直接文件系统访问**: Go 后端可以直接访问操作系统的文件系统，获取完整路径

4. **运行时 API**: Wails 提供的运行时 API 直接暴露文件路径，无需任何中间转换

这意味着：
- ✅ 获取完整文件路径
- ✅ 支持文件和文件夹
- ✅ 无跨域限制
- ✅ 无文件大小限制
- ✅ 支持所有文件类型
