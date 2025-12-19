package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

type Package struct {
	Name    string
	Manager string
	Version string
}

type model struct {
	textInput textinput.Model
	packages  []Package
	filtered  []Package
	viewport  viewport.Model
	cursor    int // Index of the selected item in the filtered list
	err       error
}

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "Search packages..."
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 20

	vp := viewport.New(80, 20)

	// Mock data
	pkgs := []Package{
		{Name: "neovim", Manager: "brew", Version: "0.9.0"},
		{Name: "ripgrep", Manager: "apt", Version: "13.0.0"},
		{Name: "fd", Manager: "apt", Version: "8.7.0"},
		{Name: "bat", Manager: "brew", Version: "0.23.0"},
		{Name: "git", Manager: "system", Version: "2.40.0"},
		{Name: "docker", Manager: "brew", Version: "24.0.0"},
		{Name: "go", Manager: "brew", Version: "1.21.0"},
		{Name: "python3", Manager: "apt", Version: "3.10.0"},
	}

	return model{
		textInput: ti,
		packages:  pkgs,
		filtered:  pkgs,
		viewport:  vp,
		cursor:    0,
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyUp:
			if m.cursor > 0 {
				m.cursor--
			}
		case tea.KeyDown:
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
		}
	case tea.WindowSizeMsg:
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 10 // Leave room for input and status bar
	}

	// Update text input
	m.textInput, cmd = m.textInput.Update(msg)

	// Filter logic
	query := m.textInput.Value()

	// Reset filter and cursor if query changed (simple check)
	// In a real app we'd track previous query to know if it changed
	newFiltered := []Package{}
	for _, pkg := range m.packages {
		if strings.Contains(strings.ToLower(pkg.Name), strings.ToLower(query)) {
			newFiltered = append(newFiltered, pkg)
		}
	}

	// If the list content changed, likely due to a search update, reset cursor
	// This is a naive check; ideally we'd check if the query string actually changed.
	// For now, if the length differs or we are just typing (handled by textInput.Update potentially consuming keys),
	// let's just re-render.
	// Optimization: check if query matches previous state. But here we re-filter every frame textInput updates.

	// To properly reset cursor on search change, we need to know if textInput changed.
	// simpler: just re-assign. If cursor is out of bounds, fix it.
	m.filtered = newFiltered
	if m.cursor >= len(m.filtered) {
		m.cursor = 0
	}
	if len(m.filtered) == 0 {
		m.cursor = -1 // No selection
	}

	// Render the list content
	var sb strings.Builder
	for i, pkg := range m.filtered {
		line := fmt.Sprintf("%-20s %-10s %s", pkg.Name, pkg.Manager, pkg.Version)
		if i == m.cursor {
			sb.WriteString(selectedItemStyle.Render(line) + "\n")
		} else {
			sb.WriteString(line + "\n")
		}
	}
	m.viewport.SetContent(sb.String())

	// Keep cursor in view (simple approach: basic scroll)
	// For proper scrolling with viewport and variable height items it's complex,
	// but here lines are fixed height (1 line).
	// Viewport handles content, but we need to set the Y offset to make sure cursor line is visible.
	// This is a bit manual with viewport. simpler to just let viewport be a dumb container and we manage the string?
	// Actually viewport is good for scrolling huge text.

	// Vertical scroll logic for keeping cursor in view
	if m.cursor >= 0 {
		// Line height is 1.
		// Viewport height is m.viewport.Height
		// Current scroll offset is m.viewport.YOffset

		if m.cursor < m.viewport.YOffset {
			m.viewport.SetYOffset(m.cursor)
		} else if m.cursor >= m.viewport.YOffset+m.viewport.Height {
			m.viewport.SetYOffset(m.cursor - m.viewport.Height + 1)
		}
	}

	return m, cmd
}

func (m model) View() string {
	statusBar := statusBarStyle.Render("Arrows: Navigate • Esc: Quit")

	return appStyle.Render(fmt.Sprintf(
		"%s\n\n%s\n%s",
		inputBoxStyle.Render(m.textInput.View()),
		listBoxStyle.Render(m.viewport.View()),
		statusBar,
	)) + "\n"
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
