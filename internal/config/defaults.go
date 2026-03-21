package config

import (
	_ "embed"
	"image/color"
)

//go:embed default_theme.yaml
var defaultThemeYAML []byte

// DefaultBundle returns the built-in theme bundle parsed from the embedded YAML.
// Falls back to hardcoded DarkTheme/LightTheme if parsing fails.
func DefaultBundle() ThemeBundle {
	bundle, err := LoadBundleFromYAML(defaultThemeYAML)
	if err != nil {
		return ThemeBundle{
			Name:     "Default",
			Subtitle: "The caffeinated editor",
			Dark:     DarkTheme(),
			Light:    LightTheme(),
		}
	}
	return bundle
}

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

		FindMatch:   nrgba(100, 90, 30, 255),   // muted yellow, fully opaque
		FindCurrent: nrgba(255, 220, 50, 255), // sun yellow, fully opaque

		Keyword:  nrgba(197, 134, 192, 255), // purple
		String:   nrgba(206, 145, 120, 255), // orange
		Comment:  nrgba(106, 153, 85, 255),  // green
		Function: nrgba(220, 220, 170, 255), // yellow
		Type:     nrgba(78, 201, 176, 255),  // teal
		Number:   nrgba(181, 206, 168, 255), // light green
		Operator: nrgba(212, 212, 212, 255), // foreground
		Variable: nrgba(156, 220, 254, 255), // light blue

		TabBarBg:       nrgba(46, 46, 46, 255),
		TabActiveBg:    nrgba(30, 30, 30, 255),
		TabBorder:      nrgba(68, 68, 68, 255),
		TabDimFg:       nrgba(140, 140, 140, 255),
		TabModifiedDot: nrgba(200, 180, 100, 255),
		TabCloseBtn:    nrgba(150, 150, 150, 255),
		TabCloseHover:  nrgba(230, 60, 60, 255),
		TabPlusHover:   nrgba(60, 130, 230, 255),
		TitleFg:        nrgba(160, 160, 160, 255),
		SubtitleFg:     nrgba(90, 90, 90, 255),
		StatusBorder:   nrgba(45, 45, 45, 255),
		GutterSep:      nrgba(50, 50, 50, 255),
		ScrollbarThumb: nrgba(100, 100, 100, 180),
		FindBarBg:      nrgba(37, 37, 38, 252),
		FindBarBorder:  nrgba(69, 69, 69, 255),
		FindBarInputBg: nrgba(60, 60, 60, 255),
		FindBarFocus:   nrgba(0, 122, 204, 255),
		FindBarText:    nrgba(212, 212, 212, 255),
		FindBarDim:     nrgba(140, 140, 140, 255),
		DropdownBg:     nrgba(37, 37, 38, 245),
		DropdownSel:    nrgba(4, 57, 94, 255),
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

		FindMatch:   nrgba(255, 235, 120, 255), // light yellow, fully opaque
		FindCurrent: nrgba(255, 210, 0, 200),   // bright yellow

		Keyword:  nrgba(175, 0, 219, 255),  // purple
		String:   nrgba(163, 21, 21, 255),  // red
		Comment:  nrgba(0, 128, 0, 255),    // green
		Function: nrgba(121, 94, 38, 255),  // brown
		Type:     nrgba(38, 127, 153, 255), // teal
		Number:   nrgba(9, 136, 90, 255),   // green
		Operator: nrgba(30, 30, 30, 255),   // foreground
		Variable: nrgba(0, 16, 128, 255),   // blue

		TabBarBg:       nrgba(236, 236, 236, 255),
		TabActiveBg:    nrgba(255, 255, 255, 255),
		TabBorder:      nrgba(200, 200, 200, 255),
		TabDimFg:       nrgba(120, 120, 120, 255),
		TabModifiedDot: nrgba(180, 150, 50, 255),
		TabCloseBtn:    nrgba(130, 130, 130, 255),
		TabCloseHover:  nrgba(220, 50, 50, 255),
		TabPlusHover:   nrgba(40, 110, 210, 255),
		TitleFg:        nrgba(100, 100, 100, 255),
		SubtitleFg:     nrgba(170, 170, 170, 255),
		StatusBorder:   nrgba(210, 210, 210, 255),
		GutterSep:      nrgba(220, 220, 220, 255),
		ScrollbarThumb: nrgba(160, 160, 160, 180),
		FindBarBg:      nrgba(242, 242, 242, 252),
		FindBarBorder:  nrgba(200, 200, 200, 255),
		FindBarInputBg: nrgba(255, 255, 255, 255),
		FindBarFocus:   nrgba(0, 122, 204, 255),
		FindBarText:    nrgba(30, 30, 30, 255),
		FindBarDim:     nrgba(120, 120, 120, 255),
		DropdownBg:     nrgba(242, 242, 242, 245),
		DropdownSel:    nrgba(0, 122, 204, 60),
	}
}

// DefaultTheme returns the appropriate theme for the given appearance.
func DefaultTheme(appearance Appearance) Theme {
	if appearance == Light {
		return LightTheme()
	}
	return DarkTheme()
}
