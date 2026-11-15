package scraper

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
	"time"

	"parser_shops/internal/config"
)

// Тестовый «магазин»: 3 страницы по 2 товара, дальше — пустая страница.
func testServer(lastPageRepeats bool) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if p == 0 {
			p = 1
		}
		if p > 3 && lastPageRepeats {
			p = 3 // некоторые сайты на «лишних» страницах отдают последнюю
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<html><head><title>Тест в Магазине</title></head><body><h1>Каталог</h1>`)
		if p <= 3 {
			for i := 1; i <= 2; i++ {
				id := (p-1)*2 + i
				fmt.Fprintf(w, `<div class="card"><a class="t" href="/item/%d" title="Товар %d полностью">Товар %d…</a>
					<span class="price"><i class="sr">Цена</i>1 %03d,50 ₽</span><b class="sku">SKU-%d</b></div>`, id, id, id, id, id)
			}
			if p < 3 {
				fmt.Fprintf(w, `<a class="next" href="/?page=%d">дальше</a>`, p+1)
			}
		} else {
			fmt.Fprint(w, `<p>Ничего не найдено</p>`)
		}
		fmt.Fprint(w, `</body></html>`)
	}))
}

func testShop(pagType string) *config.Shop {
	return &config.Shop{
		Name:    "test",
		Timeout: config.Duration{Duration: 15 * time.Second},
		Selectors: config.Selectors{
			Card: "div.card", Name: "a.t@title", Price: ".price", Link: "a.t", Remove: ".sr",
			Extra: map[string]string{"sku": ".sku"},
		},
		Category:   config.Category{TitleSplit: " в "},
		Pagination: config.Pagination{Type: pagType, Param: "page", Start: 1, Next: "a.next", MaxPages: 10},
	}
}

func newTestScraper(t *testing.T) *Scraper {
	t.Helper()
	ctx, cancel, err := NewBrowser(BrowserOptions{Headless: true})
	if err != nil {
		t.Skip("Chrome недоступен:", err)
	}
	t.Cleanup(cancel)
	return &Scraper{Browser: ctx, Headless: true, EmptyAfter: time.Second}
}

func TestPagination(t *testing.T) {
	sc := newTestScraper(t)
	cases := []struct {
		name    string
		pagType string
		repeats bool
	}{
		{"param, пустая страница в конце", "param", false},
		{"param, последняя страница повторяется", "param", true},
		{"next-ссылка", "next", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := testServer(c.repeats)
			defer srv.Close()
			got, err := sc.ScrapeShop(testShop(c.pagType), []string{srv.URL + "/"}, 0)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 6 {
				t.Fatalf("ожидали 6 товаров, получили %d: %+v", len(got), got)
			}
			p := got[0]
			if p.Name != "Товар 1 полностью" || p.Price != "1 001,50 ₽" || p.Category != "Тест" ||
				p.Link != srv.URL+"/item/1" || p.Extra["sku"] != "SKU-1" || p.Shop != "test" {
				t.Fatalf("неверные поля: %+v", p)
			}
		})
	}
}

func TestBlocked(t *testing.T) {
	sc := newTestScraper(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<html><body>Отключите VPN</body></html>`)
	}))
	defer srv.Close()
	shop := testShop("param")
	shop.Blocks = []config.Block{{Match: "Отключите VPN", Message: "IP заблокирован", Re: regexp.MustCompile("Отключите VPN")}}
	_, err := sc.ScrapeShop(shop, []string{srv.URL}, 0)
	be, ok := err.(*BlockedError)
	if !ok || be.Message != "IP заблокирован" {
		t.Fatalf("ожидали BlockedError, получили %v", err)
	}
}

func TestConfigFileLoads(t *testing.T) {
	path := filepath.Join("..", "..", "shops.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Skip("нет shops.yaml")
	}
	if _, err := config.Load(path); err != nil {
		t.Fatal(err)
	}
}
