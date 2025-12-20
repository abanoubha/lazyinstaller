package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

var (
	operatingSystem string
	verbose         bool
	forcedPM        string
)

type packageManager struct {
	Name string
	Path string
}

var pm packageManager
var detectedPMs []packageManager

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

	statusBar := statusBarStyle.Render("Arrows: Navigate • Esc: Quit")

	// Calculate component widths
	// App margin is 2 on each side (total 4)
	availableWidth := m.width - 4

	// Input box: Border takes 2. Content width matches available minus border.
	inputStyle := inputBoxStyle.Width(availableWidth - 2)

	// List box: Border takes 2. Content width matches available minus border.
	listStyle := listBoxStyle.Width(availableWidth - 2).Height(m.viewport.Height)

	return appStyle.Render(fmt.Sprintf(
		"%s\n\n%s\n%s",
		inputStyle.Render(m.textInput.View()),
		listStyle.Render(m.viewport.View()),
		statusBar,
	)) + "\n"
}

func detectPM() {
	if forcedPM != "" {
		pm = packageManager{Name: forcedPM, Path: ""}
		detectedPMs = append(detectedPMs, pm)
		return
	}

	// Check if binary name acts as an alias
	binName := filepath.Base(os.Args[0])
	if _, ok := pm_commands[binName]; ok && binName != "i" {
		pm = packageManager{Name: binName, Path: ""}
		detectedPMs = append(detectedPMs, pm)
		return
	}

	operatingSystem = runtime.GOOS
	switch operatingSystem {
	case "windows":
		fmt.Println("Windows support is minimal.")
		// Potential windows logic could be added here similar to linux/mac
	case "darwin":
		if ok, path := isInstalled("brew"); ok {
			detectedPMs = append(detectedPMs, packageManager{Name: "brew", Path: path})
		}
		if ok, path := isInstalled("port"); ok {
			detectedPMs = append(detectedPMs, packageManager{Name: "port", Path: path})
		}
	case "linux":
		// Try parsing /etc/os-release for ID
		id := getOSReleaseID()
		if id != "" {
			if val, ok := distro_pm[id]; ok {
				if okP, path := isInstalled(val); okP {
					detectedPMs = append(detectedPMs, packageManager{Name: val, Path: path})
					// Don't return, continue to check common ones for co-existing PMs (e.g. invalidating assumption that we only have one)
					// Actually, distro_pm often maps to the MAIN system PM.
					// We should probably check common ones too, but deduplicate?
					// For now, let's keep the logic simple: verify distro PM, then check common ones.
				}
			}
		}
		detectCommonLinuxPMs()

	default:
		fmt.Printf("Unknown operating system: %s\n", operatingSystem)
	}

	// Deduplicate detectedPMs based on Name
	uniquePMs := make([]packageManager, 0, len(detectedPMs))
	seen := make(map[string]bool)
	for _, p := range detectedPMs {
		if !seen[p.Name] {
			seen[p.Name] = true
			uniquePMs = append(uniquePMs, p)
		}
	}
	detectedPMs = uniquePMs

	if len(detectedPMs) > 0 {
		pm = detectedPMs[0]
	}
}

func executeCommand(template string, pkgName string) {
	if template == "" {
		fmt.Println("Command not defined for this package manager.")
		return
	}

	cmdStr := template
	// if template ends with ".x" or " x" remove "x" and add pkgName
	if strings.HasSuffix(template, ".x") || strings.HasSuffix(template, " x") {
		cmdStr = strings.TrimSuffix(template, "x") + pkgName
	}

	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return
	}

	head := parts[0]
	args := parts[1:]

	cmd := exec.Command(head, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		fmt.Printf("Error executing command: %v\n", err)
		os.Exit(1)
	}
}

func getAllPackages(pmName string, cmdStr string) ([]Package, error) {
	if cmdStr == "" {
		return []Package{}, errors.New("command not defined for this package manager")
	}

	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return []Package{}, errors.New("invalid command")
	}

	head := parts[0]
	args := parts[1:]

	cmd := exec.Command(head, args...)
	// Use Output() to capture stdout
	output, err := cmd.Output()
	if err != nil {
		return []Package{}, err
	}

	return parsePackages(pmName, string(output)), nil
}

func parsePackages(pmName string, output string) []Package {
	lines := strings.Split(output, "\n")
	var packages []Package

	switch pmName {
	case "apt":
		// apt list --installed format:
		// package_name/distro_release,distro_release version arch [installed]
		// e.g. "adduser/noble,noble,now 3.137ubuntu1 all [installed]"
		// Note call may include "Listing..." on first line
		for _, line := range lines {
			if strings.HasPrefix(line, "Listing...") {
				continue
			}
			if strings.TrimSpace(line) == "" {
				continue
			}
			// Split by slash first to get name
			// "adduser/noble,noble,now 3.137ubuntu1 all [installed]"
			slashSplit := strings.SplitN(line, "/", 2)
			if len(slashSplit) < 2 {
				continue
			}
			name := slashSplit[0]

			// rest string: "noble,noble,now 3.137ubuntu1 all [installed]"
			// We want version. It's usually the second field after spaces?
			// But the first part "noble,noble,now" might contain spaces? No, usually comma separated.
			// Let's split by spaces.
			rest := slashSplit[1]
			fields := strings.Fields(rest)
			// fields[0] should be "noble,noble,now" (the release info)
			// fields[1] should be version "3.137ubuntu1"
			// fields[2] arch
			// fields[3] [installed]...

			version := ""
			if len(fields) >= 2 {
				version = fields[1]
			}

			packages = append(packages, Package{
				Name:    name,
				Manager: "apt",
				Version: version,
			})
		}
	default:
		// For now, just return empty logic for others
	}

	return packages
}

func printUsage() {
	fmt.Printf("lazyinstaller the tool to manage all programs, apps, and packages installed via all available package managers v%v\n", version)
}

func main() {
	if len(os.Args) > 1 {
		// parse arguments
		args := os.Args[1:]
		for i := range args {
			arg := args[i]
			if strings.HasPrefix(arg, "-") {
				switch arg {
				case "--help", "-h":
					printUsage()
					return
				case "--version", "-v":
					fmt.Printf("lazyinstaller v%v\n", version)
					return
				}
			}
		}
	}

	var action string
	var pkgName string

	// Detect OS and PM
	detectPM()

	if pm.Name == "" {
		fmt.Println("No supported package manager found.")
		os.Exit(1)
	}

	if pkgName != "" {
		if !validateInput(pkgName) {
			fmt.Printf("Invalid package name: %s\n", pkgName)
			os.Exit(1)
		}
	}

	cmds, ok := pm_commands[pm.Name]
	if !ok {
		fmt.Printf("Configurations for package manager '%s' not found.\n", pm.Name)
		os.Exit(1)
	}

	// Check if we should update the index
	updateRequiredActions := map[string]bool{
		"install": true,
		"add":     true,
		"update":  true,
		"upgrade": true,
		"up":      true,
		"search":  true,
		"find":    true,
	}

	if cmds.UpdateIndex != "" && updateRequiredActions[action] {
		if verbose {
			fmt.Println("Updating local index...")
		}
		executeCommand(cmds.UpdateIndex, "")
	}

	// switch action {
	// case "pmlist":
	// 	var pms []string
	// 	for k := range pm_commands {
	// 		if k == "i" {
	// 			continue
	// 		}
	// 		pms = append(pms, k)
	// 	}
	// 	sort.Strings(pms)
	// 	fmt.Println("Supported package managers:")
	// 	for _, pm := range pms {
	// 		fmt.Println("- " + pm)
	// 	}
	// 	return
	// case "pms":
	// 	fmt.Println("Available package managers:")
	// 	for _, p := range detectedPMs {
	// 		fmt.Println("- " + p.Name)
	// 	}
	// 	return
	// case "info", "show":
	// 	if pkgName == "" {
	// 		fmt.Println("No package specified.")
	// 		return
	// 	}
	// 	executeCommand(cmds.Info, pkgName)
	// case "update", "upgrade", "up":
	// 	if pkgName == "" {
	// 		// Upgrade all packages for all detected package managers
	// 		fmt.Println("Upgrading all packages...")
	// 		for _, p := range detectedPMs {
	// 			c, ok := pm_commands[p.Name]
	// 			if !ok {
	// 				continue
	// 			}
	// 			if verbose {
	// 				fmt.Printf("Upgrading packages for manager: %s\n", p.Name)
	// 			}

	// 			// If this is not the primary PM (which was already updated at start), update its index
	// 			if p.Name != pm.Name && c.UpdateIndex != "" {
	// 				if verbose {
	// 					fmt.Printf("Updating index for %s...\n", p.Name)
	// 				}
	// 				executeCommand(c.UpdateIndex, "")
	// 			}

	// 			executeCommand(c.UpgradeAll, "")
	// 		}
	// 	} else {
	// 		executeCommand(cmds.Upgrade, pkgName)
	// 	}
	// case "install", "add":
	// 	if pkgName == "" {
	// 		fmt.Println("No package specified.")
	// 		return
	// 	}
	// 	if ok, path := isInstalled(pkgName); ok {
	// 		fmt.Printf("Package '%s' is already installed at %s\n", pkgName, path)
	// 		return
	// 	}
	// 	executeCommand(cmds.Install, pkgName)
	// case "uninstall", "remove", "rm":
	// 	if pkgName == "" {
	// 		fmt.Println("No package specified.")
	// 		return
	// 	}
	// 	executeCommand(cmds.Uninstall, pkgName)
	// case "reinstall":
	// 	// Fallback to install for now, as existing code did
	// 	fmt.Println("Reinstall not explicitly supported yet. Try install.")
	// case "search", "find":
	// 	if pkgName == "" {
	// 		fmt.Println("No term specified to search.")
	// 		return
	// 	}
	// 	executeCommand(cmds.Search, pkgName)
	// case "list", "installed":
	// 	for i, p := range detectedPMs {
	// 		c, ok := pm_commands[p.Name]
	// 		if !ok {
	// 			continue
	// 		}
	// 		if i > 0 {
	// 			fmt.Println()
	// 		}
	// 		fmt.Printf("Listing installed packages for %s:\n", p.Name)
	// 		executeCommand(c.ListInstalled, "")
	// 	}
	// default:
	// 	fmt.Printf("'%v' sub-command is not supported.\n", action)
	// }

	pkgs := []Package{}

	// Parse packages
	for _, p := range detectedPMs {
		c, ok := pm_commands[p.Name]
		if !ok {
			continue
		}

		loaded, err := getAllPackages(p.Name, c.ListInstalled)
		if err != nil {
			// If verbose, maybe log? For now just ignore failed PM listings
			continue
		}
		// add patches to the list
		for _, pkg := range loaded {
			pkg.Manager = p.Name // Ensure manager is set correctly
			pkgs = append(pkgs, pkg)
		}
	}

	p := tea.NewProgram(initialModel(pkgs), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}

func isInstalled(pkg string) (bool, string) {
	path, err := exec.LookPath(pkg)
	if errors.Is(err, exec.ErrNotFound) {
		return false, ""
	}
	return true, path
}

func validateInput(input string) bool {
	// Allow a-z, A-Z, 0-9, _, -, @, ., +
	// Some packages have dots (e.g. python3.8) or plus (g++)
	match, _ := regexp.MatchString(`^[a-zA-Z0-9_\-@.+]+$`, input)
	return match
}

func getOSReleaseID() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return ""
	}
	content := string(data)
	lines := strings.SplitSeq(content, "\n")
	for line := range lines {
		if after, ok := strings.CutPrefix(line, "ID="); ok {
			id := after
			return strings.Trim(id, "\"")
		}
	}
	return ""
}

// detectCommonLinuxPMs appends all found supported package managers to detectedPMs.
func detectCommonLinuxPMs() {
	checks := []string{"apt", "dnf", "pacman", "snap", "flatpak", "zypper", "yum", "apk", "xbps-install", "emerge", "nix-env", "brew", "port", "winget", "choco", "scoop"}
	for _, p := range checks {
		wrapperName := p
		if p == "xbps-install" {
			wrapperName = "xbps"
		}
		if ok, path := isInstalled(p); ok {
			detectedPMs = append(detectedPMs, packageManager{Name: wrapperName, Path: path})
		}
	}
}
