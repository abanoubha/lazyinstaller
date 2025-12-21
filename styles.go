package main

import "github.com/charmbracelet/lipgloss"

var (
	activeBorderColor   = lipgloss.Color("62")
	inactiveBorderColor = lipgloss.Color("240")

	// App-wide styles
	appStyle = lipgloss.NewStyle()

	// List styles
	titleStyle = lipgloss.NewStyle().
			Background(activeBorderColor).
			Foreground(lipgloss.Color("230"))

	inputBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(activeBorderColor)

	listBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(inactiveBorderColor)

	// Navigation styles
	selectedItemStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("62")).
				Foreground(lipgloss.Color("230")).
				Bold(true)

	// Status Bar styles
	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	// Command Bar styles
	commandBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))
)
