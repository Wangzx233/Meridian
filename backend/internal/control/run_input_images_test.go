package control

import (
	"errors"
	"testing"
)

func TestNormalizeRunInputImagesUsesDetectedMimeType(t *testing.T) {
	images, err := normalizeRunInputImages([]RunInputImageInput{{
		Filename:      "screen.png",
		MimeType:      "application/octet-stream",
		ContentBase64: "iVBORw0KGgo=",
	}})
	if err != nil {
		t.Fatalf("normalize image returned error: %v", err)
	}
	if len(images) != 1 {
		t.Fatalf("images length = %d, want 1", len(images))
	}
	if images[0].MimeType != "image/png" {
		t.Fatalf("mime type = %q, want image/png", images[0].MimeType)
	}
}

func TestNormalizeRunInputImagesReportsSpecificValidation(t *testing.T) {
	_, err := normalizeRunInputImages([]RunInputImageInput{{
		Filename:      "not-image.png",
		MimeType:      "image/png",
		ContentBase64: "bm90IGFuIGltYWdl",
	}})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("normalize invalid image error = %v, want validation", err)
	}
	if got, want := err.Error(), "Image 1 must be a PNG, JPEG, GIF, or WebP file."; got != want {
		t.Fatalf("validation message = %q, want %q", got, want)
	}
}
