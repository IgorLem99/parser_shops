package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"parser_shops/internal/models"
	"parser_shops/pkg/httpclient"
)

const (
	// StoreID для поиска категорий (из задания)
	StoreID = "135161"
	// URL для получения дерева категорий
	CategoriesURL = "https://kuper.ru/api/v2/stores/%s/categories"
)

func main() {
	// 1. Настройка прокси (берем из окружения, как в основном парсере)
	proxyURL := os.Getenv("PROXY_URL")
	client, err := httpclient.NewClient(proxyURL)
	if err != nil {
		log.Fatalf("Ошибка создания клиента: %v", err)
	}

	// 2. Запрос категорий
	url := fmt.Sprintf(CategoriesURL, StoreID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	// Актуальный токен из ваших HTML файлов
	req.Header.Set("client-token", "aab83ff35c0a329fa775ae68f13192c0")
	req.Header.Set("Referer", "https://kuper.ru/")
	req.Header.Set("Origin", "https://kuper.ru")
	req.Header.Set("Store-Id", StoreID)
	req.Header.Set("sbermarket-store-id", StoreID)
	req.Header.Set("lat", "55.7558")
	req.Header.Set("lon", "37.6173")
	req.Header.Set("sec-ch-ua", `"Chromium";v="122", "Not(A:Brand";v="24", "Google Chrome";v="122"`)
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-platform", `"Windows"`)
	req.Header.Set("sec-fetch-dest", "empty")
	req.Header.Set("sec-fetch-mode", "cors")
	req.Header.Set("sec-fetch-site", "same-origin")

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Ошибка запроса: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Fatalf("Ошибка API: статус %d\nТело ответа: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	var taxons []models.Taxon
	if err := json.Unmarshal(body, &taxons); err != nil {
		log.Fatalf("Ошибка парсинга JSON: %v\nДлина ответа: %d\nТело (первые 500 символов): %s", err, len(body), string(body))
	}

	// 3. Вывод дерева
	fmt.Printf("Дерево категорий для магазина %s:\n", StoreID)
	for _, t := range taxons {
		printTaxon(t, 0)
	}
}

func printTaxon(t models.Taxon, level int) {
	indent := strings.Repeat("  ", level)
	fmt.Printf("%s[%d] %s (slug: %s)\n", indent, t.ID, t.Name, t.Slug)
	for _, child := range t.Children {
		printTaxon(child, level+1)
	}
}
