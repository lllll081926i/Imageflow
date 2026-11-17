package upscale

import (
	"context"
	"image"
	"image/color"
	"image/draw"
	"os"

	"github.com/disintegration/imaging"
	"github.com/lllll081926i/imageflow-go/internal/codec"
	xdraw "golang.org/x/image/draw"
)

func writeInferenceInput(ctx context.Context, source image.Image, directory string) (string, error) {
	input, err := os.CreateTemp(directory, ".imageflow-upscale-input-*.png")
	if err != nil {
		return "", err
	}
	path := input.Name()
	if err := input.Close(); err != nil {
		os.Remove(path)
		return "", err
	}
	opaque := image.NewRGBA(source.Bounds())
	draw.Draw(opaque, opaque.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.Draw(opaque, opaque.Bounds(), source, source.Bounds().Min, draw.Over)
	if err := codec.SaveContext(ctx, opaque, path, codec.SaveOptions{SRGB: true}); err != nil {
		os.Remove(path)
		return "", err
	}
	return path, nil
}

func restoreInputAlpha(ctx context.Context, rendered, source image.Image) (image.Image, bool, error) {
	if opaque, ok := source.(interface{ Opaque() bool }); ok && opaque.Opaque() {
		return rendered, false, nil
	}
	alpha := image.NewAlpha(image.Rect(0, 0, source.Bounds().Dx(), source.Bounds().Dy()))
	for row := 0; row < alpha.Bounds().Dy(); row++ {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		for column := 0; column < alpha.Bounds().Dx(); column++ {
			_, _, _, opacity := source.At(source.Bounds().Min.X+column, source.Bounds().Min.Y+row).RGBA()
			alpha.Pix[row*alpha.Stride+column] = uint8(opacity >> 8)
		}
	}
	resized := image.NewAlpha(image.Rect(0, 0, rendered.Bounds().Dx(), rendered.Bounds().Dy()))
	xdraw.CatmullRom.Scale(resized, resized.Bounds(), alpha, alpha.Bounds(), draw.Src, nil)
	result, ok := rendered.(*image.NRGBA)
	if !ok {
		result = imaging.Clone(rendered)
	}
	for row := 0; row < result.Bounds().Dy(); row++ {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		for column := 0; column < result.Bounds().Dx(); column++ {
			result.Pix[row*result.Stride+column*4+3] = resized.Pix[row*resized.Stride+column]
		}
	}
	return result, true, nil
}
