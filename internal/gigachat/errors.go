package gigachat

import "errors"

var (
	// ErrUnauthorized означает отказ в авторизации (401 или 403)
	ErrUnauthorized = errors.New("отказ в авторизации gigachat")
	// ErrTokenResponse означает невалидный ответ от сервера токенов
	ErrTokenResponse = errors.New("невалидный ответ сервера токенов")
	// ErrCAFile означает проблему с чтением или разбором файла сертификата
	ErrCAFile = errors.New("ошибка сертификата")
)
