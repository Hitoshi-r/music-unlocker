package converter

import (
	"os"
	"path/filepath"
	"strings"
)

func FinishAudio(rawPath string, outputDir string, name string, outputFormat string, bitrate string) (string, error) {
	realExt, err := DetectAudioExt(rawPath)
	if err != nil {
		return "", err
	}

	if outputFormat == "" || outputFormat == "origin" {
		finalPath := filepath.Join(outputDir, name+realExt)

		if err := os.Rename(rawPath, finalPath); err != nil {
			return "", err
		}

		return finalPath, nil
	}

	outputFormat = strings.ToLower(outputFormat)
	finalPath := filepath.Join(outputDir, name+"."+outputFormat)

	if "."+outputFormat == realExt {
		if err := os.Rename(rawPath, finalPath); err != nil {
			return "", err
		}

		return finalPath, nil
	}

	if err := ConvertAudio(rawPath, finalPath, outputFormat, bitrate); err != nil {
		return "", err
	}

	os.Remove(rawPath)

	return finalPath, nil
}
