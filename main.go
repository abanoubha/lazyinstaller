package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type Package struct {
	Name    string
	Manager string
	Version string
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

func main() {
	const version = "25.12.20"

	if len(os.Args) > 1 {
		// parse arguments
		args := os.Args[1:]
		for i := range args {
			arg := args[i]
			if strings.HasPrefix(arg, "-") {
				switch arg {
				case "--help", "-h":
					printUsage(version)
					return
				case "--version", "-v":
					fmt.Printf("lazyinstaller v%v\n", version)
					return
				}
			} else {
				switch arg {
				case "help":
					printUsage(version)
					return
				case "version", "ver":
					fmt.Printf("lazyinstaller v%v\n", version)
					return
				}
			}
		}
	}

	// Detect OS and PM
	pms := detectPM()

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

	scannedPMs := make(map[string]struct{}, len(pms))

	// Parse packages
	for _, p := range pms {
		switch p.Name {
		case "apt", "dpkg", "dpkg-query":
			if _, exists := scannedPMs["apt"]; exists {
				continue
			}

			scannedPMs["apt"] = struct{}{}
			scannedPMs["dpkg"] = struct{}{}
			scannedPMs["dpkg-query"] = struct{}{}

			// 1. Get APT/DPKG packages
			// Using -W and -f for clean "name,version" output
			cmdDpkg := exec.Command("dpkg-query", "-W", "-f=${binary:Package},${Version}\n")
			outDpkg, err := cmdDpkg.Output()
			if err == nil {
				scanner := bufio.NewScanner(strings.NewReader(string(outDpkg)))
				for scanner.Scan() {
					line := scanner.Text()
					parts := strings.Split(line, ",")
					if len(parts) >= 2 {
						pkgs = append(pkgs, Package{
							Name:    parts[0],
							Version: parts[1],
							Manager: "apt", // dpkg
						})
					}
				}
			}
		case "snap":
			if _, exists := scannedPMs[p.Name]; exists {
				continue
			}
			scannedPMs[p.Name] = struct{}{}
			// 2. Get Snap packages
			cmdSnap := exec.Command("snap", "list")
			outSnap, err := cmdSnap.Output()
			if err == nil {
				scanner := bufio.NewScanner(strings.NewReader(string(outSnap)))
				scanner.Scan() // Skip header: "Name  Version  Rev..."
				for scanner.Scan() {
					fields := strings.Fields(scanner.Text())
					if len(fields) >= 2 {
						pkgs = append(pkgs, Package{
							Name:    fields[0],
							Version: fields[1],
							Manager: "snap",
						})
					}
				}
			}
		case "flatpak":
			if _, exists := scannedPMs[p.Name]; exists {
				continue
			}
			scannedPMs[p.Name] = struct{}{}
			// 3. Get Flatpak packages
			// --app limits to applications (hiding runtimes)
			// --columns formats output
			cmdFlatpak := exec.Command("flatpak", "list", "--app", "--columns=application,version")
			outFlatpak, err := cmdFlatpak.Output()
			if err == nil {
				scanner := bufio.NewScanner(strings.NewReader(string(outFlatpak)))
				for scanner.Scan() {
					fields := strings.Fields(scanner.Text())
					if len(fields) >= 2 {
						pkgs = append(pkgs, Package{
							Name:    fields[0],
							Version: fields[1],
							Manager: "flatpak",
						})
					}
				}
			}
		case "pacman":
			if _, exists := scannedPMs[p.Name]; exists {
				continue
			}
			scannedPMs[p.Name] = struct{}{}
			// 4. Get Pacman packages (Arch Linux)
			cmdPacman := exec.Command("pacman", "-Q")
			outPacman, err := cmdPacman.Output()
			if err == nil {
				scanner := bufio.NewScanner(strings.NewReader(string(outPacman)))
				for scanner.Scan() {
					// Output format: "name version" (e.g., "firefox 112.0.1-1")
					fields := strings.Fields(scanner.Text())
					if len(fields) >= 2 {
						pkgs = append(pkgs, Package{
							Name:    fields[0],
							Version: fields[1],
							Manager: "pacman",
						})
					}
				}
			}
		case "nix-env":
			if _, exists := scannedPMs[p.Name]; exists {
				continue
			}
			scannedPMs[p.Name] = struct{}{}
			// 5. Get Nix user-profile packages
			cmdNix := exec.Command("nix-env", "-q")
			outNix, err := cmdNix.Output()
			if err == nil {
				scanner := bufio.NewScanner(strings.NewReader(string(outNix)))
				for scanner.Scan() {
					line := scanner.Text()
					// Nix output format is usually "name-version"
					// Finding the last hyphen helps separate version from name
					lastHyphen := strings.LastIndex(line, "-")
					if lastHyphen != -1 {
						name := line[:lastHyphen]
						version := line[lastHyphen+1:]

						// Optional: check if version starts with a digit to confirm split
						if len(version) > 0 && (version[0] >= '0' && version[0] <= '9') {
							pkgs = append(pkgs, Package{
								Name:    name,
								Version: version,
								Manager: "nix-env",
							})
						} else {
							// Fallback if no clear version number
							pkgs = append(pkgs, Package{
								Name:    line,
								Version: "unknown",
								Manager: "nix-user",
							})
						}
					}
				}
			}
		case "brew":
			if _, exists := scannedPMs[p.Name]; exists {
				continue
			}
			scannedPMs[p.Name] = struct{}{}
			// 6. Get Nix user-profile packages
			cmd := exec.Command("brew", "list", "--versions")
			out, err := cmd.Output()
			if err == nil {
				scanner := bufio.NewScanner(strings.NewReader(string(out)))
				for scanner.Scan() {
					// Output lines: "readline 8.2.1" or "python@3.11 3.11.3"
					fields := strings.Fields(scanner.Text())
					if len(fields) >= 2 {
						pkgs = append(pkgs, Package{
							Name:    fields[0],
							Version: fields[1],
							Manager: "brew",
						})
					}
				}
			}
		case "port":
			if _, exists := scannedPMs[p.Name]; exists {
				continue
			}
			scannedPMs[p.Name] = struct{}{}
			// 7. Get MacPorts packages
			// "active" ensures we only get the currently linked version
			cmdPort := exec.Command("port", "installed", "active")
			outPort, err := cmdPort.Output()
			if err == nil {
				scanner := bufio.NewScanner(strings.NewReader(string(outPort)))
				// Skip the first line: "The following ports are currently installed:"
				if scanner.Scan() {
					_ = scanner.Text()
				}

				for scanner.Scan() {
					line := strings.TrimSpace(scanner.Text())
					if line == "" {
						continue
					}

					// Line format: "name @version_variant (active)"
					// Example: "curl @8.4.0_0+ssl (active)"
					parts := strings.Fields(line)
					if len(parts) >= 2 {
						name := parts[0]
						// Version is parts[1], usually starting with '@'
						version := strings.TrimPrefix(parts[1], "@")

						pkgs = append(pkgs, Package{
							Name:    name,
							Version: version,
							Manager: "macports",
						})
					}
				}
			}
		default:
			continue
		}
	}

	p := tea.NewProgram(initialModel(pkgs), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}

func printUsage(version string) {
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
