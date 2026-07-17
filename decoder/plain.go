package decoder

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"music-unlocker/converter"
)

type PlainAudioDecoder struct{}

func NewPlainAudioDecoder() *PlainAudioDecoder {
	return &PlainAudioDecoder{}
}

func (d *PlainAudioDecoder) Name() string {
	return "标准音频"
}

func (d *PlainAudioDecoder) Extensions() []string {
	return []string{".mp3", ".flac", ".ogg", ".wav", ".aac", ".m4a"}
}

func (d *PlainAudioDecoder) Decode(
	inputPath string,
	outputDir string,
	outputFormat string,
	bitrate string,
	_ DecodeOptions,
	ctx context.Context,
	onProgress ProgressCallback,
) (*DecodeResult, error) {
	if outputDir == "" {
		outputDir = filepath.Dir(inputPath)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, err
	}

	realExt := DetectPlainAudioExt(inputPath)
	if realExt == "" {
		return nil, errors.New("不是可识别的标准音频文件")
	}

	name := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	rawPath := filepath.Join(outputDir, name+"_plain_raw"+realExt)

	if err := copyWithProgress(inputPath, rawPath, ctx, onProgress); err != nil {
		os.Remove(rawPath)
		return nil, err
	}

	finalPath, err := converter.FinishAudio(ctx, rawPath, outputDir, name, outputFormat, bitrate)
	if err != nil {
		os.Remove(rawPath)
		return nil, err
	}

	if onProgress != nil {
		onProgress(100)
	}

	return &DecodeResult{
		InputPath:  inputPath,
		OutputPath: finalPath,
		Platform:   d.Name(),
		Success:    true,
		Message:    "标准音频处理完成",
	}, nil
}

func copyWithProgress(inputPath string, outputPath string, ctx context.Context, onProgress ProgressCallback) error {
	in, err := os.Open(inputPath)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer out.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	total := info.Size()
	var processed int64
	lastPercent := -1
	buffer := make([]byte, 64*1024)

	for {
		select {
		case <-ctx.Done():
			return errors.New("任务已取消")
		default:
		}

		n, readErr := in.Read(buffer)
		if n > 0 {
			if _, err := out.Write(buffer[:n]); err != nil {
				return err
			}

			processed += int64(n)
			if total > 0 && onProgress != nil {
				percent := int(processed * 100 / total)
				if percent != lastPercent {
					lastPercent = percent
					onProgress(percent)
				}
			}
		}

		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}

	return out.Close()
}
