package converter

import "os"

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
	if len(header) >= 3 && string(header[:3]) == "ID3" {
		return ".mp3"
	}
	if len(header) >= 2 && header[0] == 0xFF && header[1]&0xE0 == 0xE0 {
		return ".mp3"
	}
	return ".bin"
}
