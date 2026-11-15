// Package export пишет товары в CSV (разделитель ';', UTF-8 с BOM — открывается в Excel).
package export

import (
	"encoding/csv"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"parser_shops/internal/scraper"
)

func WriteCSV(path string, products []scraper.Product) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.WriteString("\xEF\xBB\xBF"); err != nil {
		return err
	}

	// Дополнительные колонки — объединение всех extra-полей.
	extraSet := map[string]bool{}
	for _, p := range products {
		for k := range p.Extra {
			extraSet[k] = true
		}
	}
	var extra []string
	for k := range extraSet {
		extra = append(extra, k)
	}
	sort.Strings(extra)

	w := csv.NewWriter(f)
	w.Comma = ';'
	header := []string{"Магазин", "Категория", "Наименование", "Цена", "Старая цена", "Цена (текст)", "Цена за ед.", "Ссылка", "Картинка"}
	w.Write(append(header, extra...))
	for _, p := range products {
		row := []string{
			p.Shop, p.Category, p.Name,
			num(p.Price), num(p.OldPrice), p.Price, p.Unit, p.Link, p.Image,
		}
		for _, k := range extra {
			row = append(row, p.Extra[k])
		}
		w.Write(row)
	}
	w.Flush()
	return w.Error()
}

var numRe = regexp.MustCompile(`\d[\d\s\x{00a0}\x{202f}]*(?:[.,]\d+)*`)

// ParsePrice: "1 369,99 ₽" → 1369.99, "£51.77" → 51.77, "1,234.50" → 1234.5.
func ParsePrice(s string) (float64, bool) {
	m := numRe.FindString(s)
	if m == "" {
		return 0, false
	}
	m = strings.NewReplacer(" ", "", " ", "", " ", "").Replace(m)
	if strings.Contains(m, ",") && strings.Contains(m, ".") {
		m = strings.ReplaceAll(m, ",", "") // 1,234.50
	} else {
		m = strings.ReplaceAll(m, ",", ".") // 369,99
	}
	if strings.Count(m, ".") > 1 { // 1.234.567 — точки как разделители тысяч
		m = strings.ReplaceAll(m, ".", "")
	}
	v, err := strconv.ParseFloat(m, 64)
	return v, err == nil
}

func num(s string) string {
	v, ok := ParsePrice(s)
	if !ok {
		return ""
	}
	return strconv.FormatFloat(v, 'f', 2, 64)
}
