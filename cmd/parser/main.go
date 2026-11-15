// Универсальный парсер магазинов: что и как собирать — описано в shops.yaml.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"parser_shops/internal/config"
	"parser_shops/internal/export"
	"parser_shops/internal/scraper"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "❌", err)
		os.Exit(1)
	}
}

func run() error {
	cfgPath := flag.String("config", envOr("SHOPS_CONFIG", "shops.yaml"), "файл с описанием магазинов (или SHOPS_CONFIG)")
	shopsFlag := flag.String("shop", os.Getenv("SHOP"), "какие магазины парсить через запятую (по умолчанию все; или SHOP)")
	urlsFlag := flag.String("urls", os.Getenv("TARGET_URLS"), "свои ссылки вместо urls из конфига (нужен ровно один -shop; или TARGET_URLS)")
	filesFlag := flag.String("files", "", "офлайн: разобрать сохранённые HTML по шаблону, напр. \"*.html\" (селекторы из -shop)")
	maxPages := flag.Int("max-pages", 0, "ограничить число страниц пагинации (0 — как в конфиге)")
	proxy := flag.String("proxy", os.Getenv("PROXY_URL"), "прокси для Chrome: http://host:port или socks5://host:port (или PROXY_URL)")
	headless := flag.Bool("headless", os.Getenv("HEADLESS") != "0", "без окна браузера (-headless=false или HEADLESS=0 — с окном, напр. для капчи)")
	profile := flag.String("profile", ".chrome-profile", "папка профиля Chrome (cookies сохраняются между запусками); пусто — временный")
	out := flag.String("out", "", "файл CSV (по умолчанию products_ДАТА_ВРЕМЯ.csv)")
	list := flag.Bool("list", false, "показать магазины из конфига и выйти")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	if *list {
		for _, s := range cfg.Shops {
			fmt.Printf("%-20s пагинация: %-5s ссылок: %d\n", s.Name, s.Pagination.Type, len(s.URLs))
		}
		return nil
	}

	shops := cfg.Shops
	if *shopsFlag != "" {
		shops = nil
		for _, name := range splitList(*shopsFlag) {
			s, err := cfg.Find(name)
			if err != nil {
				return err
			}
			shops = append(shops, *s)
		}
	}
	customURLs := splitList(*urlsFlag)
	if (len(customURLs) > 0 || *filesFlag != "") && len(shops) != 1 {
		if *filesFlag != "" && *shopsFlag == "" {
			shops = shops[:1] // для офлайна по умолчанию — первый магазин
		} else {
			return errors.New("-urls и -files работают с одним магазином: укажите -shop")
		}
	}

	var profileDir string
	if *profile != "" {
		profileDir, _ = filepath.Abs(*profile)
	}
	browser, closeBrowser, err := scraper.NewBrowser(scraper.BrowserOptions{Headless: *headless, Proxy: *proxy, Profile: profileDir})
	if err != nil {
		return err
	}
	defer closeBrowser()

	sc := &scraper.Scraper{Browser: browser, Headless: *headless, Log: os.Stdout}
	var all []scraper.Product
	var failed []string

	if *filesFlag != "" {
		matches, _ := filepath.Glob(*filesFlag)
		if len(matches) == 0 {
			return fmt.Errorf("не найдено файлов по шаблону %q", *filesFlag)
		}
		shop := &shops[0]
		for _, m := range matches {
			abs, _ := filepath.Abs(m)
			fmt.Printf("📄 [%s] %s\n", shop.Name, filepath.Base(m))
			products, err := sc.ScrapeFile(shop, "file:///"+filepath.ToSlash(abs))
			if err != nil {
				fmt.Printf("   ❌ %v\n", err)
				continue
			}
			fmt.Printf("   ✅ товаров: %d\n", len(products))
			all = append(all, products...)
		}
	} else {
		fmt.Printf("🚀 Магазинов: %d\n", len(shops))
		for i := range shops {
			shop := &shops[i]
			urls := shop.URLs
			if len(customURLs) > 0 {
				urls = customURLs
			}
			if len(urls) == 0 {
				fmt.Printf("⚠️  [%s] нет ссылок (urls)\n", shop.Name)
				continue
			}
			products, err := sc.ScrapeShop(shop, urls, *maxPages)
			if err != nil {
				failed = append(failed, fmt.Sprintf("%s: %v", shop.Name, err))
			}
			fmt.Printf("📦 [%s] итого: %d\n", shop.Name, len(products))
			all = append(all, products...)
		}
	}

	if len(all) == 0 {
		if len(failed) > 0 {
			return fmt.Errorf("ничего не собрано:\n   %s", strings.Join(failed, "\n   "))
		}
		return errors.New("ничего не собрано")
	}
	if *out == "" {
		*out = fmt.Sprintf("products_%s.csv", time.Now().Format("20060102_150405"))
	}
	if err := export.WriteCSV(*out, all); err != nil {
		return fmt.Errorf("запись CSV: %w", err)
	}
	fmt.Printf("🏁 Готово: %d товаров → %s\n", len(all), *out)
	for _, f := range failed {
		fmt.Printf("⚠️  с ошибкой: %s\n", f)
	}
	return nil
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
