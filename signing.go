package gotopo

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
)

func signRequest(key, method, path string, expires int64, payload []byte) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return "", fmt.Errorf("decode signing key: %w", err)
	}
	data := strings.ToUpper(method) + " " + path + "\n" + strconv.FormatInt(expires, 10) + "\n"
	if strings.EqualFold(method, "POST") {
		data += string(payload)
	}
	mac := hmac.New(sha256.New, decoded)
	_, _ = mac.Write([]byte(data))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil)), nil
}
