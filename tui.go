package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

type searchResultMsg struct {
	packages []Package
	status   string
}

type searchErrorMsg error

type model struct {
	textInput    textinput.Model
	packages     []Package
	filtered     []Package
	status       string
	viewport     viewport.Model
	cursor       int // Index of the selected item in the filtered list
	err          error
	width        int
	height       int
	searchCtx    context.Context
	searchCancel context.CancelFunc
}

func initialModel(pkgs []Package, status string) model {
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
		status:    status,
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
		vpWidth := max(msg.Width-2, 0)
		vpHeight := max(msg.Height-9, 0)

		m.viewport.Width = vpWidth
		m.viewport.Height = vpHeight

		// Update text input width
		m.textInput.Width = max(vpWidth-2, 0)
	}

	// Update text input
	var tiCmd tea.Cmd
	m.textInput, tiCmd = m.textInput.Update(msg)
	cmd = tea.Batch(cmd, tiCmd)

	// If text input changed, trigger search
	query := m.textInput.Value()

	// We check if the query changed.
	// Note: We need to store the "last query" to know if it changed.
	// But simply checking against the current results isn't enough.
	// For now, we can rely on the fact that if we get a KeyMsg that isn't navigation, it might be text.
	// Or we can just check if we are in a "Searching" state?
	// Actually, let's just use a simple heuristic: if the message is a KeyMsg (and not navigation), we assume input *might* have changed.
	// Better yet, we can check if the model's query matches the last one? No, we don't store "last query".
	// Let's just cancel and restart if it's a typing event?
	// The best way is to check `textInput.Value()` vs a stored value, but `m` is value receiver in some places (standard Bubble Tea).
	// But `Update` returns `m`. So we can't easily store "previous" unless we add it to struct.
	// However, `textInput.Update` handles the change.

	// Let's trigger search on every KeyMsg that is a text character.
	// Or simplistic: Just fire it?
	// Proper way: Add `lastQuery` to model?
	// For now, let's just trigger it. The context cancellation handles the "agility".

	// Actually, let's look at the previous attempt:
	// `if m.textInput.Value() != query` -> query was defined as `m.textInput.Value()` so that is always false!
	// We need to compare to something else.

	// To fix the "jittery" issue, we MUST cancel the old context.
	// We will assume that strictly speaking, we want to search if the input is not empty.
	// But we don't want to re-search if nothing changed.
	// Since we don't track `lastQuery`, let's just do it if `msg` is likely to have changed text.

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// If it's a key message and NOT navigation/special, it's likely text.
		if msg.Type == tea.KeyRunes || msg.Type == tea.KeyBackspace || msg.Type == tea.KeyDelete {
			if m.searchCancel != nil {
				m.searchCancel()
			}
			var ctx context.Context
			ctx, m.searchCancel = context.WithCancel(context.Background())
			m.searchCtx = ctx
			// Delay slightly? No, immediate is fine with cancellation.
			cmd = tea.Batch(cmd, performSearch(ctx, query))
			m.status = "Searching..."
		}
	}

	switch msg := msg.(type) {
	case searchResultMsg:
		m.packages = msg.packages
		m.status = msg.status
		m.filtered = m.packages
		m.cursor = 0
		if len(m.filtered) == 0 {
			m.cursor = -1
		}
	case searchErrorMsg:
		m.status = "Search failed: " + msg.Error()
	}

	// Render the list content
	var sb strings.Builder

	// Calculate column widths based on viewport width
	totalWidth := max(m.viewport.Width, 40) // Fallback

	colName := max(int(float64(totalWidth)*0.35), 10)
	colMgr := max(int(float64(totalWidth)*0.15), 6)
	colStatus := max(int(float64(totalWidth)*0.15), 10)
	colVer := max(totalWidth-colName-colMgr-colStatus-3, 10)

	formatStr := fmt.Sprintf("%%-%ds %%-%ds %%-%ds %%s", colName, colMgr, colVer)

	for i, pkg := range m.filtered {
		installed := "installed ✓"
		if !pkg.IsInstalled {
			installed = "install ↓"
		}

		name := truncate(pkg.Name, colName)
		manager := truncate(pkg.Manager, colMgr)
		version := truncate(pkg.Version, colVer)

		line := fmt.Sprintf(formatStr, name, manager, version, installed)
		if i == m.cursor {
			styled := selectedItemStyle.Width(m.viewport.Width).Render(line)
			sb.WriteString(styled + "\n")
		} else {
			sb.WriteString(line + "\n")
		}
	}
	m.viewport.SetContent(sb.String())

	// Vertical scroll logic
	if m.cursor >= 0 {
		if m.cursor < m.viewport.YOffset {
			m.viewport.SetYOffset(m.cursor)
		} else if m.cursor >= m.viewport.YOffset+m.viewport.Height {
			m.viewport.SetYOffset(m.cursor - m.viewport.Height + 1)
		}
	}

	return m, cmd
}

func performSearch(ctx context.Context, query string) tea.Cmd {
	return func() tea.Msg {
		if strings.TrimSpace(query) == "" {
			return searchResultMsg{packages: []Package{}, status: "Ready"}
		}

		var pkgs []Package

		// APT Search
		cmdApt := exec.CommandContext(ctx, "apt", "search", "--names-only", strings.ToLower(query))
		cmdApt.Env = append(cmdApt.Env, "TERM=dumb")
		outApt, err := cmdApt.Output()
		if err == nil {
			pkgs = append(pkgs, parseAptOutput(outApt)...)
		} else if ctx.Err() != nil {
			return nil // Cancelled
		}

		// Snap Search
		cmdSnap := exec.CommandContext(ctx, "snap", "search", strings.ToLower(query))
		cmdSnap.Env = append(cmdSnap.Env, "TERM=dumb")
		outSnap, err := cmdSnap.Output()
		if err == nil {
			pkgs = append(pkgs, parseSnapOutput(outSnap)...)
		} else if ctx.Err() != nil {
			return nil // Cancelled
		}

		return searchResultMsg{
			packages: pkgs,
			status:   fmt.Sprintf("Found %d packages", len(pkgs)),
		}
	}
}

func parseAptOutput(output []byte) []Package {
	var pkgs []Package
	scanner := bufio.NewScanner(bytes.NewReader(output))
	var currentPkg *Package

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line == "Sorting... Done" || line == "Full Text Search... Done" {
			continue
		}
		if strings.Contains(line, "/") {
			if currentPkg != nil {
				pkgs = append(pkgs, *currentPkg)
			}
			parts := strings.Fields(line)
			if len(parts) < 2 {
				currentPkg = nil
				continue
			}
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
			pkgs = append(pkgs, *currentPkg)
		}
	}
	if currentPkg != nil {
		pkgs = append(pkgs, *currentPkg)
	}
	return pkgs
}

func parseSnapOutput(output []byte) []Package {
	var pkgs []Package
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 || parts[0] == "Name" { // Skip header too if present
			continue
		}
		pkgs = append(pkgs, Package{
			Name:        parts[0],
			Version:     parts[1],
			Manager:     "snap",
			IsInstalled: false, // todo check
		})
	}
	return pkgs
}

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	commandBar := commandBarStyle.Render("Arrows: Navigate • Esc: Quit")

	statusBar := statusBarStyle.Width(m.width)

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
		statusBar.Render(m.status),
	)) + "\n"
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 3 {
		return s[:maxLen]
	}
	return s[:maxLen-1] + "…"
}
