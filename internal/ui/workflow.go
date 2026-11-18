package ui

import (
	myui "github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
)

type pageWorkflow struct {
	action string
	input  string
}

var pageWorkflows = map[string]pageWorkflow{
	ToolConvert:    {"转换格式", "添加要转换的图片，再选输出格式"},
	ToolResize:     {"调整尺寸", "添加图片，再设置倍率或目标尺寸"},
	ToolCrop:       {"裁剪图片", "添加图片，再在预览中调整裁剪范围"},
	ToolGrid:       {"导出切片", "添加要切分的图片，再选宫格数量"},
	ToolLayoutPage: {"导出分割图", "添加图片，再设置行数、列数和填充方式"},
	ToolCollage:    {"导出拼图", "添加多张图片，再选择拼图排版"},
	ToolStitch:     {"拼接图片", "添加多张图片，再按拼接顺序排列"},
	ToolWatermark:  {"添加水印", "添加图片，再设置水印文字和位置"},
	ToolPalette:    {"导出色板", "添加图片，再设置提取颜色数"},
	ToolIcon:       {"生成图标", "添加标志图片，再选择输出的图标格式"},
	ToolPDF:        {"生成 PDF", "添加图片，再按 PDF 页序排列"},
	ToolGIF:        {"导出动图", "添加动图，再选择处理方式与输出格式"},
	ToolRichText:   {"生成长图", "在参数区编辑内容；不必先添加图片"},
	ToolCompress:   {"压缩图片", "添加图片，再选择压缩格式与质量"},
	ToolEdit:       {"导出编辑结果", "添加图片，再调整旋转、翻转或灰度"},
	ToolStudio:     {"导出画布", "可添加图片作为图层，也可直接创建画布"},
	ToolAnnotate:   {"导出标注图", "添加图片，再在预览中添加标注"},
	ToolScreenshot: {"导出美化图", "添加截图，再设置背景、边距和阴影"},
	ToolGrade:      {"导出调色图", "添加图片，再选预设或手动调整颜色"},
	ToolFilter:     {"导出滤镜图", "添加图片，再选滤镜与强度"},
	ToolCompare:    {"导出对比报告", "可从列表取图，也可在参数区单独选图"},
	ToolAnalysis:   {"分析图片", "选择列表中的图片，也可在参数区单独选图"},
	ToolPixel:      {"导出像素图", "添加图片，再选择像素尺寸和配色"},
	ToolEmote:      {"导出表情", "添加静图或动图，再选择表情规格"},
	ToolUpscale:    {"放大图片", "添加图片，也可在 AI 面板粘贴剪贴板图片"},
}

func workflowFor(tool Tool) pageWorkflow {
	if tool == nil {
		return pageWorkflow{"开始处理", "点击选择文件，或拖入文件夹"}
	}
	workflow := pageWorkflows[tool.ID()]
	switch current := tool.(type) {
	case *GIFTool:
		current.ensure()
		switch gifActionKeys[current.action] {
		case "reverse":
			workflow.action = "导出倒放动图"
		case "change_speed":
			workflow.action = "导出变速动图"
		case "compress":
			workflow.action = "压缩动图"
		case "resize":
			workflow.action = "导出缩放动图"
		case "export_frames":
			workflow.action = "导出帧图片"
			workflow.input = "添加动图，再选择要导出的帧范围"
		}
	case *CompareTool:
		switch {
		case current.reference == "":
			workflow.action = "选择参考图"
		case current.result == "":
			workflow.action = "选择结果图"
		case current.busy:
			workflow.action = "正在计算指标…"
		case !current.metrics.Computable:
			workflow.action = "重新计算指标"
		}
	case *ResizeTool:
		if current.mode == 3 {
			workflow.action = "导出多尺寸图片"
			workflow.input = "添加图片，再输入要导出的尺寸列表"
		}
	}
	return workflow
}

func (a *App) pageAction(tool Tool) (string, *myui.SVG, bool, func()) {
	label := workflowFor(tool).action
	switch current := tool.(type) {
	case *CompareTool:
		choose := func(path *string) func() {
			return func() { a.pickImageInto(path, func() { current.recompute(a) }) }
		}
		switch {
		case current.reference == "":
			return label, lucide.FolderOpen(), false, choose(&current.reference)
		case current.result == "":
			return label, lucide.FolderOpen(), false, choose(&current.result)
		case current.busy:
			return label, lucide.Activity(), true, a.Start
		case !current.metrics.Computable:
			return label, lucide.Activity(), false, func() { current.recompute(a) }
		}
	case *UpscaleTool:
		return current.primaryAction(a)
	case *AnalysisTool:
		if current.busy {
			return "正在分析…", lucide.Activity(), true, a.Start
		}
		if current.path == "" && a.selectedPreviewPath() == "" {
			return "选择要分析的图片", lucide.FolderOpen(), false, func() {
				a.pickImageInto(&current.path, func() { current.analyse(a, current.path) })
			}
		}
	}
	return label, lucide.Play(), a.Session.Len() == 0 && !toolRunsWithoutList(tool), a.Start
}
