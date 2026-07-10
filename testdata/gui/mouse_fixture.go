package fixture

// This file is intentionally long and contains tabs, Unicode, and nested folds.
// It is opened by the isolated GUI harness and is never saved by the smoke test.

type PointerFixture struct {
	Name    string
	Enabled bool
	Count   int
}

func nestedFold(value int) int {
	if value > 0 {
		for index := 0; index < value; index++ {
			value += index
		}
	}
	return value
}

func tabColumns() string {
	left := "before-tab"
	right := "after-tab"
	return left + " → " + right
}

// Scroll targets follow. The numbering makes a one-line click offset visible.
var scrollTargets = []string{
	"line 01 — blåbær",
	"line 02 — mouse",
	"line 03 — pointer",
	"line 04 — click",
	"line 05 — drag",
	"line 06 — scroll",
	"line 07 — primary",
	"line 08 — secondary",
	"line 09 — middle",
	"line 10 — hover",
	"line 11 — cancel",
	"line 12 — release",
	"line 13 — selection",
	"line 14 — gutter",
	"line 15 — folding",
	"line 16 — markdown",
	"line 17 — preview",
	"line 18 — tab",
	"line 19 — window",
	"line 20 — tooltip",
	"line 21 — offset",
	"line 22 — fractional",
	"line 23 — viewport",
	"line 24 — cursor",
	"line 25 — anchor",
	"line 26 — unicode",
	"line 27 — syntax",
	"line 28 — highlight",
	"line 29 — status",
	"line 30 — footer",
	"line 31 — command",
	"line 32 — option",
	"line 33 — control",
	"line 34 — shift",
	"line 35 — keyboard",
	"line 36 — trackpad",
	"line 37 — wheel",
	"line 38 — pixel",
	"line 39 — display",
	"line 40 — buffer",
}
