package converter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func FinishAudio(rawPath string, outputDir string, name string, outputFormat string, bitrate string) (string, error) {
	realExt, err := DetectAudioExt(rawPath)
	if err != nil {
		return "", err
	}

	if outputFormat == "" || outputFormat == "auto" || outputFormat == "origin" {
		finalPath := UniqueOutputPath(outputDir, name, realExt)
		if err := os.Rename(rawPath, finalPath); err != nil {
			return "", err
		}
		return finalPath, nil
	}

	outputFormat = strings.ToLower(strings.TrimPrefix(outputFormat, "."))
	finalExt := "." + outputFormat
	finalPath := UniqueOutputPath(outputDir, name, finalExt)

	if finalExt == realExt {
		if err := os.Rename(rawPath, finalPath); err != nil {
			return "", err
		}
		return finalPath, nil
	}

	if err := ConvertAudio(rawPath, finalPath, outputFormat, bitrate); err != nil {
		return "", err
	}

	_ = os.Remove(rawPath)
	return finalPath, nil
}

func UniqueOutputPath(outputDir string, name string, ext string) string {
	if ext == "" {
		ext = ".bin"
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}

	candidate := filepath.Join(outputDir, name+ext)
	if _, err := os.Stat(candidate); os.IsNotExist(err) {
		return candidate
	}

	for i := 1; ; i++ {
		candidate = filepath.Join(outputDir, fmt.Sprintf("%s_%d%s", name, i, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}
