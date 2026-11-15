// Package scraper — универсальный движок: открывает страницы в Chrome (chromedp),
// собирает карточки товаров по селекторам из конфига и ходит по пагинации.
package scraper

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"

	"parser_shops/internal/config"
)

//go:embed extract.js
var extractJS string

type Product struct {
	Shop      string
	Category  string            `json:"category"`
	Name      string            `json:"name"`
	Price     string            `json:"price"`
	OldPrice  string            `json:"old_price"`
	Unit      string            `json:"unit"`
	Link      string            `json:"link"`
	Image     string            `json:"image"`
	Extra     map[string]string `json:"extra"`
	SourceURL string
}

// BlockedError — сайт показал заглушку/капчу из shop.blocks.
type BlockedError struct {
	Message string
	Manual  bool
}

func (e *BlockedError) Error() string { return e.Message }

type BrowserOptions struct {
	Headless bool
	Proxy    string // http://host:port, socks5://host:port
	Profile  string // папка профиля Chrome; пусто — временный
}

// NewBrowser запускает Chrome. cancel обязательно вызвать — иначе Chrome останется висеть.
func NewBrowser(o BrowserOptions) (context.Context, context.CancelFunc, error) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"),
		chromedp.Flag("headless", o.Headless),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("enable-automation", false),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("no-sandbox", true), // нужно в Docker
		chromedp.WindowSize(1920, 1080),
	)
	if o.Profile != "" {
		opts = append(opts, chromedp.UserDataDir(o.Profile))
	}
	if o.Proxy != "" {
		opts = append(opts, chromedp.ProxyServer(o.Proxy))
	}
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	quiet := func(string, ...interface{}) {}
	ctx, cancelCtx := chromedp.NewContext(allocCtx, chromedp.WithLogf(quiet), chromedp.WithErrorf(quiet))
	cancel := func() { cancelCtx(); cancelAlloc() }
	if err := chromedp.Run(ctx); err != nil {
		cancel()
		return nil, nil, fmt.Errorf("не удалось запустить Chrome (установлен ли Chrome/Chromium?): %w", err)
	}
	return ctx, cancel, nil
}

type Scraper struct {
	Browser  context.Context
	Headless bool
	Log      io.Writer
	// Сколько ждать на страницах пагинации (2-я и дальше), если страница загрузилась,
	// а карточек нет — значит, страницы кончились.
	EmptyAfter time.Duration
}

func (s *Scraper) logf(format string, a ...interface{}) {
	if s.Log != nil {
		fmt.Fprintf(s.Log, format, a...)
	}
}

// ScrapeShop обходит стартовые ссылки магазина со всеми страницами пагинации.
// maxPages > 0 переопределяет pagination.max_pages из конфига.
func (s *Scraper) ScrapeShop(shop *config.Shop, urls []string, maxPages int) ([]Product, error) {
	if maxPages <= 0 {
		maxPages = shop.Pagination.MaxPages
	}
	if shop.Pagination.Type == "none" {
		maxPages = 1
	}
	seen := map[string]bool{}
	var all []Product
	var lastErr error
	okURLs := 0
	for _, start := range urls {
		s.logf("🔍 [%s] %s\n", shop.Name, short(start))
		pageURL := start
		visited := map[string]bool{}
		for n := 0; n < maxPages && pageURL != ""; n++ {
			if visited[pageURL] {
				break
			}
			visited[pageURL] = true
			if n > 0 && shop.Delay.Duration > 0 {
				time.Sleep(shop.Delay.Duration)
			}
			products, next, err := s.scrapePage(shop, pageURL, n > 0)
			if err != nil {
				s.logf("   ❌ стр. %d: %v\n", n+1, err)
				lastErr = err
				var be *BlockedError
				if errors.As(err, &be) && !be.Manual {
					return all, err // IP-блок: дальше по этому магазину смысла нет
				}
				break
			}
			added := 0
			for _, p := range products {
				if p.Name == "" {
					continue
				}
				key := p.Link
				if key == "" {
					key = p.Name + "|" + p.Price
				}
				if seen[key] {
					continue
				}
				seen[key] = true
				p.Shop, p.SourceURL = shop.Name, pageURL
				all = append(all, p)
				added++
			}
			s.logf("   ✅ стр. %d: +%d товаров\n", n+1, added)
			if n == 0 {
				okURLs++
			}
			if added == 0 {
				break // страница повторяет предыдущие или пуста — пагинация кончилась
			}
			pageURL = nextPage(shop, start, n+1, next)
		}
	}
	if okURLs == 0 && lastErr != nil {
		return all, lastErr
	}
	return all, nil
}

// nextPage возвращает адрес страницы с индексом idx (0 — стартовая).
func nextPage(shop *config.Shop, start string, idx int, nextHref string) string {
	switch shop.Pagination.Type {
	case "next":
		return nextHref
	case "param":
		u, err := url.Parse(start)
		if err != nil {
			return ""
		}
		q := u.Query()
		q.Set(shop.Pagination.Param, strconv.Itoa(shop.Pagination.Start+idx))
		u.RawQuery = q.Encode()
		return u.String()
	}
	return ""
}

// ScrapeFile разбирает сохранённую HTML-страницу (без пагинации и без JS страницы).
func (s *Scraper) ScrapeFile(shop *config.Shop, fileURL string) ([]Product, error) {
	products, _, err := s.scrapePage(shop, fileURL, false)
	for i := range products {
		products[i].Shop, products[i].SourceURL = shop.Name, fileURL
	}
	return products, err
}

func (s *Scraper) scrapePage(shop *config.Shop, target string, isNextPage bool) ([]Product, string, error) {
	tabCtx, cancelTab := chromedp.NewContext(s.Browser)
	defer cancelTab()
	ctx, cancel := context.WithTimeout(tabCtx, shop.Timeout.Duration)
	defer cancel()

	local := strings.HasPrefix(target, "file:")
	if local {
		// Сохранённая страница: её собственный JS перерисовывает DOM — отключаем.
		if err := chromedp.Run(ctx, emulation.SetScriptExecutionDisabled(true)); err != nil {
			return nil, "", err
		}
	}
	// page.Navigate не ждёт события load (оно может висеть на сторонних ресурсах),
	// готовность определяем сами — по появлению карточек.
	if err := chromedp.Run(ctx, chromedp.ActionFunc(func(c context.Context) error {
		_, _, errText, err := page.Navigate(target).Do(c)
		if err == nil && errText != "" {
			err = errors.New(errText)
		}
		return err
	})); err != nil {
		return nil, "", fmt.Errorf("загрузка: %w", err)
	}

	cards, err := s.waitCards(ctx, shop, target, isNextPage)
	if err != nil || cards == 0 {
		return nil, "", err
	}

	if shop.Scroll && !local {
		prev := -1
		for i := 0; i < 40; i++ {
			var n int
			if err := chromedp.Run(ctx,
				chromedp.Evaluate(`window.scrollTo(0, document.body.scrollHeight); document.querySelectorAll(`+jsString(shop.Selectors.Card)+`).length`, &n),
				chromedp.Sleep(1500*time.Millisecond),
			); err != nil || n == prev {
				break
			}
			prev = n
		}
	}

	cfg, _ := json.Marshal(map[string]interface{}{
		"card": shop.Selectors.Card, "name": shop.Selectors.Name, "price": shop.Selectors.Price,
		"old_price": shop.Selectors.OldPrice, "unit": shop.Selectors.Unit, "link": shop.Selectors.Link,
		"image": shop.Selectors.Image, "remove": shop.Selectors.Remove, "extra": shop.Selectors.Extra,
		"next":     shop.Pagination.Next,
		"category": map[string]string{"selector": shop.Category.Selector, "title_split": shop.Category.TitleSplit, "fixed": shop.Category.Fixed},
	})
	var res struct {
		Products []Product `json:"products"`
		Next     string    `json:"next"`
	}
	if err := chromedp.Run(ctx, chromedp.Evaluate(extractJS+"("+string(cfg)+")", &res)); err != nil {
		return nil, "", fmt.Errorf("извлечение: %w", err)
	}
	return res.Products, res.Next, nil
}

// waitCards опрашивает страницу, пока не появятся карточки или заглушка из shop.blocks.
func (s *Scraper) waitCards(ctx context.Context, shop *config.Shop, target string, isNextPage bool) (int, error) {
	probe := `(() => ({
		n: document.querySelectorAll(` + jsString(shop.Selectors.Card) + `).length,
		ready: document.readyState,
		t: (document.body ? document.body.innerText : "").replace(/\s+/g, " ").slice(0, 3000)
	}))()`
	var lastText string
	var emptySince time.Time
	manualShown := false
	for {
		var st struct {
			N     int    `json:"n"`
			Ready string `json:"ready"`
			T     string `json:"t"`
		}
		if err := chromedp.Run(ctx, chromedp.Evaluate(probe, &st)); err != nil {
			hint := ""
			if lastText != "" {
				hint = "\n      на странице: " + short(lastText)
			}
			return 0, fmt.Errorf("карточки (%s) не появились за %s (%v)%s", shop.Selectors.Card, shop.Timeout.Duration, err, hint)
		}
		lastText = st.T
		if st.N > 0 {
			return st.N, nil
		}
		for _, b := range shop.Blocks {
			if !b.Re.MatchString(st.T) {
				continue
			}
			if !b.Manual || s.Headless {
				return 0, &BlockedError{Message: b.Message, Manual: b.Manual}
			}
			if !manualShown {
				s.logf("   🧩 %s — пройдите проверку в окне браузера, парсер подождёт (до %s)…\n", b.Message, shop.Timeout.Duration)
				manualShown = true
			}
		}
		// Страница пагинации загрузилась, но пустая — значит, страницы кончились.
		if isNextPage && st.Ready == "complete" && !manualShown {
			if emptySince.IsZero() {
				emptySince = time.Now()
			} else if time.Since(emptySince) > s.emptyAfter() {
				return 0, nil
			}
		}
		if strings.HasPrefix(target, "file:") && st.Ready == "complete" {
			return 0, fmt.Errorf("в файле нет карточек по селектору %s", shop.Selectors.Card)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func (s *Scraper) emptyAfter() time.Duration {
	if s.EmptyAfter > 0 {
		return s.EmptyAfter
	}
	return 8 * time.Second
}

func jsString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func short(s string) string {
	if r := []rune(s); len(r) > 110 {
		return string(r[:110]) + "…"
	}
	return s
}
