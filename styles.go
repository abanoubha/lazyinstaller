package main

import "github.com/charmbracelet/lipgloss"

var (
	activeBorderColor   = lipgloss.Color("62")
	inactiveBorderColor = lipgloss.Color("240")

	// App-wide styles
	appStyle = lipgloss.NewStyle().Margin(1, 2)

	// List styles
	titleStyle = lipgloss.NewStyle().
			Background(activeBorderColor).
			Foreground(lipgloss.Color("230")).
			Padding(0, 1)

	inputBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(activeBorderColor).
			Padding(0, 1).
			Width(80)

	listBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(inactiveBorderColor).
			Padding(0, 1).
			Width(80).
			Height(20)

	// Navigation styles
	selectedItemStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("62")).
				Foreground(lipgloss.Color("230")).
				Bold(true)

	// Status Bar styles
	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1)
)
