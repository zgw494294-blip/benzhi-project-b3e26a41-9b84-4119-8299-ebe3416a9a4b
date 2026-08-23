package workflow

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

type IDGenerator func(prefix string) string

func RandomID(prefix string) string {
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err != nil {
		panic(fmt.Sprintf("生成安全随机 ID 失败: %v", err))
	}
	return prefix + "_" + hex.EncodeToString(buffer)
}
