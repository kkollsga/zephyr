package config

import "image/color"

func nrgba(r, g, b, a uint8) color.NRGBA {
	return color.NRGBA{R: r, G: g, B: b, A: a}
}

// DarkTheme returns the default dark color scheme.
func DarkTheme() Theme {
	return Theme{
		Name:          "Dark",
		Background:    nrgba(30, 30, 30, 255),
		Foreground:    nrgba(212, 212, 212, 255),
		Gutter:        nrgba(110, 110, 110, 255),
		GutterBg:      nrgba(28, 28, 28, 255),
		Cursor:        nrgba(212, 212, 212, 255),
		Selection:     nrgba(60, 90, 140, 128),
		LineHighlight: nrgba(40, 40, 40, 255),
		StatusBg:      nrgba(25, 25, 25, 255),
		StatusFg:      nrgba(150, 150, 150, 255),

		Keyword:  nrgba(197, 134, 192, 255), // purple
		String:   nrgba(206, 145, 120, 255), // orange
		Comment:  nrgba(106, 153, 85, 255),  // green
		Function: nrgba(220, 220, 170, 255), // yellow
		Type:     nrgba(78, 201, 176, 255),  // teal
		Number:   nrgba(181, 206, 168, 255), // light green
		Operator: nrgba(212, 212, 212, 255), // foreground
		Variable: nrgba(156, 220, 254, 255), // light blue
	}
}

// LightTheme returns the default light color scheme.
func LightTheme() Theme {
	return Theme{
		Name:          "Light",
		Background:    nrgba(255, 255, 255, 255),
		Foreground:    nrgba(30, 30, 30, 255),
		Gutter:        nrgba(150, 150, 150, 255),
		GutterBg:      nrgba(255, 255, 255, 255),
		Cursor:        nrgba(30, 30, 30, 255),
		Selection:     nrgba(173, 214, 255, 128),
		LineHighlight: nrgba(245, 245, 245, 255),
		StatusBg:      nrgba(240, 240, 240, 255),
		StatusFg:      nrgba(100, 100, 100, 255),

		Keyword:  nrgba(175, 0, 219, 255),  // purple
		String:   nrgba(163, 21, 21, 255),  // red
		Comment:  nrgba(0, 128, 0, 255),    // green
		Function: nrgba(121, 94, 38, 255),  // brown
		Type:     nrgba(38, 127, 153, 255), // teal
		Number:   nrgba(9, 136, 90, 255),   // green
		Operator: nrgba(30, 30, 30, 255),   // foreground
		Variable: nrgba(0, 16, 128, 255),   // blue
	}
}

// DefaultTheme returns the appropriate theme for the given appearance.
func DefaultTheme(appearance Appearance) Theme {
	if appearance == Light {
		return LightTheme()
	}
	return DarkTheme()
}
