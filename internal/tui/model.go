package tui

import (
	"fmt"
	"strings"

	"clickhouse-tui/internal/cloud"
	"clickhouse-tui/internal/config"
	"clickhouse-tui/internal/service"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type view int

const (
	viewList view = iota
	viewAdd
	viewConfirmDelete
	viewCloudServices
	viewCloudSetup
	viewCloudDetail
)

type tab int

const (
	tabLocal tab = iota
	tabCloud
)

type statusMsg struct {
	msg string
	err bool
}

type serviceResultMsg struct {
	msg string
	err bool
}

type cloudServicesMsg struct {
	services []cloud.Service
	err      error
}

type cloudOrgsMsg struct {
	orgs []cloud.Organization
	err  error
}

type cloudActionMsg struct {
	msg string
	err bool
}

type Model struct {
	store    *config.Store
	cursor   int
	view     view
	tab      tab
	width    int
	height   int
	status   string
	statusOk bool

	// Add form
	inputs     []textinput.Model
	focusIndex int

	// Confirm delete
	confirmDelete bool

	// Cloud
	cloudClient   *cloud.Client
	cloudServices []cloud.Service
	cloudCursor   int
	cloudLoading  bool
	cloudInputs   []textinput.Model
	cloudFocus    int
}

var addFormLabels = []string{"Name", "Host", "Port", "User", "Password", "Database"}
var addFormDefaults = []string{"", "localhost", "9000", "default", "", "default"}

var cloudSetupLabels = []string{"API Key", "API Secret"}

func NewModel(store *config.Store) Model {
	inputs := make([]textinput.Model, len(addFormLabels))
	for i := range inputs {
		t := textinput.New()
		t.Placeholder = addFormDefaults[i]
		t.CharLimit = 128
		t.Width = 30
		if i == 4 { // password
			t.EchoMode = textinput.EchoPassword
		}
		inputs[i] = t
	}
	inputs[0].Focus()

	cloudInputs := make([]textinput.Model, len(cloudSetupLabels))
	for i := range cloudInputs {
		t := textinput.New()
		t.CharLimit = 128
		t.Width = 50
		t.EchoMode = textinput.EchoPassword
		cloudInputs[i] = t
	}

	m := Model{
		store:       store,
		inputs:      inputs,
		cloudInputs: cloudInputs,
	}

	// Initialize cloud client if credentials exist
	if store.HasCloud() {
		m.cloudClient = cloud.NewClient(store.Cloud.APIKey, store.Cloud.APISecret)
	}

	return m
}

func (m Model) Init() tea.Cmd {
	return tea.SetWindowTitle("ClickHouse TUI")
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case serviceResultMsg:
		m.status = msg.msg
		m.statusOk = !msg.err
		return m, nil

	case cloudServicesMsg:
		m.cloudLoading = false
		if msg.err != nil {
			m.status = fmt.Sprintf("Cloud error: %v", msg.err)
			m.statusOk = false
		} else {
			m.cloudServices = msg.services
			m.status = fmt.Sprintf("Loaded %d cloud service(s)", len(msg.services))
			m.statusOk = true
		}
		return m, nil

	case cloudOrgsMsg:
		if msg.err != nil {
			m.status = fmt.Sprintf("Cloud error: %v", msg.err)
			m.statusOk = false
			m.cloudLoading = false
			return m, nil
		}
		if len(msg.orgs) == 0 {
			m.status = "No organizations found"
			m.statusOk = false
			m.cloudLoading = false
			return m, nil
		}
		// Save the first org ID and fetch services
		m.store.Cloud.OrgID = msg.orgs[0].ID
		_ = m.store.Save()
		return m, m.fetchCloudServices()

	case cloudActionMsg:
		m.status = msg.msg
		m.statusOk = !msg.err
		if !msg.err {
			// Refresh services after action
			return m, m.fetchCloudServices()
		}
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		switch m.view {
		case viewList:
			return m.updateList(msg)
		case viewAdd:
			return m.updateAddForm(msg)
		case viewConfirmDelete:
			return m.updateConfirmDelete(msg)
		case viewCloudServices:
			return m.updateCloudServices(msg)
		case viewCloudSetup:
			return m.updateCloudSetup(msg)
		case viewCloudDetail:
			return m.updateCloudDetail(msg)
		}
	}

	return m, nil
}

func (m Model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.store.Connections)-1 {
			m.cursor++
		}
	case "a":
		m.view = viewAdd
		m.focusIndex = 0
		for i := range m.inputs {
			m.inputs[i].SetValue("")
			m.inputs[i].Blur()
		}
		m.inputs[0].Focus()
		return m, m.inputs[0].Cursor.BlinkCmd()
	case "d":
		if len(m.store.Connections) > 0 {
			m.view = viewConfirmDelete
			m.confirmDelete = false
		}
	case "s":
		if len(m.store.Connections) > 0 {
			conn := m.store.Connections[m.cursor]
			status := service.Check(conn)
			if status == service.StatusRunning {
				return m, func() tea.Msg {
					err := service.Stop(conn)
					if err != nil {
						return serviceResultMsg{fmt.Sprintf("Error stopping %s: %v", conn.Name, err), true}
					}
					return serviceResultMsg{fmt.Sprintf("Stopped %s", conn.Name), false}
				}
			} else {
				return m, func() tea.Msg {
					err := service.Start(conn)
					if err != nil {
						return serviceResultMsg{fmt.Sprintf("Error starting %s: %v", conn.Name, err), true}
					}
					return serviceResultMsg{fmt.Sprintf("Started %s", conn.Name), false}
				}
			}
		}
	case "tab":
		m.tab = tabCloud
		m.view = viewCloudServices
		m.cloudCursor = 0
		if m.cloudClient == nil {
			// No credentials, show setup
			m.view = viewCloudSetup
			m.cloudFocus = 0
			for i := range m.cloudInputs {
				m.cloudInputs[i].SetValue("")
				m.cloudInputs[i].Blur()
			}
			m.cloudInputs[0].Focus()
			return m, m.cloudInputs[0].Cursor.BlinkCmd()
		}
		if len(m.cloudServices) == 0 {
			m.cloudLoading = true
			return m, m.loadCloudOrgsAndServices()
		}
		return m, nil
	}
	return m, nil
}

func (m Model) updateCloudServices(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "up", "k":
		if m.cloudCursor > 0 {
			m.cloudCursor--
		}
	case "down", "j":
		if m.cloudCursor < len(m.cloudServices)-1 {
			m.cloudCursor++
		}
	case "tab":
		m.tab = tabLocal
		m.view = viewList
		return m, nil
	case "s":
		if len(m.cloudServices) > 0 {
			svc := m.cloudServices[m.cloudCursor]
			orgID := m.store.Cloud.OrgID
			if svc.IsRunning() {
				m.status = fmt.Sprintf("Stopping %s...", svc.Name)
				m.statusOk = true
				return m, func() tea.Msg {
					err := m.cloudClient.StopService(orgID, svc.ID)
					if err != nil {
						return cloudActionMsg{fmt.Sprintf("Error stopping %s: %v", svc.Name, err), true}
					}
					return cloudActionMsg{fmt.Sprintf("Stop requested for %s", svc.Name), false}
				}
			} else if svc.IsStopped() {
				m.status = fmt.Sprintf("Starting %s...", svc.Name)
				m.statusOk = true
				return m, func() tea.Msg {
					err := m.cloudClient.StartService(orgID, svc.ID)
					if err != nil {
						return cloudActionMsg{fmt.Sprintf("Error starting %s: %v", svc.Name, err), true}
					}
					return cloudActionMsg{fmt.Sprintf("Start requested for %s", svc.Name), false}
				}
			}
		}
	case "r":
		m.cloudLoading = true
		m.status = "Refreshing..."
		m.statusOk = true
		return m, m.fetchCloudServices()
	case "enter":
		if len(m.cloudServices) > 0 {
			m.view = viewCloudDetail
		}
		return m, nil
	case "c":
		// Reconfigure cloud credentials
		m.view = viewCloudSetup
		m.cloudFocus = 0
		for i := range m.cloudInputs {
			m.cloudInputs[i].SetValue("")
			m.cloudInputs[i].Blur()
		}
		m.cloudInputs[0].Focus()
		return m, m.cloudInputs[0].Cursor.BlinkCmd()
	}
	return m, nil
}

func (m Model) updateCloudDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "backspace":
		m.view = viewCloudServices
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) updateCloudSetup(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		if m.cloudClient != nil {
			m.view = viewCloudServices
		} else {
			m.tab = tabLocal
			m.view = viewList
		}
		return m, nil
	case "tab", "down":
		m.cloudInputs[m.cloudFocus].Blur()
		m.cloudFocus = (m.cloudFocus + 1) % len(m.cloudInputs)
		m.cloudInputs[m.cloudFocus].Focus()
		return m, m.cloudInputs[m.cloudFocus].Cursor.BlinkCmd()
	case "shift+tab", "up":
		m.cloudInputs[m.cloudFocus].Blur()
		m.cloudFocus = (m.cloudFocus - 1 + len(m.cloudInputs)) % len(m.cloudInputs)
		m.cloudInputs[m.cloudFocus].Focus()
		return m, m.cloudInputs[m.cloudFocus].Cursor.BlinkCmd()
	case "enter":
		if m.cloudFocus == len(m.cloudInputs)-1 {
			return m.submitCloudSetup()
		}
		m.cloudInputs[m.cloudFocus].Blur()
		m.cloudFocus++
		m.cloudInputs[m.cloudFocus].Focus()
		return m, m.cloudInputs[m.cloudFocus].Cursor.BlinkCmd()
	}

	var cmd tea.Cmd
	m.cloudInputs[m.cloudFocus], cmd = m.cloudInputs[m.cloudFocus].Update(msg)
	return m, cmd
}

func (m Model) submitCloudSetup() (tea.Model, tea.Cmd) {
	apiKey := m.cloudInputs[0].Value()
	apiSecret := m.cloudInputs[1].Value()

	if apiKey == "" || apiSecret == "" {
		m.status = "API Key and Secret are required"
		m.statusOk = false
		return m, nil
	}

	creds := config.CloudCredentials{
		APIKey:    apiKey,
		APISecret: apiSecret,
	}

	if err := m.store.SetCloud(creds); err != nil {
		m.status = fmt.Sprintf("Error saving credentials: %v", err)
		m.statusOk = false
		return m, nil
	}

	m.cloudClient = cloud.NewClient(apiKey, apiSecret)
	m.view = viewCloudServices
	m.cloudLoading = true
	m.status = "Connecting to ClickHouse Cloud..."
	m.statusOk = true

	return m, m.loadCloudOrgsAndServices()
}

func (m Model) loadCloudOrgsAndServices() tea.Cmd {
	return func() tea.Msg {
		if m.store.Cloud.OrgID != "" {
			services, err := m.cloudClient.ListServices(m.store.Cloud.OrgID)
			if err != nil {
				return cloudServicesMsg{nil, err}
			}
			return cloudServicesMsg{services, nil}
		}
		orgs, err := m.cloudClient.ListOrganizations()
		if err != nil {
			return cloudOrgsMsg{nil, err}
		}
		return cloudOrgsMsg{orgs, nil}
	}
}

func (m Model) fetchCloudServices() tea.Cmd {
	return func() tea.Msg {
		services, err := m.cloudClient.ListServices(m.store.Cloud.OrgID)
		return cloudServicesMsg{services, err}
	}
}

func (m Model) updateAddForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.view = viewList
		return m, nil
	case "tab", "down":
		m.inputs[m.focusIndex].Blur()
		m.focusIndex = (m.focusIndex + 1) % len(m.inputs)
		m.inputs[m.focusIndex].Focus()
		return m, m.inputs[m.focusIndex].Cursor.BlinkCmd()
	case "shift+tab", "up":
		m.inputs[m.focusIndex].Blur()
		m.focusIndex = (m.focusIndex - 1 + len(m.inputs)) % len(m.inputs)
		m.inputs[m.focusIndex].Focus()
		return m, m.inputs[m.focusIndex].Cursor.BlinkCmd()
	case "enter":
		if m.focusIndex == len(m.inputs)-1 {
			return m.submitAddForm()
		}
		m.inputs[m.focusIndex].Blur()
		m.focusIndex++
		m.inputs[m.focusIndex].Focus()
		return m, m.inputs[m.focusIndex].Cursor.BlinkCmd()
	}

	// Update the focused input
	var cmd tea.Cmd
	m.inputs[m.focusIndex], cmd = m.inputs[m.focusIndex].Update(msg)
	return m, cmd
}

func (m Model) submitAddForm() (tea.Model, tea.Cmd) {
	val := func(i int) string {
		v := m.inputs[i].Value()
		if v == "" {
			return addFormDefaults[i]
		}
		return v
	}

	name := val(0)
	if name == "" {
		m.status = "Name is required"
		m.statusOk = false
		return m, nil
	}

	conn := config.Connection{
		Name:     name,
		Host:     val(1),
		Port:     val(2),
		User:     val(3),
		Password: val(4),
		Database: val(5),
	}

	if err := m.store.Add(conn); err != nil {
		m.status = fmt.Sprintf("Error: %v", err)
		m.statusOk = false
		return m, nil
	}

	m.status = fmt.Sprintf("Added connection %q", conn.Name)
	m.statusOk = true
	m.view = viewList
	m.cursor = len(m.store.Connections) - 1
	return m, nil
}

func (m Model) updateConfirmDelete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		name := m.store.Connections[m.cursor].Name
		if err := m.store.Delete(name); err != nil {
			m.status = fmt.Sprintf("Error: %v", err)
			m.statusOk = false
		} else {
			m.status = fmt.Sprintf("Deleted %q", name)
			m.statusOk = true
		}
		if m.cursor >= len(m.store.Connections) && m.cursor > 0 {
			m.cursor--
		}
		m.view = viewList
	case "n", "N", "esc":
		m.view = viewList
	}
	return m, nil
}

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	var b strings.Builder

	// Header
	header := headerStyle.Render("  ClickHouse TUI")
	b.WriteString(header)
	b.WriteString("\n")

	// Tabs
	b.WriteString(m.renderTabs())
	b.WriteString("\n")

	switch m.view {
	case viewList:
		b.WriteString(m.renderList())
	case viewAdd:
		b.WriteString(m.renderAddForm())
	case viewConfirmDelete:
		b.WriteString(m.renderConfirmDelete())
	case viewCloudServices:
		b.WriteString(m.renderCloudServices())
	case viewCloudSetup:
		b.WriteString(m.renderCloudSetup())
	case viewCloudDetail:
		b.WriteString(m.renderCloudDetail())
	}

	// Status bar
	if m.status != "" {
		b.WriteString("\n")
		if m.statusOk {
			b.WriteString(successMsgStyle.Render("  " + m.status))
		} else {
			b.WriteString(errorStyle.Render("  " + m.status))
		}
	}

	// Help
	b.WriteString("\n")
	b.WriteString(m.renderHelp())

	return appStyle.Width(m.width).Height(m.height).Render(b.String())
}

func (m Model) renderTabs() string {
	localTab := tabInactiveStyle.Render("Local")
	cloudTab := tabInactiveStyle.Render("Cloud")

	if m.tab == tabLocal {
		localTab = tabActiveStyle.Render("Local")
	} else {
		cloudTab = tabActiveStyle.Render("Cloud")
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, "  ", localTab, "  ", cloudTab)
}

func (m Model) renderList() string {
	if len(m.store.Connections) == 0 {
		return subtitleStyle.Render("  No connections configured. Press 'a' to add one.")
	}

	var rows []string
	for i, conn := range m.store.Connections {
		cursor := "  "
		style := normalItemStyle
		if i == m.cursor {
			cursor = " >"
			style = selectedItemStyle
		}

		status := service.Check(conn)
		var statusStr string
		if status == service.StatusRunning {
			statusStr = statusRunningStyle.Render("● Running")
		} else {
			statusStr = statusStoppedStyle.Render("○ Stopped")
		}

		line := fmt.Sprintf("%s %s  %s@%s:%s/%s  %s",
			cursor,
			style.Render(conn.Name),
			subtitleStyle.Render(conn.User),
			subtitleStyle.Render(conn.Host),
			subtitleStyle.Render(conn.Port),
			subtitleStyle.Render(conn.Database),
			statusStr,
		)
		rows = append(rows, line)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return panelStyle.Width(min(m.width-4, 72)).Render(content)
}

func (m Model) renderAddForm() string {
	var rows []string
	rows = append(rows, titleStyle.Render("Add Connection"))
	rows = append(rows, "")

	for i, label := range addFormLabels {
		lbl := inputLabelStyle.Render(fmt.Sprintf("  %-10s", label))
		field := m.inputs[i].View()
		rows = append(rows, fmt.Sprintf("%s %s", lbl, field))
	}

	rows = append(rows, "")
	rows = append(rows, subtitleStyle.Render("  Enter to submit  |  Esc to cancel"))

	content := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return activePanelStyle.Width(min(m.width-4, 60)).Render(content)
}

func (m Model) renderConfirmDelete() string {
	name := m.store.Connections[m.cursor].Name
	msg := fmt.Sprintf("  Delete connection %q? (y/n)", name)
	content := lipgloss.NewStyle().Foreground(dangerColor).Bold(true).Render(msg)
	return activePanelStyle.Width(min(m.width-4, 60)).Render(content)
}

func (m Model) renderCloudServices() string {
	if m.cloudLoading {
		return subtitleStyle.Render("  Loading cloud services...")
	}

	if len(m.cloudServices) == 0 {
		return subtitleStyle.Render("  No cloud services found. Press 'c' to configure credentials.")
	}

	var rows []string
	for i, svc := range m.cloudServices {
		cursor := "  "
		style := normalItemStyle
		if i == m.cloudCursor {
			cursor = " >"
			style = selectedItemStyle
		}

		stateStr := renderCloudState(svc.State)

		providerRegion := subtitleStyle.Render(fmt.Sprintf("%s/%s", svc.Provider, svc.Region))

		line := fmt.Sprintf("%s %s  %s  %s  %s",
			cursor,
			style.Render(svc.Name),
			subtitleStyle.Render(svc.Tier),
			providerRegion,
			stateStr,
		)
		rows = append(rows, line)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return panelStyle.Width(min(m.width-4, 80)).Render(content)
}

func (m Model) renderCloudDetail() string {
	if m.cloudCursor >= len(m.cloudServices) {
		return ""
	}

	svc := m.cloudServices[m.cloudCursor]

	var rows []string
	rows = append(rows, cloudTitleStyle.Render(fmt.Sprintf("Service: %s", svc.Name)))
	rows = append(rows, "")

	rows = append(rows, fmt.Sprintf("%s %s",
		cloudDetailLabel.Render("ID:       "), cloudDetailValue.Render(svc.ID)))
	rows = append(rows, fmt.Sprintf("%s %s",
		cloudDetailLabel.Render("State:    "), renderCloudState(svc.State)))
	rows = append(rows, fmt.Sprintf("%s %s",
		cloudDetailLabel.Render("Provider: "), cloudDetailValue.Render(svc.Provider)))
	rows = append(rows, fmt.Sprintf("%s %s",
		cloudDetailLabel.Render("Region:   "), cloudDetailValue.Render(svc.Region)))
	rows = append(rows, fmt.Sprintf("%s %s",
		cloudDetailLabel.Render("Tier:     "), cloudDetailValue.Render(svc.Tier)))

	if len(svc.Endpoints) > 0 {
		rows = append(rows, "")
		rows = append(rows, cloudTitleStyle.Render("Endpoints:"))
		for _, ep := range svc.Endpoints {
			rows = append(rows, fmt.Sprintf("%s %s",
				cloudDetailLabel.Render(fmt.Sprintf("  %-8s", ep.Protocol)),
				cloudDetailValue.Render(fmt.Sprintf("%s:%d", ep.Host, ep.Port))))
		}
	}

	rows = append(rows, "")
	rows = append(rows, subtitleStyle.Render("  Esc to go back"))

	content := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return activePanelStyle.Width(min(m.width-4, 72)).Render(content)
}

func (m Model) renderCloudSetup() string {
	var rows []string
	rows = append(rows, cloudTitleStyle.Render("ClickHouse Cloud Setup"))
	rows = append(rows, "")
	rows = append(rows, subtitleStyle.Render("  Enter your ClickHouse Cloud API credentials."))
	rows = append(rows, subtitleStyle.Render("  Get them at: clickhouse.cloud > Settings > API Keys"))
	rows = append(rows, "")

	for i, label := range cloudSetupLabels {
		lbl := inputLabelStyle.Render(fmt.Sprintf("  %-12s", label))
		field := m.cloudInputs[i].View()
		rows = append(rows, fmt.Sprintf("%s %s", lbl, field))
	}

	rows = append(rows, "")
	rows = append(rows, subtitleStyle.Render("  Enter to submit  |  Esc to cancel"))

	content := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return activePanelStyle.Width(min(m.width-4, 65)).Render(content)
}

func renderCloudState(state string) string {
	switch state {
	case cloud.StateRunning:
		return cloudStateRunning.Render("● Running")
	case cloud.StateStopped:
		return cloudStateStopped.Render("○ Stopped")
	case cloud.StateIdle:
		return cloudStateStopped.Render("◌ Idle")
	case cloud.StateStarting, cloud.StateAwaking:
		return cloudStateTransient.Render("◐ Starting")
	case cloud.StateStopping:
		return cloudStateTransient.Render("◑ Stopping")
	case cloud.StateProvisioning:
		return cloudStateTransient.Render("◓ Provisioning")
	case cloud.StateDegraded:
		return cloudStateDegraded.Render("✖ Degraded")
	default:
		return cloudStateTransient.Render("? " + state)
	}
}

func (m Model) renderHelp() string {
	switch m.view {
	case viewList:
		return helpStyle.Render("  j/k: navigate  |  a: add  |  d: delete  |  s: start/stop  |  tab: cloud  |  q: quit")
	case viewAdd:
		return helpStyle.Render("  tab: next field  |  enter: submit  |  esc: cancel")
	case viewCloudServices:
		return helpStyle.Render("  j/k: navigate  |  enter: details  |  s: start/stop  |  r: refresh  |  c: config  |  tab: local  |  q: quit")
	case viewCloudSetup:
		return helpStyle.Render("  tab: next field  |  enter: submit  |  esc: cancel")
	case viewCloudDetail:
		return helpStyle.Render("  esc: back  |  q: quit")
	default:
		return helpStyle.Render("  y: confirm  |  n: cancel")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
