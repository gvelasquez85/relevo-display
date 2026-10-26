//go:build !windows || !amd64

package jpegfast

func EncodeBGRA(pixels []byte, width, height, stride, quality int) ([]byte, error) {
	if err := validate(pixels, width, height, stride, quality); err != nil {
		return nil, err
	}
	return nil, ErrUnavailable
}

// Shutdown is a no-op where no native encoder is loaded.
func Shutdown() error { return nil }
