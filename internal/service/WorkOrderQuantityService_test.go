// --------- новый код chat: точные значения и границы ввода
package service

import "testing"

func TestWorkQuantities(t *testing.T) {
	for _, c := range []struct {
		text   string
		digits int
		want   int64
	}{{"0,125", 3, 125}, {"1.25", 3, 1250}, {"12.5", 2, 1250}, {"0", 2, 0}, {"999999999.999", 3, 999999999999}} {
		got, err := ParseWorkQuantity(c.text, c.digits, true)
		if err != nil || got != c.want {
			t.Fatalf("%s: %d %v", c.text, got, err)
		}
	}
	for _, s := range []string{"", "-1", "1e3", "NaN", "1.2345", "1.", "1 000", "1000000000", ".5"} {
		if _, err := ParseWorkQuantity(s, 3, true); err == nil {
			t.Fatalf("accepted %s", s)
		}
	}
	if _, err := ParseWorkQuantity("0", 3, false); err == nil {
		t.Fatal("zero accepted")
	}
}

// --------- конец нового кода chat
