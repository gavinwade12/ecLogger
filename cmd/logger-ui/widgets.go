package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func NewWrappedLabel(text string) *widget.Label {
	label := widget.NewLabel(text)
	label.Wrapping = fyne.TextWrapWord
	return label
}

// hint is secondary text that explains a page, like why it's empty, or
// what went wrong.
type hint struct {
	*widget.RichText
	segment *widget.TextSegment
}

func newHint(text string) *hint {
	h := &hint{segment: &widget.TextSegment{Style: widget.RichTextStyle{
		Alignment: fyne.TextAlignCenter,
		ColorName: colorNameSecondaryText,
	}}}
	h.RichText = widget.NewRichText(h.segment)
	h.SetText(text)
	return h
}

// Text returns the hint's text.
func (h *hint) Text() string {
	return h.segment.Text
}

// SetText shows text as the hint.
func (h *hint) SetText(text string) {
	h.set(text, colorNameSecondaryText)
}

// SetError shows text as the hint, styled as an error.
func (h *hint) SetError(text string) {
	h.set(text, theme.ColorNameError)
}

func (h *hint) set(text string, color fyne.ThemeColorName) {
	h.segment.Text = text
	h.segment.Style.ColorName = color
	h.Refresh()
}
