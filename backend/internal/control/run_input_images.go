package control

import (
	"encoding/base64"
	"mime"
	"path/filepath"
	"strings"
	"unicode"
)

const (
	maxRunInputImages     = 4
	maxRunInputImageBytes = 8 * 1024 * 1024
	maxRunInputImageTotal = 24 * 1024 * 1024
)

type normalizedRunInputImage struct {
	Filename string
	MimeType string
	Content  []byte
}

func normalizeRunInputImages(images []RunInputImageInput) ([]normalizedRunInputImage, error) {
	if len(images) == 0 {
		return nil, nil
	}
	if len(images) > maxRunInputImages {
		return nil, ErrValidation
	}
	out := make([]normalizedRunInputImage, 0, len(images))
	total := 0
	for index, image := range images {
		content, err := decodeImageBase64(image.ContentBase64)
		if err != nil {
			return nil, ErrValidation
		}
		if len(content) == 0 || len(content) > maxRunInputImageBytes {
			return nil, ErrValidation
		}
		total += len(content)
		if total > maxRunInputImageTotal {
			return nil, ErrValidation
		}
		mimeType := normalizeImageMimeType(image.MimeType, content)
		if mimeType == "" {
			return nil, ErrValidation
		}
		filename := sanitizeInputImageFilename(image.Filename, index, mimeType)
		out = append(out, normalizedRunInputImage{
			Filename: filename,
			MimeType: mimeType,
			Content:  content,
		})
	}
	return out, nil
}

func decodeImageBase64(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if before, after, ok := strings.Cut(value, ","); ok && strings.Contains(before, ";base64") {
		value = after
	}
	if value == "" {
		return nil, base64.CorruptInputError(0)
	}
	if len(value) > base64.StdEncoding.EncodedLen(maxRunInputImageBytes)+1024 {
		return nil, base64.CorruptInputError(0)
	}
	content, err := base64.StdEncoding.DecodeString(value)
	if err == nil {
		return content, nil
	}
	return base64.RawStdEncoding.DecodeString(value)
}

func normalizeImageMimeType(declared string, content []byte) string {
	declared = strings.ToLower(strings.TrimSpace(strings.Split(declared, ";")[0]))
	detected := detectAllowedImageMime(content)
	if detected == "" {
		return ""
	}
	if declared == "" || declared == "application/octet-stream" || declared == detected {
		return detected
	}
	if declared == "image/jpg" && detected == "image/jpeg" {
		return detected
	}
	return ""
}

func detectAllowedImageMime(content []byte) string {
	if len(content) >= 8 &&
		content[0] == 0x89 &&
		content[1] == 'P' &&
		content[2] == 'N' &&
		content[3] == 'G' &&
		content[4] == '\r' &&
		content[5] == '\n' &&
		content[6] == 0x1a &&
		content[7] == '\n' {
		return "image/png"
	}
	if len(content) >= 3 && content[0] == 0xff && content[1] == 0xd8 && content[2] == 0xff {
		return "image/jpeg"
	}
	if len(content) >= 6 && (string(content[:6]) == "GIF87a" || string(content[:6]) == "GIF89a") {
		return "image/gif"
	}
	if len(content) >= 12 && string(content[:4]) == "RIFF" && string(content[8:12]) == "WEBP" {
		return "image/webp"
	}
	return ""
}

func sanitizeInputImageFilename(filename string, index int, mimeType string) string {
	filename = strings.TrimSpace(filepath.Base(strings.ReplaceAll(filename, "\\", "/")))
	if filename == "." || filename == string(filepath.Separator) {
		filename = ""
	}
	if filename == "" {
		filename = "image-" + intString(index+1) + extensionForImageMimeType(mimeType)
	}
	filename = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == 0 || unicode.IsControl(r) {
			return '-'
		}
		return r
	}, filename)
	if len(filename) > 160 {
		ext := filepath.Ext(filename)
		stem := strings.TrimSuffix(filename, ext)
		if len(ext) > 20 {
			ext = ""
		}
		if len(stem) > 140 {
			stem = stem[:140]
		}
		filename = stem + ext
	}
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		filename += extensionForImageMimeType(mimeType)
	}
	return filename
}

func extensionForImageMimeType(mimeType string) string {
	if exts, err := mime.ExtensionsByType(mimeType); err == nil && len(exts) > 0 {
		if mimeType == "image/jpeg" {
			return ".jpg"
		}
		return exts[0]
	}
	switch mimeType {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ".img"
	}
}
