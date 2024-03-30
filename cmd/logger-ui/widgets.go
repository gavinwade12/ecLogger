package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

func NewWrappedLabel(text string) *widget.Label {
	label := widget.NewLabel(text)
	label.Wrapping = fyne.TextWrapWord
	return label
}
