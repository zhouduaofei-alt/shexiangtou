package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

func main() {
	sizes := []int{16, 24, 32, 48, 64, 128, 256}
	images := make([]*image.NRGBA, len(sizes))
	for i, n := range sizes {
		images[i] = drawMark(n)
	}
	out := "icon.ico"
	if len(os.Args) > 1 {
		out = os.Args[1]
	}
	if err := os.WriteFile(out, buildICO(images), 0o644); err != nil {
		panic(err)
	}
	preview := filepath.Join(filepath.Dir(out), "icon-preview.png")
	f, err := os.Create(preview)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := png.Encode(f, images[len(images)-1]); err != nil {
		panic(err)
	}
}

func drawMark(n int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, n, n))
	cx := float64(n) / 2
	cy := float64(n) / 2
	aa := math.Max(0.8, float64(n)/128)
	rDisk := float64(n) * 0.47
	rings := [][2]float64{
		{0.40, 0.335},
		{0.24, 0.175},
	}
	rCore := float64(n) * 0.055
	dark := [4]float64{22, 20, 16, 255}
	gold := [4]float64{198, 163, 106, 255}
	light := [4]float64{236, 220, 180, 255}
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			dx := float64(x) + 0.5 - cx
			dy := float64(y) + 0.5 - cy
			d := math.Hypot(dx, dy)
			disk := smooth(rDisk-d, aa)
			if disk <= 0 {
				continue
			}
			px := dark
			for i, ring := range rings {
				outer := ring[0] * float64(n)
				inner := ring[1] * float64(n)
				cover := smooth(d-inner, aa) * smooth(outer-d, aa)
				tone := gold
				if i == 1 {
					tone = light
				}
				px = mix(px, tone, clamp(cover, 0, 1))
			}
			px = mix(px, light, clamp(smooth(rCore-d, aa), 0, 1))
			px[3] *= disk
			img.SetNRGBA(x, y, nrgba(px))
		}
	}
	return img
}

func smooth(x, aa float64) float64 {
	if aa <= 0 {
		if x >= 0 {
			return 1
		}
		return 0
	}
	return clamp(x/aa+0.5, 0, 1)
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func mix(a, b [4]float64, t float64) [4]float64 {
	return [4]float64{
		a[0] + (b[0]-a[0])*t,
		a[1] + (b[1]-a[1])*t,
		a[2] + (b[2]-a[2])*t,
		a[3] + (b[3]-a[3])*t,
	}
}

func nrgba(c [4]float64) color.NRGBA {
	return color.NRGBA{
		R: uint8(clamp(c[0], 0, 255)),
		G: uint8(clamp(c[1], 0, 255)),
		B: uint8(clamp(c[2], 0, 255)),
		A: uint8(clamp(c[3], 0, 255)),
	}
}

func buildICO(images []*image.NRGBA) []byte {
	frames := make([][]byte, len(images))
	for i, img := range images {
		frames[i] = dib32(img)
	}
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.LittleEndian, uint16(0))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(len(images)))
	offset := 6 + 16*len(images)
	for i, img := range images {
		n := img.Bounds().Dx()
		dim := byte(n)
		if n >= 256 {
			dim = 0
		}
		buf.WriteByte(dim)
		buf.WriteByte(dim)
		buf.WriteByte(0)
		buf.WriteByte(0)
		_ = binary.Write(&buf, binary.LittleEndian, uint16(1))
		_ = binary.Write(&buf, binary.LittleEndian, uint16(32))
		_ = binary.Write(&buf, binary.LittleEndian, uint32(len(frames[i])))
		_ = binary.Write(&buf, binary.LittleEndian, uint32(offset))
		offset += len(frames[i])
	}
	for _, frame := range frames {
		buf.Write(frame)
	}
	return buf.Bytes()
}

func dib32(img *image.NRGBA) []byte {
	n := img.Bounds().Dx()
	h := img.Bounds().Dy()
	xor := make([]byte, n*h*4)
	for y := 0; y < h; y++ {
		srcY := h - 1 - y
		for x := 0; x < n; x++ {
			c := img.NRGBAAt(x, srcY)
			i := (y*n + x) * 4
			xor[i] = c.B
			xor[i+1] = c.G
			xor[i+2] = c.R
			xor[i+3] = c.A
		}
	}
	row := ((n + 31) / 32) * 4
	and := make([]byte, row*h)
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.LittleEndian, uint32(40))
	_ = binary.Write(&buf, binary.LittleEndian, int32(n))
	_ = binary.Write(&buf, binary.LittleEndian, int32(h*2))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(32))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(len(xor)+len(and)))
	_ = binary.Write(&buf, binary.LittleEndian, int32(0))
	_ = binary.Write(&buf, binary.LittleEndian, int32(0))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0))
	buf.Write(xor)
	buf.Write(and)
	return buf.Bytes()
}
