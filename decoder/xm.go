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

	outFile, _ := os.Create(rawPath)
	defer outFile.Close()

	buffer := make([]byte, 64*1024)

	for {
		select {
		case <-ctx.Done():
			os.Remove(rawPath)
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

			outFile.Write(data)
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}

	finalPath, err := converter.FinishAudio(
		rawPath,
		outputDir,
		name,
		outputFormat,
		bitrate,
	)

	if err != nil {
		return nil, err
	}

	return &DecodeResult{
		InputPath:  inputPath,
		OutputPath: finalPath,
		Platform:   d.Name(),
		Success:    true,
		Message:    "XM 解密尝试完成（可能失败）",
	}, nil
}
