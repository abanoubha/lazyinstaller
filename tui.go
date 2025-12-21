package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

type model struct {
	textInput textinput.Model
	packages  []Package
	filtered  []Package
	viewport  viewport.Model
	cursor    int // Index of the selected item in the filtered list
	err       error
	width     int
	height    int
}

func initialModel(pkgs []Package) model {
	ti := textinput.New()
	ti.Placeholder = "Search packages..."
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 20

	vp := viewport.New(80, 20)

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
		m.width = msg.Width
		m.height = msg.Height

		// Update viewport size
		// App margins (2x2=4 horizontal, 1x2=2 vertical) + Component overhead
		// Width: Window - 4 (App Margin) - 2 (List Border) - 2 (List Padding) = Window - 8
		// Height: Window - 2 (App Margin) - 3 (Input) - 2 (Gap) - 2 (Status) - 2 (List Border) = Window - 11

		vpWidth := max(msg.Width-8, 0)

		vpHeight := max(msg.Height-11, 0)

		m.viewport.Width = vpWidth
		m.viewport.Height = vpHeight
	}
	// Update text input
	m.textInput, cmd = m.textInput.Update(msg)

	// Filter logic
	query := m.textInput.Value()

	////////////// apt search ///////////

	// Using --names-only restricts search to package names (ignoring descriptions)
	cmdApt := exec.Command("apt", "search", "--names-only", strings.ToLower(query))
	cmdApt.Env = append(cmdApt.Env, "TERM=dumb") // Disable colors

	output, err := cmdApt.Output()
	if err == nil {
		scanner := bufio.NewScanner(bytes.NewReader(output))

		var currentPkg *Package

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())

			// Skip empty lines and headers
			if line == "" || line == "Sorting... Done" || line == "Full Text Search... Done" {
				continue
			}

			// Identify if this is a "Package Line" or a "Description Line"
			// Package lines usually look like: "name/release version arch [status]"
			// We detect this by looking for the slash '/' which separates name and suite
			if strings.Contains(line, "/") {
				// If we were building a previous package, save it now
				if currentPkg != nil {
					m.packages = append(m.packages, *currentPkg)
				}

				// Parse new package line
				parts := strings.Fields(line)
				if len(parts) < 2 {
					currentPkg = nil
					continue
				}

				// split "vim/jammy" -> "vim"
				rawName := parts[0]
				name := strings.Split(rawName, "/")[0]

				version := parts[1]

				isInstalled := strings.Contains(line, "[installed")

				currentPkg = &Package{
					Name:        name,
					Version:     version,
					Manager:     "apt/dpkg",
					IsInstalled: isInstalled,
				}

				m.packages = append(m.packages, *currentPkg)

			}
		}

		// Catch the very last package if the loop finished without appending
		if currentPkg != nil {
			m.packages = append(m.packages, *currentPkg)
		}

	} else {
		// todo: show in status bar (can not search available packages)
	}

	////////////// end of apt search ///////////

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
		installed := "installed ✓"
		if !pkg.IsInstalled {
			installed = "install ↓"
		}
		line := fmt.Sprintf("%-30s %-10s %-30s %s", pkg.Name, pkg.Manager, pkg.Version, installed)
		if i == m.cursor {
			// Ensure the highlight spans the full viewport width
			styled := selectedItemStyle.Width(m.viewport.Width).Render(line)
			sb.WriteString(styled + "\n")
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
	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	commandBar := commandBarStyle.Render("Arrows: Navigate • Esc: Quit")

	statusBar := statusBarStyle.Render("LazyInstaller just started")

	// Calculate component widths
	availableWidth := m.width

	// Input box: Border takes 2. Content width matches available minus border.
	inputStyle := inputBoxStyle.Width(availableWidth - 2)

	// List box: Border takes 2. Content width matches available minus border.
	listStyle := listBoxStyle.Width(availableWidth - 2).Height(m.viewport.Height)

	return appStyle.Render(fmt.Sprintf(
		"%s\n\n%s\n%s\n%s",
		inputStyle.Render(m.textInput.View()),
		listStyle.Render(m.viewport.View()),
		commandBar,
		statusBar,
	)) + "\n"
}
