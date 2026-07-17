//go:build !windows

package decoder

import (
	"context"
	"errors"
)

func getLocalQQMusicCredentials(context.Context) (qqLocalCredentials, error) {
	return qqLocalCredentials{}, errors.New("自动读取本机 QQ 音乐登录状态目前仅支持 Windows")
}
