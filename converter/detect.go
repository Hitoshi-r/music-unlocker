package converter

import (
	"bytes"
	"os"
)

var asfHeader = []byte{
	0x30, 0x26, 0xb2, 0x75, 0x8e, 0x66, 0xcf, 0x11,
	0xa6, 0xd9, 0x00, 0xaa, 0x00, 0x62, 0xce, 0x6c,
}

func DetectAudioExt(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	header := make([]byte, 16)
	n, err := file.Read(header)
	if err != nil {
		return "", err
	}

	return DetectAudioExtFromHeader(header[:n]), nil
}

func DetectAudioExtFromHeader(header []byte) string {
	if len(header) >= 4 && string(header[:4]) == "fLaC" {
		return ".flac"
	}
	if len(header) >= 4 && string(header[:4]) == "OggS" {
		return ".ogg"
	}
	if len(header) >= 4 && string(header[:4]) == "RIFF" {
		return ".wav"
	}
	if len(header) >= 8 && string(header[4:8]) == "ftyp" {
		return ".m4a"
	}
	if len(header) >= 4 && string(header[:4]) == "MAC " {
		return ".ape"
	}
	if len(header) >= len(asfHeader) && bytes.Equal(header[:len(asfHeader)], asfHeader) {
		return ".wma"
	}
	if len(header) >= 3 && string(header[:3]) == "ID3" {
		return ".mp3"
	}
	if len(header) >= 2 && header[0] == 0xFF && (header[1]&0xF6) == 0xF0 {
		return ".aac"
	}
	if len(header) >= 2 && header[0] == 0xFF && header[1]&0xE0 == 0xE0 {
		return ".mp3"
	}
	return ".bin"
}
