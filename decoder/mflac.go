package decoder

import (
	"context"
	"errors"
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
	return []string{".mflac", ".mgg"}
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

	switch strings.ToLower(filepath.Ext(inputPath)) {
	case ".mflac", ".mgg":
		if onProgress != nil {
			onProgress(100)
		}
		return nil, errors.New("该文件属于新版受保护格式，当前版本仅支持识别，不提供解密绕过")
	default:
		return nil, errors.New("未知 QQ 新版格式")
	}
}
