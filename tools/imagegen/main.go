// Package main is the build-time image generator for the MuxMaster
// documentation website.
//
// It reads the canonical 1024×1024 logo at tools/imagegen/source.png and produces
// every sized variant the templates and OG metadata reference: header logos,
// favicons, apple-touch-icons, and a 1200×630 Open Graph composition. All
// outputs are written deterministically (no metadata timestamps in the PNG
// stream) so successive runs against the same input yield identical bytes,
// keeping the static-tending principle intact.
//
// Invoked from `make assets`. Not linked into the runtime binary; the
// dependency on golang.org/x/image stays out of the production import graph.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"path/filepath"

	_ "image/png" // PNG decoder for loadPNG.

	"golang.org/x/image/draw"
)

// sourcePath is the canonical 1024×1024 RGBA PNG vendored from
// `../MuxMaster/assets/logo-muxmaster.png` by `make logo`. It lives in
// `tools/imagegen/` rather than under `static/` because it is a *build
// input*, not a runtime asset: serving it publicly would expose a 1.6 MB
// PNG that no page references. The `make logo` target writes here; this
// program reads from here; `make assets` derives every variant under
// `static/img/` and `static/favicon/` from this single source.
const sourcePath = "tools/imagegen/source.png"

type variant struct {
	path    string
	w, h    int
	scaler  draw.Interpolator // CatmullRom for downscale; ApproxBiLinear is faster but blurrier.
}

func main() {
	src, err := loadPNG(sourcePath)
	if err != nil {
		log.Fatalf("imagegen: %v", err)
	}

	variants := []variant{
		// Header logo, with retina densities. The header renders at 32px on
		// small viewports and 40px on md+. logo-80.png covers up to 2.5×.
		{path: "static/img/logo-32.png", w: 32, h: 32, scaler: draw.CatmullRom},
		{path: "static/img/logo-64.png", w: 64, h: 64, scaler: draw.CatmullRom},
		{path: "static/img/logo-80.png", w: 80, h: 80, scaler: draw.CatmullRom},
		{path: "static/img/logo-128.png", w: 128, h: 128, scaler: draw.CatmullRom},
		{path: "static/img/logo-192.png", w: 192, h: 192, scaler: draw.CatmullRom},
		{path: "static/img/logo-256.png", w: 256, h: 256, scaler: draw.CatmullRom},
		{path: "static/img/logo-384.png", w: 384, h: 384, scaler: draw.CatmullRom},

		// Favicon set used by templates/partials/head.html. PNG only; ICO is
		// not produced (modern browsers prefer PNG and the spec lists PNGs
		// at /static/favicon/, not /favicon.ico).
		{path: "static/favicon/favicon-32.png", w: 32, h: 32, scaler: draw.CatmullRom},
		{path: "static/favicon/favicon-192.png", w: 192, h: 192, scaler: draw.CatmullRom},
		{path: "static/favicon/favicon-512.png", w: 512, h: 512, scaler: draw.CatmullRom},
		{path: "static/favicon/apple-touch-icon-180.png", w: 180, h: 180, scaler: draw.CatmullRom},
	}

	for _, v := range variants {
		dst := image.NewNRGBA(image.Rect(0, 0, v.w, v.h))
		v.scaler.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
		if err := writePNG(v.path, dst); err != nil {
			log.Fatalf("imagegen: write %s: %v", v.path, err)
		}
		fmt.Printf("wrote %-44s %4d×%-4d\n", v.path, v.w, v.h)
	}

	// Open Graph image. 1200×630 is the recommended Twitter/Facebook ratio.
	// White background (zinc-50, #fafafa) with the logo centred at 480×480.
	og := image.NewNRGBA(image.Rect(0, 0, 1200, 630))
	bg := color.NRGBA{R: 250, G: 250, B: 250, A: 255}
	draw.Draw(og, og.Bounds(), &image.Uniform{C: bg}, image.Point{}, draw.Src)
	logoSize := 480
	logoX := (1200 - logoSize) / 2
	logoY := (630 - logoSize) / 2
	logoRect := image.Rect(logoX, logoY, logoX+logoSize, logoY+logoSize)
	draw.CatmullRom.Scale(og, logoRect, src, src.Bounds(), draw.Over, nil)
	if err := writePNG("static/img/og-image.png", og); err != nil {
		log.Fatalf("imagegen: write og-image: %v", err)
	}
	fmt.Printf("wrote %-44s %4d×%-4d\n", "static/img/og-image.png", 1200, 630)
}

func loadPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

func writePNG(path string, img image.Image) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	return enc.Encode(f, img)
}
