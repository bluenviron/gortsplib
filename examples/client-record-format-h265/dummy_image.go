//go:build cgo

package main

import (
	"image"
	"image/color"
)

var dummyImageCount = 0

func createDummyImage() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 640, 480))

	var cl color.RGBA
	switch dummyImageCount {
	case 0:
		cl = color.RGBA{255, 0, 0, 0}
	case 1:
		cl = color.RGBA{0, 255, 0, 0}
	case 2:
		cl = color.RGBA{0, 0, 255, 0}
	}
	dummyImageCount = (dummyImageCount + 1) % 3

	for y := 0; y < img.Rect.Dy(); y++ {
		for x := 0; x < img.Rect.Dx(); x++ {
			img.SetRGBA(x, y, cl)
		}
	}

	return img
}
