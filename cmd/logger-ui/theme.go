package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// The sizes and colors of the app's own widgets. They live in the theme so
// a theme for another screen size only has to change them there.
const (
	// sizeNameTileMinWidth is the narrowest a tile gets before the tile
	// grid drops a column.
	sizeNameTileMinWidth fyne.ThemeSizeName = "tileMinWidth"
	// sizeNameTileValueText is the text size of a tile's value.
	sizeNameTileValueText fyne.ThemeSizeName = "tileValueText"
	// sizeNameTileDetailText is the text size of a tile's details, like
	// its min and max.
	sizeNameTileDetailText fyne.ThemeSizeName = "tileDetailText"
	// sizeNameFormMaxWidth is the widest a form gets, so its inputs don't
	// stretch across a wide screen.
	sizeNameFormMaxWidth fyne.ThemeSizeName = "formMaxWidth"
	// sizeNameChartLaneMinHeight is the shortest a chart lane gets, its
	// title included. Four fit on an 800x480 screen beside the tab bar.
	sizeNameChartLaneMinHeight fyne.ThemeSizeName = "chartLaneMinHeight"
	// sizeNameChartMinWidth is the narrowest a chart gets.
	sizeNameChartMinWidth fyne.ThemeSizeName = "chartMinWidth"
	// sizeNameChartLineWidth is the thickness of a chart's traces.
	sizeNameChartLineWidth fyne.ThemeSizeName = "chartLineWidth"

	// colorNameTile is the background of a tile.
	colorNameTile fyne.ThemeColorName = "tile"
	// colorNameSecondaryText is the color of text that supports other
	// text, like units and hints.
	colorNameSecondaryText fyne.ThemeColorName = "secondaryText"
)

// appTheme is Fyne's default theme plus the app's own sizes and colors.
type appTheme struct {
	fyne.Theme
}

func newAppTheme() fyne.Theme {
	return appTheme{Theme: theme.DefaultTheme()}
}

func (t appTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case colorNameTile:
		return t.Theme.Color(theme.ColorNameButton, variant)
	case colorNameSecondaryText:
		return t.Theme.Color(theme.ColorNamePlaceHolder, variant)
	}
	return t.Theme.Color(name, variant)
}

func (t appTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case sizeNameTileMinWidth:
		return 180
	case sizeNameTileValueText:
		return 28
	case sizeNameTileDetailText:
		return 12
	case sizeNameFormMaxWidth:
		return 560
	case sizeNameChartLaneMinHeight:
		return 84
	case sizeNameChartMinWidth:
		return 240
	case sizeNameChartLineWidth:
		return 1.5
	}
	return t.Theme.Size(name)
}
