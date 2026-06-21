package components

import (
	"charm.land/lipgloss/v2"
	"github.com/skubalj/switchboard/tui/style"
)

type Frame struct {
	Title    string
	Width    int
	Height   int
	PaddingX int
	PaddingY int
}

func (f Frame) InnerWidth() int {
	return f.Width - 2 - (2 * f.PaddingX)
}

func (f Frame) InnerHeight() int {
	return f.Height - 2 - (2 * f.PaddingY)
}

func (f Frame) Render(content string) string {
	sizedContent := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder(), true).
		Padding(f.PaddingY, f.PaddingX).
		Render(lipgloss.Place(f.InnerWidth(), f.InnerHeight(), lipgloss.Top, lipgloss.Left, content))
	if f.Title == "" {
		return sizedContent
	}

	titleContent := style.Header.Padding(0, 1).Render(f.Title)
	return lipgloss.NewCompositor(
		lipgloss.NewLayer(titleContent).X(2).Z(1),
		lipgloss.NewLayer(sizedContent),
	).Render()
}
