package converter

import "os"

func DetectAudioExt(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	header := make([]byte, 4)
	n, err := file.Read(header)
	if err != nil {
		return "", err
	}

	if n >= 4 && string(header[:4]) == "fLaC" {
		return ".flac", nil
	}

	if n >= 3 && string(header[:3]) == "ID3" {
		return ".mp3", nil
	}

	if n >= 2 && header[0] == 0xFF && (header[1]&0xE0) == 0xE0 {
		return ".mp3", nil
	}

	if n >= 4 && string(header[:4]) == "OggS" {
		return ".ogg", nil
	}

	return ".bin", nil
}
