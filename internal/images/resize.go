package images

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	_ "image/png"
)

const (
	maxDimension = 800
	jpegQuality  = 82
)

func Resize(data []byte) ([]byte, error) {
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	flat := flatten(src)
	w, h := fit(flat.Bounds())
	dst := scale(flat, w, h)

	buf := new(bytes.Buffer)

	err = jpeg.Encode(buf, dst, &jpeg.Options{Quality: jpegQuality})
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func flatten(src image.Image) *image.RGBA {
	bounds := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(dst, dst.Bounds(), src, bounds.Min, draw.Over)

	return dst
}

func fit(bounds image.Rectangle) (int, int) {
	w, h := bounds.Dx(), bounds.Dy()
	if w <= maxDimension && h <= maxDimension {
		return w, h
	}

	if w >= h {
		return maxDimension, max(1, h*maxDimension/w)
	}

	return max(1, w*maxDimension/h), maxDimension
}

func scale(src *image.RGBA, w, h int) *image.RGBA {
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	if w == sw && h == sh {
		return src
	}

	dst := image.NewRGBA(image.Rect(0, 0, w, h))

	for y := range h {
		y0, y1 := y*sh/h, (y+1)*sh/h
		if y1 == y0 {
			y1++
		}

		for x := range w {
			x0, x1 := x*sw/w, (x+1)*sw/w
			if x1 == x0 {
				x1++
			}

			var r, g, b, n uint32
			for sy := y0; sy < y1; sy++ {
				row := src.Pix[sy*src.Stride:]
				for sx := x0; sx < x1; sx++ {
					p := row[sx*4:]
					r += uint32(p[0])
					g += uint32(p[1])
					b += uint32(p[2])
					n++
				}
			}

			p := dst.Pix[y*dst.Stride+x*4:]
			p[0] = uint8(r / n)
			p[1] = uint8(g / n)
			p[2] = uint8(b / n)
			p[3] = 0xff
		}
	}

	return dst
}
