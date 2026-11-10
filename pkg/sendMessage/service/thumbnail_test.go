package send_service

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os/exec"
	"testing"

	"go.mau.fi/whatsmeow/proto/waE2E"
)

// encodePNG builds a simple solid-color PNG of the given size for testing.
func encodePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 10, G: 120, B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("failed to encode test PNG: %v", err)
	}
	return buf.Bytes()
}

func TestMakeJPEGThumbnail_ResizesLandscape(t *testing.T) {
	src := encodePNG(t, 800, 400)

	thumb := makeJPEGThumbnail(src, 72)
	if thumb == nil {
		t.Fatal("expected a thumbnail, got nil")
	}

	cfg, format, err := image.DecodeConfig(bytes.NewReader(thumb))
	if err != nil {
		t.Fatalf("thumbnail is not a decodable image: %v", err)
	}
	if format != "jpeg" {
		t.Fatalf("expected jpeg thumbnail, got %q", format)
	}
	if cfg.Width != 72 {
		t.Fatalf("expected width 72, got %d", cfg.Width)
	}
	// Aspect ratio (2:1) must be preserved: 72 wide -> 36 tall.
	if cfg.Height != 36 {
		t.Fatalf("expected height 36 to preserve aspect ratio, got %d", cfg.Height)
	}
}

func TestMakeJPEGThumbnail_DoesNotUpscaleSmallImages(t *testing.T) {
	src := encodePNG(t, 40, 40)

	thumb := makeJPEGThumbnail(src, 72)
	if thumb == nil {
		t.Fatal("expected a thumbnail, got nil")
	}

	cfg, _, err := image.DecodeConfig(bytes.NewReader(thumb))
	if err != nil {
		t.Fatalf("thumbnail is not a decodable image: %v", err)
	}
	if cfg.Width != 40 || cfg.Height != 40 {
		t.Fatalf("expected the thumbnail to keep the original 40x40 size, got %dx%d", cfg.Width, cfg.Height)
	}
}

func TestMakeJPEGThumbnail_InvalidInputReturnsNil(t *testing.T) {
	if thumb := makeJPEGThumbnail([]byte("not an image"), 72); thumb != nil {
		t.Fatalf("expected nil for non-image input, got %d bytes", len(thumb))
	}
	if thumb := makeJPEGThumbnail(nil, 72); thumb != nil {
		t.Fatalf("expected nil for nil input, got %d bytes", len(thumb))
	}
}

func TestMakeJPEGThumbnail_DefaultsInvalidMaxWidth(t *testing.T) {
	src := encodePNG(t, 800, 800)

	thumb := makeJPEGThumbnail(src, 0)
	if thumb == nil {
		t.Fatal("expected a thumbnail with defaulted maxWidth, got nil")
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(thumb))
	if err != nil {
		t.Fatalf("thumbnail is not a decodable image: %v", err)
	}
	if cfg.Width != 72 {
		t.Fatalf("expected defaulted width 72, got %d", cfg.Width)
	}
}

func TestMakePDFThumbnail_InvalidInputReturnsNil(t *testing.T) {
	// With or without pdftoppm installed, garbage input must never panic and
	// must yield nil so the caller falls back to sending without a preview.
	if thumb := makePDFThumbnail([]byte("%PDF-not-really"), 200); thumb != nil {
		t.Fatalf("expected nil for invalid PDF input, got %d bytes", len(thumb))
	}
	if thumb := makePDFThumbnail(nil, 200); thumb != nil {
		t.Fatalf("expected nil for nil input, got %d bytes", len(thumb))
	}
}

func TestMakePDFThumbnail_RendersFirstPageWhenPopplerAvailable(t *testing.T) {
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		t.Skip("pdftoppm not installed; skipping PDF rasterization test")
	}

	pdf := minimalPDF(t)
	thumb := makePDFThumbnail(pdf, 200)
	if thumb == nil {
		t.Fatal("expected a thumbnail from a valid PDF, got nil")
	}

	cfg, format, err := image.DecodeConfig(bytes.NewReader(thumb))
	if err != nil {
		t.Fatalf("PDF thumbnail is not a decodable image: %v", err)
	}
	if format != "jpeg" {
		t.Fatalf("expected jpeg thumbnail, got %q", format)
	}
	if cfg.Width < 1 || cfg.Width > 200 {
		t.Fatalf("expected width within (0, 200], got %d", cfg.Width)
	}
}

// minimalPDF returns the bytes of a tiny, valid single-page PDF.
func minimalPDF(t *testing.T) []byte {
	t.Helper()
	const doc = "%PDF-1.1\n" +
		"1 0 obj<< /Type /Catalog /Pages 2 0 R >>endobj\n" +
		"2 0 obj<< /Type /Pages /Kids [3 0 R] /Count 1 >>endobj\n" +
		"3 0 obj<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] /Resources << >> >>endobj\n" +
		"trailer<< /Root 1 0 R >>\n" +
		"%%EOF\n"
	return []byte(doc)
}

// ensure the jpeg encoder import stays referenced even if the helper changes.
var _ = jpeg.DefaultQuality

func TestMakeJPEGThumbnail_CapsLongestSideOnPortrait(t *testing.T) {
	src := encodePNG(t, 400, 800)

	thumb := makeJPEGThumbnail(src, 72)
	cfg, _, err := image.DecodeConfig(bytes.NewReader(thumb))
	if err != nil {
		t.Fatalf("thumbnail is not a decodable image: %v", err)
	}
	if cfg.Width != 36 || cfg.Height != 72 {
		t.Fatalf("expected 36x72 (longest side capped), got %dx%d", cfg.Width, cfg.Height)
	}
}

func TestApplyImagePresentation_SetsDimensionsAndThumbnail(t *testing.T) {
	for _, tc := range []struct{ w, h, tw, th int }{
		{1080, 1920, 40, 72}, // retrato
		{1200, 600, 72, 36},  // paisagem
	} {
		msg := &waE2E.ImageMessage{}
		applyImagePresentation(msg, encodePNG(t, tc.w, tc.h))

		if msg.GetWidth() != uint32(tc.w) || msg.GetHeight() != uint32(tc.h) {
			t.Fatalf("expected %dx%d, got %dx%d", tc.w, tc.h, msg.GetWidth(), msg.GetHeight())
		}
		cfg, format, err := image.DecodeConfig(bytes.NewReader(msg.GetJPEGThumbnail()))
		if err != nil || format != "jpeg" {
			t.Fatalf("expected jpeg thumbnail, got format=%q err=%v", format, err)
		}
		if cfg.Width != tc.tw || cfg.Height != tc.th {
			t.Fatalf("expected thumbnail %dx%d, got %dx%d", tc.tw, tc.th, cfg.Width, cfg.Height)
		}
	}
}

func TestApplyImagePresentation_InvalidImageLeavesFieldsEmpty(t *testing.T) {
	msg := &waE2E.ImageMessage{}
	applyImagePresentation(msg, []byte("not an image"))
	if msg.Width != nil || msg.Height != nil || msg.JPEGThumbnail != nil {
		t.Fatalf("expected no dimensions/thumbnail for invalid input, got %v", msg)
	}
	applyImagePresentation(nil, encodePNG(t, 10, 10)) // nao pode entrar em panico
}

func TestApplyMediaPresentation_ImageMessageGetsDimensions(t *testing.T) {
	media := &waE2E.Message{ImageMessage: &waE2E.ImageMessage{}}
	applyMediaPresentation(media, &MediaStruct{}, encodePNG(t, 300, 500))
	if media.GetImageMessage().GetWidth() != 300 || media.GetImageMessage().GetHeight() != 500 {
		t.Fatalf("expected 300x500, got %dx%d", media.GetImageMessage().GetWidth(), media.GetImageMessage().GetHeight())
	}
}
