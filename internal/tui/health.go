package tui

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"clickhouse-tui/internal/health"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/NimbleMarkets/ntcharts/sparkline"
)

const (
	healthTickInterval = 3 * time.Second
	maxSparklinePoints = 60 // keep last 60 data points (~3 min at 3s interval)
)

// Messages for health tab
type healthMetricsMsg struct {
	metrics *health.Metrics
	err     error
}

type healthTickMsg struct{}

type healthConnectedMsg struct {
	err error
}

// healthState holds all state for the health dashboard.
type healthState struct {
	client    *health.Client
	connected bool
	loading   bool
	err       string
	metrics   *health.Metrics

	// Sparkline data (rolling window)
	qpsHistory    []float64
	memHistory    []float64
	cpuHistory    []float64

	// Connection selection
	connCursor int
}

func (m Model) updateHealth(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If not connected, we're on the connection picker
	if !m.health.connected && m.health.client == nil && !m.health.loading {
		return m.updateHealthConnPicker(msg)
	}

	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "tab":
		m.tab = tabLocal
		m.view = viewList
		return m, nil
	case "shift+tab":
		m.tab = tabCloud
		m.view = viewCloudServices
		return m, nil
	case "r":
		if m.health.connected {
			m.status = "Refreshing health metrics..."
			m.statusOk = true
			return m, m.fetchHealthMetrics()
		}
	case "esc":
		// Disconnect and go back to connection picker
		if m.health.client != nil {
			m.health.client.Close()
		}
		m.health = healthState{}
		return m, nil
	}
	return m, nil
}

func (m Model) updateHealthConnPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "tab":
		m.tab = tabLocal
		m.view = viewList
		return m, nil
	case "shift+tab":
		m.tab = tabCloud
		m.view = viewCloudServices
		return m, nil
	case "up", "k":
		if m.health.connCursor > 0 {
			m.health.connCursor--
		}
	case "down", "j":
		if m.health.connCursor < len(m.store.Connections)-1 {
			m.health.connCursor++
		}
	case "enter":
		if len(m.store.Connections) > 0 {
			conn := m.store.Connections[m.health.connCursor]
			m.health.loading = true
			m.status = fmt.Sprintf("Connecting to %s...", conn.Name)
			m.statusOk = true
			return m, func() tea.Msg {
				client, err := health.NewClient(conn)
				if err != nil {
					return healthConnectedMsg{err: err}
				}
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := client.Ping(ctx); err != nil {
					client.Close()
					return healthConnectedMsg{err: fmt.Errorf("ping failed: %w", err)}
				}
				return healthConnectedMsg{err: nil}
			}
		}
	}
	return m, nil
}

func (m Model) handleHealthConnected(msg healthConnectedMsg) (tea.Model, tea.Cmd) {
	m.health.loading = false
	if msg.err != nil {
		m.health.err = msg.err.Error()
		m.status = fmt.Sprintf("Connection failed: %v", msg.err)
		m.statusOk = false
		return m, nil
	}

	// Connection successful — create the client and start collecting
	conn := m.store.Connections[m.health.connCursor]
	client, err := health.NewClient(conn)
	if err != nil {
		m.health.err = err.Error()
		m.status = fmt.Sprintf("Connection failed: %v", err)
		m.statusOk = false
		return m, nil
	}

	m.health.client = client
	m.health.connected = true
	m.health.err = ""
	m.status = fmt.Sprintf("Connected to %s", conn.Name)
	m.statusOk = true

	return m, tea.Batch(m.fetchHealthMetrics(), m.healthTick())
}

func (m Model) handleHealthMetrics(msg healthMetricsMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.health.err = msg.err.Error()
		m.status = fmt.Sprintf("Metrics error: %v", msg.err)
		m.statusOk = false
		// Still schedule the next tick so we retry
		return m, m.healthTick()
	}

	m.health.metrics = msg.metrics
	m.health.err = ""

	// Append to sparkline histories
	m.health.qpsHistory = appendCapped(m.health.qpsHistory, msg.metrics.QueriesPerSec, maxSparklinePoints)
	m.health.memHistory = appendCapped(m.health.memHistory, msg.metrics.MemoryUsageMB, maxSparklinePoints)
	m.health.cpuHistory = appendCapped(m.health.cpuHistory, msg.metrics.CPUPercent, maxSparklinePoints)

	return m, m.healthTick()
}

func (m Model) fetchHealthMetrics() tea.Cmd {
	return func() tea.Msg {
		if m.health.client == nil {
			return healthMetricsMsg{err: fmt.Errorf("not connected")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		metrics, err := m.health.client.Collect(ctx)
		return healthMetricsMsg{metrics: metrics, err: err}
	}
}

func (m Model) healthTick() tea.Cmd {
	return tea.Tick(healthTickInterval, func(time.Time) tea.Msg {
		return healthTickMsg{}
	})
}

func (m Model) renderHealth() string {
	// Connection picker
	if !m.health.connected && m.health.client == nil && !m.health.loading {
		return m.renderHealthConnPicker()
	}

	if m.health.loading {
		return subtitleStyle.Render("  Connecting...")
	}

	if m.health.metrics == nil {
		msg := "  Waiting for metrics..."
		if m.health.err != "" {
			msg = fmt.Sprintf("  Error: %s", m.health.err)
		}
		return subtitleStyle.Render(msg)
	}

	met := m.health.metrics
	connName := ""
	if m.health.connCursor < len(m.store.Connections) {
		connName = m.store.Connections[m.health.connCursor].Name
	}

	panelWidth := min(m.width-4, 78)
	innerWidth := panelWidth - 4 // account for panel border + padding

	var sections []string

	// -- Header --
	header := healthTitleStyle.Render(fmt.Sprintf("  %s  —  ClickHouse %s  —  uptime %s",
		connName, met.Version, formatUptime(met.Uptime)))
	sections = append(sections, header)
	sections = append(sections, "")

	// -- Key Metrics Row --
	metricBoxWidth := (innerWidth - 6) / 4 // 4 boxes with gaps
	if metricBoxWidth < 14 {
		metricBoxWidth = 14
	}

	qpsBox := renderMetricBox("Queries/s", fmt.Sprintf("%.1f", met.QueriesPerSec), healthAccentColor, metricBoxWidth)
	activeBox := renderMetricBox("Active", fmt.Sprintf("%d", met.CurrentQueryCount), healthAccentColor, metricBoxWidth)
	memBox := renderMetricBox("Memory", fmt.Sprintf("%.0f MB", met.MemoryUsageMB), memColor(met.MemoryPercent), metricBoxWidth)
	cpuBox := renderMetricBox("CPU", fmt.Sprintf("%.1f%%", met.CPUPercent), cpuColor(met.CPUPercent), metricBoxWidth)

	metricsRow := lipgloss.JoinHorizontal(lipgloss.Top, qpsBox, " ", activeBox, " ", memBox, " ", cpuBox)
	sections = append(sections, metricsRow)
	sections = append(sections, "")

	// -- Sparklines --
	sparkWidth := (innerWidth - 3) / 2
	if sparkWidth < 20 {
		sparkWidth = 20
	}

	qpsSpark := renderSparkline("Queries/sec", m.health.qpsHistory, sparkWidth, 5, healthAccentColor)
	memSpark := renderSparkline("Memory (MB)", m.health.memHistory, sparkWidth, 5, lipgloss.Color("#3B82F6"))

	sparkRow := lipgloss.JoinHorizontal(lipgloss.Top, qpsSpark, "   ", memSpark)
	sections = append(sections, sparkRow)
	sections = append(sections, "")

	// -- Storage & Replication Row --
	var storageLines []string
	storageLines = append(storageLines, healthLabelStyle.Render("Storage"))
	storageLines = append(storageLines, fmt.Sprintf("  %s %s",
		healthDetailLabel.Render("Disk Used:"), healthDetailVal.Render(fmt.Sprintf("%.2f GB", met.DiskUsedGB))))
	storageLines = append(storageLines, fmt.Sprintf("  %s %s",
		healthDetailLabel.Render("Parts:    "), healthDetailVal.Render(fmt.Sprintf("%d active", met.TotalParts))))
	storageLines = append(storageLines, fmt.Sprintf("  %s %s",
		healthDetailLabel.Render("Merges:   "), healthDetailVal.Render(fmt.Sprintf("%d running", met.TotalMerges))))

	var throughputLines []string
	throughputLines = append(throughputLines, healthLabelStyle.Render("Throughput (10s)"))
	throughputLines = append(throughputLines, fmt.Sprintf("  %s %s",
		healthDetailLabel.Render("Read Rows:    "), healthDetailVal.Render(formatCount(met.ReadRows))))
	throughputLines = append(throughputLines, fmt.Sprintf("  %s %s",
		healthDetailLabel.Render("Inserted Rows:"), healthDetailVal.Render(formatCount(met.InsertedRows))))
	throughputLines = append(throughputLines, fmt.Sprintf("  %s %s",
		healthDetailLabel.Render("Repl. Queue:  "), healthDetailVal.Render(fmt.Sprintf("%d", met.ReplicaQueueSize))))

	halfWidth := (innerWidth - 3) / 2
	storagePanel := lipgloss.NewStyle().Width(halfWidth).Render(
		lipgloss.JoinVertical(lipgloss.Left, storageLines...))
	throughputPanel := lipgloss.NewStyle().Width(halfWidth).Render(
		lipgloss.JoinVertical(lipgloss.Left, throughputLines...))

	infoRow := lipgloss.JoinHorizontal(lipgloss.Top, storagePanel, "   ", throughputPanel)
	sections = append(sections, infoRow)

	// -- Memory bar --
	sections = append(sections, "")
	memBar := renderProgressBar("Memory", met.MemoryPercent, innerWidth-2)
	sections = append(sections, memBar)

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	return healthPanelStyle.Width(panelWidth).Render(content)
}

func (m Model) renderHealthConnPicker() string {
	if len(m.store.Connections) == 0 {
		return subtitleStyle.Render("  No connections configured. Go to Local tab and add one first.")
	}

	var rows []string
	rows = append(rows, healthTitleStyle.Render("Select Connection to Monitor"))
	rows = append(rows, "")

	for i, conn := range m.store.Connections {
		cursor := "  "
		style := normalItemStyle
		if i == m.health.connCursor {
			cursor = " >"
			style = selectedItemStyle
		}
		line := fmt.Sprintf("%s %s  %s",
			cursor,
			style.Render(conn.Name),
			subtitleStyle.Render(fmt.Sprintf("%s@%s:%s", conn.User, conn.Host, conn.Port)))
		rows = append(rows, line)
	}

	if m.health.err != "" {
		rows = append(rows, "")
		rows = append(rows, errorStyle.Render("  "+m.health.err))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return activePanelStyle.Width(min(m.width-4, 60)).Render(content)
}

// -- Rendering helpers --

func renderMetricBox(label, value string, color lipgloss.Color, width int) string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(color).
		Width(width).
		Align(lipgloss.Center).
		Padding(0, 1)

	valStyle := lipgloss.NewStyle().
		Foreground(color).
		Bold(true)

	lblStyle := lipgloss.NewStyle().
		Foreground(mutedColor)

	content := valStyle.Render(value) + "\n" + lblStyle.Render(label)
	return boxStyle.Render(content)
}

func renderSparkline(title string, data []float64, width, height int, color lipgloss.Color) string {
	titleStr := lipgloss.NewStyle().
		Foreground(color).
		Bold(true).
		Render(title)

	if len(data) == 0 {
		placeholder := lipgloss.NewStyle().
			Foreground(mutedColor).
			Width(width).
			Height(height).
			Align(lipgloss.Center).
			Render("waiting for data...")
		return lipgloss.JoinVertical(lipgloss.Left, titleStr, placeholder)
	}

	sl := sparkline.New(width, height,
		sparkline.WithStyle(lipgloss.NewStyle().Foreground(color)))

	// Push data points
	for _, v := range data {
		sl.Push(v)
	}

	return lipgloss.JoinVertical(lipgloss.Left, titleStr, sl.View())
}

func renderProgressBar(label string, percent float64, width int) string {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	barWidth := width - 20 // space for label + percentage
	if barWidth < 10 {
		barWidth = 10
	}

	filled := int(math.Round(float64(barWidth) * percent / 100))
	if filled > barWidth {
		filled = barWidth
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	color := successColor
	if percent > 80 {
		color = dangerColor
	} else if percent > 60 {
		color = lipgloss.Color("#F59E0B")
	}

	barStyled := lipgloss.NewStyle().Foreground(color).Render(bar)
	lblStyled := healthDetailLabel.Render(fmt.Sprintf("  %-8s", label))
	pctStyled := lipgloss.NewStyle().Foreground(color).Bold(true).Render(fmt.Sprintf(" %5.1f%%", percent))

	return lblStyled + barStyled + pctStyled
}

func formatUptime(seconds uint64) string {
	d := seconds / 86400
	h := (seconds % 86400) / 3600
	m := (seconds % 3600) / 60
	if d > 0 {
		return fmt.Sprintf("%dd %dh %dm", d, h, m)
	}
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

func formatCount(n uint64) string {
	if n >= 1_000_000_000 {
		return fmt.Sprintf("%.1fB", float64(n)/1_000_000_000)
	}
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

func appendCapped(slice []float64, val float64, maxLen int) []float64 {
	slice = append(slice, val)
	if len(slice) > maxLen {
		slice = slice[len(slice)-maxLen:]
	}
	return slice
}

func memColor(pct float64) lipgloss.Color {
	if pct > 80 {
		return lipgloss.Color("#EF4444")
	}
	if pct > 60 {
		return lipgloss.Color("#F59E0B")
	}
	return lipgloss.Color("#22C55E")
}

func cpuColor(pct float64) lipgloss.Color {
	if pct > 80 {
		return lipgloss.Color("#EF4444")
	}
	if pct > 50 {
		return lipgloss.Color("#F59E0B")
	}
	return lipgloss.Color("#22C55E")
}
