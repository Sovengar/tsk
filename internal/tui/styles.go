package tui

import "charm.land/lipgloss/v2"

var (
	styleTitle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	styleSep   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	styleTabActive   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(lipgloss.Color("12"))
	styleTabInactive = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	styleSelected = lipgloss.NewStyle().Bold(true)
	styleDim      = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Faint(true)
	styleHelp     = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Faint(true)
	styleWarn     = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	styleError    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))

	stylePriorityHigh   = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	stylePriorityMedium = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	stylePriorityLow    = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	stylePriorityNone   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	styleStatusActive = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	styleStatusDone   = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Faint(true)
	styleStatusCancel = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Strikethrough(true)

	styleColumnHeader = lipgloss.NewStyle().Bold(true)
	styleKanbanCard   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder(), true).Padding(0, 1)
	styleKanbanCursor = lipgloss.NewStyle().Border(lipgloss.DoubleBorder(), true).Padding(0, 1).Bold(true)

	styleFilterActive = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	styleFilterDim    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	styleProjectTag = lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Bold(true)
	styleAssignee   = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))

	styleStatusKey  = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	styleStatusDesc = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleStatusSep  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)
