package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

var (
	operatingSystem string
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

func detectPM() {
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
		// todo : status bar
		// fmt.Printf("Error executing command: %v\n", err)
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
			} else {
				switch arg {
				case "help":
					printUsage()
					return
				case "version", "ver":
					fmt.Printf("lazyinstaller v%v\n", version)
					return
				}
			}
		}
	}

	var action string

	// Detect OS and PM
	detectPM()

	if pm.Name == "" {
		fmt.Println("No supported package manager found.")
		os.Exit(1)
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
		// todo: status bar
		//fmt.Println("Updating local index...")
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

func printUsage() {
	fmt.Printf("lazyinstaller v%v\nthe tool to manage all programs, apps, and packages installed via all available package managers\n", version)
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
