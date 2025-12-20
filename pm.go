package main

import (
	"fmt"
	"runtime"
)

type packageManager struct {
	Name string
	Path string
}

// var pm packageManager

func detectPM() []packageManager {
	var detectedPMs []packageManager
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
		checks := []string{"dpkg", "dpkg-query", "apt", "dnf", "pacman", "snap", "flatpak", "zypper", "yum", "apk", "xbps-install", "emerge", "nix-env", "brew", "port", "winget", "choco", "scoop"}
		for _, p := range checks {
			wrapperName := p
			if p == "xbps-install" {
				wrapperName = "xbps"
			}
			if ok, path := isInstalled(p); ok {
				detectedPMs = append(detectedPMs, packageManager{Name: wrapperName, Path: path})
			}
		}

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

	// if len(detectedPMs) > 0 {
	// 	pm = detectedPMs[0]
	// }

	return detectedPMs
}
