package main

import (
	"fmt"
	"os"

	"clickhouse-tui/internal/config"
	"clickhouse-tui/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	store, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	model := tui.NewModel(store)
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
