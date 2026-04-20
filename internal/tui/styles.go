package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors inspired by Claude's UI
	primaryColor   = lipgloss.Color("#D97706") // warm amber
	secondaryColor = lipgloss.Color("#92400E")
	bgColor        = lipgloss.Color("#1C1917")
	surfaceColor   = lipgloss.Color("#292524")
	textColor      = lipgloss.Color("#F5F5F4")
	mutedColor     = lipgloss.Color("#A8A29E")
	successColor   = lipgloss.Color("#22C55E")
	dangerColor    = lipgloss.Color("#EF4444")

	// Layout styles
	appStyle = lipgloss.NewStyle().
			Background(bgColor).
			Foreground(textColor)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			PaddingLeft(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			PaddingLeft(1)

	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#44403C")).
			Padding(0, 1)

	activePanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(primaryColor).
				Padding(0, 1)

	selectedItemStyle = lipgloss.NewStyle().
				Foreground(primaryColor).
				Bold(true)

	normalItemStyle = lipgloss.NewStyle().
			Foreground(textColor)

	statusRunningStyle = lipgloss.NewStyle().
				Foreground(successColor).
				Bold(true)

	statusStoppedStyle = lipgloss.NewStyle().
				Foreground(dangerColor).
				Bold(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			PaddingLeft(1).
			PaddingTop(1)

	inputLabelStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	inputStyle = lipgloss.NewStyle().
			Foreground(textColor)

	errorStyle = lipgloss.NewStyle().
			Foreground(dangerColor).
			Bold(true).
			PaddingLeft(1)

	successMsgStyle = lipgloss.NewStyle().
			Foreground(successColor).
			PaddingLeft(1)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(lipgloss.Color("#44403C")).
			Width(60).
			PaddingLeft(1).
			PaddingBottom(0)

	// Cloud-specific styles
	cloudColor = lipgloss.Color("#3B82F6") // blue for cloud

	cloudTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(cloudColor).
			PaddingLeft(1)

	tabActiveStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(primaryColor).
			PaddingLeft(1).
			PaddingRight(1)

	tabInactiveStyle = lipgloss.NewStyle().
				Foreground(mutedColor).
				PaddingLeft(1).
				PaddingRight(1)

	cloudStateRunning = lipgloss.NewStyle().
				Foreground(successColor).
				Bold(true)

	cloudStateStopped = lipgloss.NewStyle().
				Foreground(dangerColor).
				Bold(true)

	cloudStateTransient = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#F59E0B")). // yellow/amber for transitional states
				Bold(true)

	cloudStateDegraded = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#EF4444")).
				Bold(true)

	cloudInfoStyle = lipgloss.NewStyle().
			Foreground(cloudColor)

	cloudDetailLabel = lipgloss.NewStyle().
				Foreground(primaryColor).
				Bold(true).
				PaddingLeft(2)

	cloudDetailValue = lipgloss.NewStyle().
				Foreground(textColor)
)
