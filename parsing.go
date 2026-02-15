package main

import (
	"bufio"
	"strings"
)

func parseAptOutput(output string) []Package {
	var pkgs []Package
	scanner := bufio.NewScanner(strings.NewReader(output))
	var currentPkg *Package

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line == "Sorting... Done" || line == "Full Text Search... Done" || strings.HasPrefix(line, "Listing...") {
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
			isUpgradable := strings.Contains(line, "[upgradable") // logic for apt list --upgradable

			currentPkg = &Package{
				Name:         name,
				Version:      version,
				Manager:      "apt",
				IsInstalled:  isInstalled,
				IsUpgradable: isUpgradable,
			}
			pkgs = append(pkgs, *currentPkg)
		}
	}
	if currentPkg != nil {
		pkgs = append(pkgs, *currentPkg)
	}
	return pkgs
}

func parseSnapOutput(output string) []Package {
	var pkgs []Package
	scanner := bufio.NewScanner(strings.NewReader(output))
	// Skip header check inside loop or before?
	// main.go implementation skips first line.
	first := true
	for scanner.Scan() {
		if first {
			first = false
			continue // Skip header: "Name  Version  Rev..."
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			pkgs = append(pkgs, Package{
				Name:        fields[0],
				Version:     fields[1],
				Manager:     "snap",
				IsInstalled: true, // usually 'snap list' implies installed
			})
		}
	}
	return pkgs
}

func parseFlatpakOutput(output string) []Package {
	var pkgs []Package
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 {
			pkgs = append(pkgs, Package{
				Name:        fields[0],
				Version:     fields[1],
				Manager:     "flatpak",
				IsInstalled: true,
			})
		}
	}
	return pkgs
}

func parsePacmanOutput(output string) []Package {
	var pkgs []Package
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 {
			pkgs = append(pkgs, Package{
				Name:        fields[0],
				Version:     fields[1],
				Manager:     "pacman",
				IsInstalled: true,
			})
		}
	}
	return pkgs

}

func parseBrewOutput(output string) []Package { // brew list --versions
	var pkgs []Package
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 {
			pkgs = append(pkgs, Package{
				Name:        fields[0],
				Version:     fields[1],
				Manager:     "brew",
				IsInstalled: true,
			})
		}
	}
	return pkgs
}

func parsePortOutput(output string) []Package {
	var pkgs []Package
	scanner := bufio.NewScanner(strings.NewReader(output))
	// Skip the first line: "The following ports are currently installed:"
	if scanner.Scan() {
		_ = scanner.Text()
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) >= 2 {
			name := parts[0]
			version := strings.TrimPrefix(parts[1], "@")

			pkgs = append(pkgs, Package{
				Name:        name,
				Version:     version,
				Manager:     "port",
				IsInstalled: true,
			})
		}
	}
	return pkgs
}

func parseDnfOutput(output string) []Package { // and rpm
	var pkgs []Package
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			pkgs = append(pkgs, Package{
				Name:        parts[0],
				Version:     parts[1],
				Manager:     "dnf", // or rpm
				IsInstalled: true,
			})
		}
	}
	return pkgs
}
