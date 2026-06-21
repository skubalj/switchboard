package modal

import (
	tea "charm.land/bubbletea/v2"
)

type Window interface {
	Update(msg tea.Msg) (Window, tea.Cmd)
	Render() ContentBlock
}

type ContentBlock struct {
	Content string
	Width   int
	Height  int
}
