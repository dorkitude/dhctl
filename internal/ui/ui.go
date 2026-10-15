// Package ui holds dhctl's lipgloss styles.
package ui

import "github.com/charmbracelet/lipgloss"

var (
	TitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7B61FF"))
	SubtleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	okStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#3FB950"))
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#F85149"))
	warnStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#D29922"))
	HeaderStyle = lipgloss.NewStyle().Bold(true)
)

func OK(s string) string   { return okStyle.Render("✓ ") + s }
func Err(s string) string  { return errStyle.Render("✗ ") + s }
func Warn(s string) string { return warnStyle.Render("! ") + s }
