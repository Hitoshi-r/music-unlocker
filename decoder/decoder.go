package decoder

import "context"

type DecodeResult struct {
	InputPath  string
	OutputPath string
	Platform   string
	Success    bool
	Message    string
}

type ProgressCallback func(percent int)

type Decoder interface {
	Name() string
	Extensions() []string
	Decode(
		inputPath string,
		outputDir string,
		outputFormat string,
		bitrate string,
		ctx context.Context,
		onProgress ProgressCallback,
	) (*DecodeResult, error)
}
