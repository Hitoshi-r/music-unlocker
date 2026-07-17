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

type XMDecoder struct{}

func (d *XMDecoder) Name() string {
	return "虾米音乐"
}

func (d *XMDecoder) Extensions() []string {
	return []string{".xm"}
}

func (d *XMDecoder) Decode(
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

	file, err := os.Open(inputPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	if outputDir == "" {
		outputDir = filepath.Dir(inputPath)
	}

	os.MkdirAll(outputDir, 0755)

	name := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	rawPath := filepath.Join(outputDir, name+"_xm_raw.bin")

	outFile, err := os.Create(rawPath)
	if err != nil {
		return nil, err
	}
	completed := false
	defer func() {
		_ = outFile.Close()
		if !completed {
			_ = os.Remove(rawPath)
		}
	}()

	buffer := make([]byte, 64*1024)

	for {
		select {
		case <-ctx.Done():
			return nil, errors.New("任务已取消")
		default:
		}

		n, err := file.Read(buffer)

		if n > 0 {
			data := buffer[:n]

			// ⚠️ 简单尝试 XOR（可能无效）
			for i := 0; i < n; i++ {
				data[i] ^= 0xA3
			}

			if _, err := outFile.Write(data); err != nil {
				return nil, err
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	if err := outFile.Close(); err != nil {
		return nil, err
	}

	finalPath, err := converter.FinishAudio(
		ctx,
		rawPath,
		outputDir,
		name,
		outputFormat,
		bitrate,
	)

	if err != nil {
		return nil, err
	}
	completed = true

	return &DecodeResult{
		InputPath:  inputPath,
		OutputPath: finalPath,
		Platform:   d.Name(),
		Success:    true,
		Message:    "XM 解密尝试完成（可能失败）",
	}, nil
}
