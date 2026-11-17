package engine

import (
	"image"
	"image/color"
	"math"
	"sort"
)

type paletteSample struct {
	value [3]int
	count int
}

func pixelPalette(src image.Image, limit int) []color.NRGBA {
	bounds := src.Bounds()
	stride := maxInt(1, int(math.Ceil(math.Sqrt(float64(bounds.Dx()*bounds.Dy())/65536))))
	histogram := map[int]int{}
	for row := bounds.Min.Y; row < bounds.Max.Y; row += stride {
		for column := bounds.Min.X; column < bounds.Max.X; column += stride {
			pixel := color.NRGBAModel.Convert(src.At(column, row)).(color.NRGBA)
			key := int(pixel.R>>3)<<10 | int(pixel.G>>3)<<5 | int(pixel.B>>3)
			histogram[key]++
		}
	}
	keys := make([]int, 0, len(histogram))
	for key := range histogram {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	samples := make([]paletteSample, 0, len(keys))
	for _, key := range keys {
		samples = append(samples, paletteSample{[3]int{(key >> 10) * 255 / 31, ((key >> 5) & 31) * 255 / 31, (key & 31) * 255 / 31}, histogram[key]})
	}
	boxes := [][]paletteSample{samples}
	for len(boxes) < limit {
		bestBox, bestChannel, bestScore := -1, 0, -1
		for index, box := range boxes {
			if len(box) < 2 {
				continue
			}
			for channel := 0; channel < 3; channel++ {
				low, high, weight := 255, 0, 0
				for _, sample := range box {
					low = min(low, sample.value[channel])
					high = max(high, sample.value[channel])
					weight += sample.count
				}
				if score := (high - low) * weight; score > bestScore {
					bestBox, bestChannel, bestScore = index, channel, score
				}
			}
		}
		if bestBox < 0 {
			break
		}
		box := boxes[bestBox]
		sort.SliceStable(box, func(left, right int) bool { return box[left].value[bestChannel] < box[right].value[bestChannel] })
		weight := 0
		for _, sample := range box {
			weight += sample.count
		}
		middle, accumulated := 1, box[0].count
		for middle < len(box)-1 && accumulated < weight/2 {
			accumulated += box[middle].count
			middle++
		}
		boxes[bestBox] = box[:middle]
		boxes = append(boxes, box[middle:])
	}
	result := make([]color.NRGBA, 0, len(boxes))
	for _, box := range boxes {
		var total [3]int
		weight := 0
		for _, sample := range box {
			weight += sample.count
			for channel := range total {
				total[channel] += sample.value[channel] * sample.count
			}
		}
		if weight > 0 {
			result = append(result, color.NRGBA{R: uint8(total[0] / weight), G: uint8(total[1] / weight), B: uint8(total[2] / weight), A: 255})
		}
	}
	return result
}

func nearestPixelColor(value [3]float64, palette []color.NRGBA) color.NRGBA {
	best := color.NRGBA{A: 255}
	distance := math.Inf(1)
	for _, candidate := range palette {
		red, green, blue := value[0]-float64(candidate.R), value[1]-float64(candidate.G), value[2]-float64(candidate.B)
		if current := red*red + green*green + blue*blue; current < distance {
			distance, best = current, candidate
		}
	}
	return best
}
