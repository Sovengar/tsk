package tui

import "charm.land/lipgloss/v2"

var (
	styleTitle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	styleSep   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	styleSelected = lipgloss.NewStyle().Bold(true)
	styleDim      = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Faint(true)
	styleWarn     = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	styleError    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))

	stylePriorityHigh   = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	stylePriorityMedium = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	stylePriorityLow    = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	stylePriorityNone   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	styleStatusActive = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))

	styleColumnHeader = lipgloss.NewStyle().Bold(true)

	styleFilterDim = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	styleProjectTag = lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Bold(true)

	styleStatusKey  = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	styleStatusDesc = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleStatusSep  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)
