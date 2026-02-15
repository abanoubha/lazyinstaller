package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

type Package struct {
	Name         string
	Manager      string
	Version      string
	IsInstalled  bool
	IsUpgradable bool
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

const version = "260203"

func main() {
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

	ctx := context.Background()

	// Fetch installed packages
	fmt.Println("Fetching installed packages...")
	installed := fetchPackages(ctx, pms, "installed")

	// Fetch upgradable packages
	fmt.Println("Fetching upgradable packages...")
	upgradable := fetchPackages(ctx, pms, "upgradable")

	// Merge logic
	packageMap := make(map[string]Package)
	for _, p := range installed {
		key := p.Manager + "|" + p.Name
		p.IsInstalled = true
		packageMap[key] = p
	}
	for _, p := range upgradable {
		key := p.Manager + "|" + p.Name
		if existing, ok := packageMap[key]; ok {
			existing.IsUpgradable = true
			// existing.Version is currently installed. Do we want to show new version?
			// Maybe add NewVersion field? For now, keep installed version
			// but mark as upgradable.
			packageMap[key] = existing
		} else {
			p.IsUpgradable = true
			p.IsInstalled = false // It's an update candidate, maybe not installed?
			// Usually update available implies base is installed.
			// But duplicate names might exist?
			// If 'apt list --upgradable' returns 'foo 2.0', and 'apt list --installed' returns 'foo 1.0'.
			// We might match by name.
			// But here we key by Manager|Name.
			// If upgradable returns same Name/Manager, we assume it's the update for the installed one.
			// We should probably mark the INSTALLED one as upgradable.
			packageMap[key] = p
		}
	}

	var pkgs []Package
	for _, p := range packageMap {
		pkgs = append(pkgs, p)
	}

	// Launch GUI
	runGUI(pms, pkgs)
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
