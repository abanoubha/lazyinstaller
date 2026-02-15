package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// fetchPackages fetches all packages of a specific type (installed, upgradable, all) from all detected PMs.
func fetchPackages(ctx context.Context, pms []packageManager, listType string) []Package {
	var wg sync.WaitGroup
	resultCh := make(chan []Package, len(pms))

	for _, pm := range pms {
		wg.Add(1)
		go func(pm packageManager) {
			defer wg.Done()
			pkgs, err := getPackagesForPM(ctx, pm, listType)
			if err == nil {
				resultCh <- pkgs
			} else {
				// Log error?
				// fmt.Printf("Error fetching %s for %s: %v\n", listType, pm.Name, err)
			}
		}(pm)
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	var allPkgs []Package
	for pkgs := range resultCh {
		allPkgs = append(allPkgs, pkgs...)
	}
	return allPkgs
}

func getPackagesForPM(ctx context.Context, pm packageManager, listType string) ([]Package, error) {
	// Map pm.Name to command key if needed (pm.go: detectPM returns wrapperName)
	// commands.go: pm_commands keys.
	// We assume pm.Name matches keys in pm_commands (e.g. "apt", "snap").
	// pm.go has logic: "apt" -> "apt".

	cmdStruct, ok := pm_commands[pm.Name]
	if !ok {
		// Try to find if there is a mapping? pm.go: detectPM uses same names mostly.
		return nil, fmt.Errorf("unknown pm: %s", pm.Name)
	}

	var cmdStr string
	switch listType {
	case "installed":
		cmdStr = cmdStruct.ListInstalled
	case "upgradable":
		cmdStr = cmdStruct.ListUpgradable
	case "all":
		cmdStr = cmdStruct.ListAll
	default:
		return nil, fmt.Errorf("unknown list type: %s", listType)
	}

	if cmdStr == "" {
		return nil, nil // Not supported
	}

	// Execute command
	// Parsing is PM specific.
	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty command")
	}
	head := parts[0]
	args := parts[1:]

	cmd := exec.CommandContext(ctx, head, args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	output := string(out)

	var pkgs []Package
	switch pm.Name {
	case "apt", "dpkg":
		// apt list output parsing
		pkgs = parseAptOutput(output)
	case "snap":
		pkgs = parseSnapOutput(output)
	case "flatpak":
		pkgs = parseFlatpakOutput(output)
	case "pacman":
		pkgs = parsePacmanOutput(output)
	case "brew":
		pkgs = parseBrewOutput(output)
	case "port":
		pkgs = parsePortOutput(output)
	case "dnf", "rpm", "yum":
		pkgs = parseDnfOutput(output)
	// Add other cases as needed
	default:
		// Generic fallback or return empty?
		// We can try generic line parsing if we know format? Use simple fields.
		// For now return empty if no parser.
	}

	// Post-process to ensure Manager field is set correctly and IsUpgradable/IsInstalled
	for i := range pkgs {
		pkgs[i].Manager = pm.Name
		if listType == "installed" {
			pkgs[i].IsInstalled = true
		} else if listType == "upgradable" {
			pkgs[i].IsUpgradable = true
			// Usually upgradable packages are also installed (the old version).
			// But the list might return the *new* version which is NOT installed yet.
			// So IsInstalled depends.
			// For "apt list --upgradable", it lists the new version. IsInstalled = false (for that version).
			// But current GUI logic displays "Installed" if IsInstalled is true.
			// If we want to show "Update Available", we should probably link it to the installed package.
			// Or just display it as a separate item "New Version"?
			// The Requirement: "dropdown list to let user choose to show 'installed', 'update availabe', or 'all'".
			// So list of upgradable packages should likely show the *available* updates.
		}
	}

	return pkgs, nil
}
