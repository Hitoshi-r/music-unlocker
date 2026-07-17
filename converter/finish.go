package converter

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func FinishAudio(
	ctx context.Context,
	rawPath string,
	outputDir string,
	name string,
	outputFormat string,
	bitrate string,
) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cleanupRaw := true
	defer func() {
		if cleanupRaw {
			_ = os.Remove(rawPath)
		}
	}()
	if ctx.Err() != nil {
		return "", ErrTaskCanceled
	}

	realExt, err := DetectAudioExt(rawPath)
	if err != nil {
		return "", err
	}

	if outputFormat == "" || outputFormat == "auto" || outputFormat == "origin" {
		finalPath := UniqueOutputPath(outputDir, name, realExt)
		if ctx.Err() != nil {
			return "", ErrTaskCanceled
		}
		if err := os.Rename(rawPath, finalPath); err != nil {
			return "", err
		}
		cleanupRaw = false
		if ctx.Err() != nil {
			_ = os.Remove(finalPath)
			return "", ErrTaskCanceled
		}
		return finalPath, nil
	}

	outputFormat = strings.ToLower(strings.TrimPrefix(outputFormat, "."))
	finalExt := "." + outputFormat
	finalPath := UniqueOutputPath(outputDir, name, finalExt)

	if finalExt == realExt {
		if ctx.Err() != nil {
			return "", ErrTaskCanceled
		}
		if err := os.Rename(rawPath, finalPath); err != nil {
			return "", err
		}
		cleanupRaw = false
		if ctx.Err() != nil {
			_ = os.Remove(finalPath)
			return "", ErrTaskCanceled
		}
		return finalPath, nil
	}

	if err := ConvertAudio(ctx, rawPath, finalPath, outputFormat, bitrate); err != nil {
		_ = os.Remove(finalPath)
		return "", err
	}
	if ctx.Err() != nil {
		_ = os.Remove(finalPath)
		return "", ErrTaskCanceled
	}

	_ = os.Remove(rawPath)
	cleanupRaw = false
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
