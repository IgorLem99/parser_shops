package export

import "testing"

func TestParsePrice(t *testing.T) {
	cases := map[string]float64{
		"369,99 ₽":         369.99,
		"1 369,99 ₽":       1369.99,
		"£51.77":           51.77,
		"$1,234.50":        1234.5,
		"от 1.234.567 р.":  1234567,
		"389,00 ₽ за 1 кг": 389,
	}
	for in, want := range cases {
		if got, ok := ParsePrice(in); !ok || got != want {
			t.Errorf("ParsePrice(%q) = %v, %v; want %v", in, got, ok, want)
		}
	}
	if _, ok := ParsePrice("нет цены"); ok {
		t.Error("ожидали ok=false")
	}
}
