package textinputmodal

import (
	"cmp"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/skubalj/switchboard/tui/components"
	"github.com/skubalj/switchboard/tui/modal"
	"github.com/skubalj/switchboard/tui/style"
)

type keyMap struct {
	Apply    key.Binding
	Cancel   key.Binding
	Quit     key.Binding
	Next     key.Binding
	Previous key.Binding
	Up       key.Binding
	Down     key.Binding
	Left     key.Binding
	Right    key.Binding
}

var textInputKeymap = keyMap{
	Apply:    key.NewBinding(key.WithKeys("enter")),
	Cancel:   key.NewBinding(key.WithKeys("esc")),
	Quit:     key.NewBinding(key.WithKeys("ctrl+c")),
	Next:     key.NewBinding(key.WithKeys("tab")),
	Previous: key.NewBinding(key.WithKeys("shift+tab")),
	Up:       key.NewBinding(key.WithKeys("up")),
	Down:     key.NewBinding(key.WithKeys("down")),
	Left:     key.NewBinding(key.WithKeys("left")),
	Right:    key.NewBinding(key.WithKeys("right")),
}

type State int

const (
	Ok State = iota
	Cancel
	Quit
)

type TextInputModal struct {
	title       string
	errorString string
	selectedIdx int
	inputs      []textinput.Model
	buttons     []string
}

type TextInput struct {
	Prompt      string
	Placeholder string
	IsPassword  bool
	Value       string
}

func NewTextInputModal(title string, prompts []TextInput, buttons []string) TextInputModal {
	baseInput := textinput.New()
	baseInput.SetVirtualCursor(true)
	baseInput.SetWidth(48)
	baseInput.SetStyles(style.InputBox)

	inputs := make([]textinput.Model, 0, len(prompts))
	for _, p := range prompts {
		input := baseInput
		input.Prompt = p.Prompt
		input.Placeholder = p.Placeholder
		input.SetValue(p.Value)
		if p.IsPassword {
			input.EchoMode = textinput.EchoPassword
			input.EchoCharacter = '*'
		}
		inputs = append(inputs, input)
	}

	m := TextInputModal{
		title:       title,
		errorString: "",
		inputs:      inputs,
		buttons:     buttons,
	}
	m.updateFocus()
	return m
}

func (m *TextInputModal) GetValues() []string {
	values := make([]string, 0, len(m.inputs))
	for _, input := range m.inputs {
		values = append(values, cmp.Or(input.Value(), input.Placeholder))
	}
	return values
}

func (m *TextInputModal) SetError(err string) {
	m.errorString = err
}

func (m *TextInputModal) selectionInInputs() bool {
	return m.selectedIdx < len(m.inputs)
}

func (m *TextInputModal) selectionInButtons() bool {
	return m.selectedIdx >= len(m.inputs)
}

func (m *TextInputModal) maxIndex() int {
	return len(m.inputs) + len(m.buttons) - 1
}

func (m *TextInputModal) Update(msg tea.Msg) (button string, state State) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, textInputKeymap.Apply):
			idx := m.selectedIdx - len(m.inputs)
			if idx >= 0 {
				return m.buttons[idx], Ok
			}
		case key.Matches(msg, textInputKeymap.Cancel):
			return "", Cancel
		case key.Matches(msg, textInputKeymap.Quit):
			return "", Quit
		case key.Matches(msg, textInputKeymap.Next):
			if m.selectedIdx < m.maxIndex() {
				m.selectedIdx += 1
			}
		case key.Matches(msg, textInputKeymap.Previous):
			if m.selectedIdx > 0 {
				m.selectedIdx -= 1
			}
		case key.Matches(msg, textInputKeymap.Up):
			if m.selectionInButtons() {
				// go to last input
				m.selectedIdx = len(m.inputs) - 1
			} else if m.selectedIdx > 0 {
				m.selectedIdx -= 1
			}
		case key.Matches(msg, textInputKeymap.Down):
			if m.selectionInInputs() {
				m.selectedIdx += 1
			}
		case key.Matches(msg, textInputKeymap.Left):
			if m.selectionInButtons() && m.selectedIdx > len(m.inputs) {
				m.selectedIdx -= 1
			}
		case key.Matches(msg, textInputKeymap.Right):
			if m.selectionInButtons() && m.selectedIdx < m.maxIndex() {
				m.selectedIdx += 1
			}
		}
	}

	m.updateFocus()
	m.updateInputs(msg)

	return "", Ok
}

func (m *TextInputModal) SetInputs(values []string) {
	for idx, value := range values {
		m.inputs[idx].SetValue(value)
	}
}

func (m *TextInputModal) updateInputs(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	for i := range m.inputs {
		m.inputs[i], _ = m.inputs[i].Update(msg)
	}
	return tea.Batch(cmds...)
}

func (m *TextInputModal) updateFocus() {
	for i := range m.inputs {
		if m.selectedIdx == i {
			m.inputs[i].Focus()
		} else {
			m.inputs[i].Blur()
		}
	}
}

func (m *TextInputModal) width() int {
	return 64 // fixme
}

func (m *TextInputModal) height() int {
	return len(m.inputs) + 4 // add for borders, buttons, and error message row
}

func (m *TextInputModal) Render() modal.ContentBlock {
	var buf strings.Builder
	for _, input := range m.inputs {
		buf.WriteString(input.View())
		buf.WriteRune('\n')
	}

	buf.WriteString(style.ErrString.Render(m.errorString))
	buf.WriteRune('\n')

	btnIdx := m.selectedIdx - len(m.inputs)
	for idx, btn := range m.buttons {
		btn = "<" + btn + ">"
		if idx == btnIdx {
			btn = style.ButtonSelected.Render(btn)
		}
		buf.WriteString(btn)
		buf.WriteString("  ")
	}

	frame := components.Frame{
		Title:    m.title,
		Width:    m.width(),
		Height:   m.height(),
		PaddingX: 1,
	}

	return modal.ContentBlock{
		Content: frame.Render(buf.String()),
		Width:   frame.Width,
		Height:  frame.Height,
	}
}
