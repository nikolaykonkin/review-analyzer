package gigachat

import (
	"crypto/rand"
	"fmt"
)

// newRqUID генерирует UUID v4 для заголовка RqUID
// Используется crypto/rand, чтобы избежать зависимости от внешних библиотек
func newRqUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("генерация RqUID: %w", err)
	}
	// Устанавливаем версию (4) и вариант (10xx) согласно RFC 4122
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}
