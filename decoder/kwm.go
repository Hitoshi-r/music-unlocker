package decoder

import (
	"context"
	"crypto/aes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"music-unlocker/converter"
)

type KWMDecoder struct{}

func (d *KWMDecoder) Name() string {
	return "酷我音乐"
}

func (d *KWMDecoder) Extensions() []string {
	return []string{".kwm"}
}

// 固定 key（常见）
var kwmKey = []byte("0123456789abcdef")

func (d *KWMDecoder) Decode(
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
	rawPath := filepath.Join(outputDir, name+"_kwm_raw.bin")

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

	// 跳过头部（常见）
	file.Seek(0x400, io.SeekStart)

	block, err := aes.NewCipher(kwmKey)
	if err != nil {
		return nil, err
	}

	buffer := make([]byte, 16)

	for {
		select {
		case <-ctx.Done():
			return nil, errors.New("任务已取消")
		default:
		}

		n, err := file.Read(buffer)

		if n > 0 {
			data := make([]byte, n)
			copy(data, buffer[:n])

			// AES 解密（按块）
			if n == 16 {
				block.Decrypt(data, data)
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
		Message:    "KWM 解密完成",
	}, nil
}
