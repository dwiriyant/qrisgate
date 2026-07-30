package qris

import (
	"encoding/base64"

	qrcode "github.com/skip2/go-qrcode"
)

// PNGBase64 renders payload as a QR code PNG encoded in base64 (no data: prefix).
func PNGBase64(payload string, size int) (string, error) {
	if size <= 0 {
		size = 256
	}
	png, err := qrcode.Encode(payload, qrcode.Medium, size)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(png), nil
}
