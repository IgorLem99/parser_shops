package models

// KuperResponse представляет ответ от API Kuper (SberMarket).
type KuperResponse struct {
	Products []Product `json:"products"`
	Meta     Meta      `json:"meta"`
}

type Meta struct {
	TotalCount  interface{} `json:"total_count"`
	TotalPages  interface{} `json:"total_pages"`
	CurrentPage interface{} `json:"current_page"`
}

// Product представляет отдельный товар в ответе.
type Product struct {
	ID            int64   `json:"id"`
	Name          string  `json:"name"`
	Slug          string  `json:"slug"`
	CanonicalSlug string  `json:"canonical_slug,omitempty"`
	Permalink     string  `json:"permalink,omitempty"`
	Price         float64 `json:"price"`
	OldPrice      float64 `json:"original_price,omitempty"`
	Discount      float64 `json:"discount,omitempty"`
	WebURL        string  `json:"web_url,omitempty"` // Мы сформируем это поле сами или возьмем из API если есть
}

// Taxon представляет категорию в дереве каталога.
type Taxon struct {
	ID       int64   `json:"id"`
	Name     string  `json:"name"`
	Slug     string  `json:"slug"`
	Children []Taxon `json:"children,omitempty"`
}
