package httpclient

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"time"
)

// NewClient создает новый HTTP-клиент с поддержкой cookie и прокси.
// proxyURLStr - это строка URL прокси, например "http://user:pass@host:port".
// Если proxyURLStr пустая, клиент будет работать без прокси.
func NewClient(proxyURLStr string) (*http.Client, error) {
	// Создаем хранилище для cookie, чтобы клиент мог автоматически
	// управлять сессиями.
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	// Настраиваем транспорт.
	transport := &http.Transport{}

	// Если указан URL прокси, парсим его и добавляем в транспорт.
	if proxyURLStr != "" {
		proxyURL, err := url.Parse(proxyURLStr)
		if err != nil {
			return nil, err
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	// Создаем и возвращаем клиент.
	client := &http.Client{
		Jar:       jar,
		Transport: transport,
		Timeout:   30 * time.Second, // Устанавливаем таймаут для запросов.
	}

	return client, nil
}
