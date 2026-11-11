package engine

import (
	"context"
	"fmt"
	"os"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"

	"github.com/lllll081926i/imageflow-go/internal/colorprofile"
)

// tagPDFSRGB 为已统一到 sRGB 的 JPEG 图像指定 ICCBased，避免依赖阅读器的 DeviceRGB 默认值。
func tagPDFSRGB(source, target string) error {
	f, err := os.Open(source)
	if err != nil {
		return err
	}
	defer f.Close()
	conf := model.NewDefaultConfiguration()
	conf.WriteObjectStream = false
	conf.WriteXRefStream = false
	c := context.Background()
	ctx, err := api.ReadAndValidate(c, f, conf)
	if err != nil {
		return err
	}
	profile, err := ctx.NewStreamDictForBuf(colorprofile.SRGB())
	if err != nil {
		return err
	}
	profile.InsertInt("N", 3)
	profile.InsertName("Alternate", "DeviceRGB")
	if err := profile.Encode(); err != nil {
		return err
	}
	ref, err := ctx.IndRefForNewObject(*profile)
	if err != nil {
		return err
	}
	count := 0
	for _, entry := range ctx.Table {
		sd, ok := entry.Object.(types.StreamDict)
		if !ok {
			continue
		}
		if subtype := sd.NameEntry("Subtype"); subtype == nil || *subtype != "Image" {
			continue
		}
		if space := sd.NameEntry("ColorSpace"); space == nil || *space != "DeviceRGB" {
			return fmt.Errorf("PDF 图像包含尚未归一化的色彩空间")
		}
		sd.Update("ColorSpace", types.Array{types.Name("ICCBased"), *ref})
		entry.Object = sd
		count++
	}
	if count == 0 {
		return fmt.Errorf("PDF 未找到可设置色彩配置的图像")
	}
	return api.WriteContextFile(c, ctx, target)
}
