package decoder

import (
	"os"
	"path/filepath"
	"strings"
)

var registeredDecoders = []Decoder{
	NewPlainAudioDecoder(),
	&NCMDecoder{},
	NewQMCDecoder(),
	NewMFLACDecoder(),
	&KGMDecoder{},
	&KWMDecoder{},
	&XMDecoder{},
}

func RegisteredDecoders() []Decoder {
	return append([]Decoder(nil), registeredDecoders...)
}

func SupportedExtensions() []string {
	seen := make(map[string]bool)
	var extensions []string

	for _, item := range registeredDecoders {
		for _, ext := range item.Extensions() {
			ext = strings.ToLower(ext)
			if !seen[ext] {
				seen[ext] = true
				extensions = append(extensions, ext)
			}
		}
	}

	return extensions
}

func FileDialogPattern() string {
	return "*" + strings.Join(SupportedExtensions(), ";*")
}

func SupportsPath(path string) bool {
	return FindByPath(path) != nil
}

func FindByPath(path string) Decoder {
	if hasPlainAudioHeader(path) {
		return NewPlainAudioDecoder()
	}

	ext := strings.ToLower(filepath.Ext(path))
	for _, item := range registeredDecoders {
		for _, supportedExt := range item.Extensions() {
			if ext == strings.ToLower(supportedExt) {
				return item
			}
		}
	}

	return nil
}

func DetectPlatform(path string) string {
	item := FindByPath(path)
	if item == nil {
		return "未知格式"
	}
	return item.Name()
}

func DetectPlainAudioExt(path string) string {
	header := readHeader(path, 16)
	if len(header) >= 4 && string(header[:4]) == "fLaC" {
		return ".flac"
	}
	if len(header) >= 4 && string(header[:4]) == "OggS" {
		return ".ogg"
	}
	if len(header) >= 4 && string(header[:4]) == "RIFF" {
		return ".wav"
	}
	if len(header) >= 3 && string(header[:3]) == "ID3" {
		return ".mp3"
	}
	if len(header) >= 2 && header[0] == 0xFF && header[1]&0xE0 == 0xE0 {
		return ".mp3"
	}
	return ""
}

func hasPlainAudioHeader(path string) bool {
	return DetectPlainAudioExt(path) != ""
}

func readHeader(path string, size int) []byte {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	header := make([]byte, size)
	n, err := file.Read(header)
	if err != nil {
		return nil
	}
	return header[:n]
}
