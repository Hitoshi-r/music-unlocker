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

type KGMDecoder struct{}

func (d *KGMDecoder) Name() string {
	return "酷狗音乐"
}

func (d *KGMDecoder) Extensions() []string {
	return []string{".kgm", ".vpr"}
}

// 简化 key（演示用）
var kgmKey = []byte{
	0x6A, 0x6B, 0x6C, 0x31, 0x32, 0x33, 0x34, 0x35,
}

func (d *KGMDecoder) Decode(
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
	rawPath := filepath.Join(outputDir, name+"_kgm_raw.bin")

	outFile, _ := os.Create(rawPath)
	defer outFile.Close()

	buffer := make([]byte, 64*1024)
	var processed int64

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

			// 🔥 简单 XOR 解密
			for i := 0; i < n; i++ {
				data[i] ^= kgmKey[(int(processed)+i)%len(kgmKey)]
			}

			outFile.Write(data)
			processed += int64(n)
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
		Message:    "KGM 解密完成",
	}, nil
}
