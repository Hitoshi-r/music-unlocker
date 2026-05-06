package decoder

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type MFLACDecoder struct{}

func NewMFLACDecoder() *MFLACDecoder {
	return &MFLACDecoder{}
}

func (d *MFLACDecoder) Name() string {
	return "QQ音乐新版"
}

func (d *MFLACDecoder) Extensions() []string {
	return []string{
		".mflac",
		".mgg",
	}
}

func (d *MFLACDecoder) Decode(
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

	ext := strings.ToLower(filepath.Ext(inputPath))

	switch ext {
	case ".mflac":
		return d.decodeMFLAC(inputPath, outputDir, outputFormat, bitrate, ctx, onProgress)

	case ".mgg":
		return d.decodeMGG(inputPath, outputDir, outputFormat, bitrate, ctx, onProgress)

	default:
		return nil, errors.New("未知 QQ 新版格式")
	}
}

func (d *MFLACDecoder) decodeMFLAC(
	inputPath string,
	outputDir string,
	outputFormat string,
	bitrate string,
	ctx context.Context,
	onProgress ProgressCallback,
) (*DecodeResult, error) {

	file, err := os.Open(inputPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	size := info.Size()

	header := make([]byte, 32)

	_, err = file.Read(header)
	if err != nil {
		return nil, err
	}

	fmt.Println("===== mflac 文件头 HEX =====")
	fmt.Println(hex.EncodeToString(header))

	tailSize := int64(512)
	if size < tailSize {
		tailSize = size
	}

	tail := make([]byte, tailSize)

	_, err = file.ReadAt(tail, size-tailSize)
	if err != nil {
		return nil, err
	}

	fmt.Println("===== mflac 文件尾 TEXT =====")
	fmt.Println(string(tail))

	if onProgress != nil {
		onProgress(100)
	}

	return nil, errors.New("mflac 文件分析完成，请查看终端输出")
}

func (d *MFLACDecoder) decodeMGG(
	inputPath string,
	outputDir string,
	outputFormat string,
	bitrate string,
	ctx context.Context,
	onProgress ProgressCallback,
) (*DecodeResult, error) {

	if onProgress != nil {
		onProgress(100)
	}

	return nil, errors.New("mgg 解密入口已建立，但解密核心尚未实现")
}
