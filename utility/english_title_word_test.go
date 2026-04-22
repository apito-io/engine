package utility

import "testing"

func TestEnglishTitleWord_asciiNoPanic(t *testing.T) {
	// Segment length 5 (e.g. "order") previously triggered x/text/cases panics in production.
	for _, s := range []string{"order", "hello", "a", "xy", "category"} {
		_ = englishTitleWord(s)
	}
	if got := englishTitleWord("order"); got != "Order" {
		t.Fatalf("englishTitleWord(order) = %q want Order", got)
	}
}

func TestCamelFromCanonical_food_order(t *testing.T) {
	if got := CamelFromCanonical("food_order"); got != "foodOrder" {
		t.Fatalf("got %q", got)
	}
}
