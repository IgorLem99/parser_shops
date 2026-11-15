// Package config описывает магазины: откуда брать товары и как их вытаскивать со страницы.
// Новый магазин добавляется блоком в shops.yaml, без правки кода.
package config

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Shops []Shop `yaml:"shops"`
}

type Shop struct {
	Name string   `yaml:"name"`
	URLs []string `yaml:"urls"`

	// Ожидание карточек на одной странице.
	Timeout Duration `yaml:"timeout"`
	// Пауза между страницами пагинации (вежливость к сайту).
	Delay Duration `yaml:"delay"`
	// Прокручивать страницу вниз, пока подгружаются карточки (ленивая загрузка).
	Scroll bool `yaml:"scroll"`

	Selectors  Selectors  `yaml:"selectors"`
	Category   Category   `yaml:"category"`
	Pagination Pagination `yaml:"pagination"`
	// Признаки заглушек/блокировок: текст страницы → понятная ошибка.
	Blocks []Block `yaml:"blocks"`
}

// Selectors — CSS-селекторы относительно карточки товара.
// Поле можно взять из атрибута: "h3.title@title" (если атрибут пуст — берётся текст).
type Selectors struct {
	Card     string            `yaml:"card"`
	Name     string            `yaml:"name"`
	Price    string            `yaml:"price"`
	OldPrice string            `yaml:"old_price"`
	Unit     string            `yaml:"unit"`
	Link     string            `yaml:"link"`
	Image    string            `yaml:"image"`
	Remove   string            `yaml:"remove"` // что вырезать из текста полей (скрытые подписи и т.п.)
	Extra    map[string]string `yaml:"extra"`  // любые дополнительные колонки
}

type Category struct {
	Selector   string `yaml:"selector"`    // CSS-селектор на странице, напр. "h1"
	TitleSplit string `yaml:"title_split"` // взять часть <title> до этого разделителя
	Fixed      string `yaml:"fixed"`       // или задать вручную
}

type Pagination struct {
	// none | param | next
	Type     string `yaml:"type"`
	Param    string `yaml:"param"`     // для type=param: имя параметра, напр. page
	Start    int    `yaml:"start"`     // номер первой страницы (по умолчанию 1)
	Next     string `yaml:"next"`      // для type=next: селектор ссылки «Следующая»
	MaxPages int    `yaml:"max_pages"` // предохранитель
}

type Block struct {
	Match   string `yaml:"match"`   // регулярное выражение по тексту страницы
	Message string `yaml:"message"` // что сказать пользователю
	Manual  bool   `yaml:"manual"`  // можно пройти руками в окне браузера (капча)

	Re *regexp.Regexp `yaml:"-"`
}

// Duration читается из YAML как "30s", "2m".
type Duration struct{ time.Duration }

func (d *Duration) UnmarshalYAML(n *yaml.Node) error {
	v, err := time.ParseDuration(n.Value)
	if err != nil {
		return fmt.Errorf("строка %d: неверная длительность %q", n.Line, n.Value)
	}
	d.Duration = v
	return nil
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if len(c.Shops) == 0 {
		return nil, fmt.Errorf("%s: нет ни одного магазина в shops", path)
	}
	seen := map[string]bool{}
	for i := range c.Shops {
		s := &c.Shops[i]
		if err := s.normalize(); err != nil {
			return nil, fmt.Errorf("%s: магазин %q: %w", path, s.Name, err)
		}
		if seen[s.Name] {
			return nil, fmt.Errorf("%s: магазин %q указан дважды", path, s.Name)
		}
		seen[s.Name] = true
	}
	return &c, nil
}

func (s *Shop) normalize() error {
	if s.Name == "" {
		return fmt.Errorf("не задано name")
	}
	if s.Selectors.Card == "" || s.Selectors.Name == "" {
		return fmt.Errorf("обязательны selectors.card и selectors.name")
	}
	if s.Timeout.Duration == 0 {
		s.Timeout.Duration = 60 * time.Second
	}
	p := &s.Pagination
	switch p.Type {
	case "", "none":
		p.Type = "none"
	case "param":
		if p.Param == "" {
			return fmt.Errorf("pagination.type=param требует pagination.param")
		}
	case "next":
		if p.Next == "" {
			return fmt.Errorf("pagination.type=next требует pagination.next")
		}
	default:
		return fmt.Errorf("pagination.type: ожидается none|param|next, получено %q", p.Type)
	}
	if p.Start == 0 {
		p.Start = 1
	}
	if p.MaxPages == 0 {
		p.MaxPages = 50
	}
	for i := range s.Blocks {
		re, err := regexp.Compile(s.Blocks[i].Match)
		if err != nil {
			return fmt.Errorf("blocks[%d].match: %w", i, err)
		}
		s.Blocks[i].Re = re
	}
	return nil
}

func (c *Config) Find(name string) (*Shop, error) {
	for i := range c.Shops {
		if c.Shops[i].Name == name {
			return &c.Shops[i], nil
		}
	}
	names := make([]string, len(c.Shops))
	for i, s := range c.Shops {
		names[i] = s.Name
	}
	return nil, fmt.Errorf("магазин %q не найден, есть: %v", name, names)
}
