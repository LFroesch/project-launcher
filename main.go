package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Project struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Command string `json:"command"`
}

type statusMsg struct {
	message string
}

func showStatus(msg string) tea.Cmd {
	return func() tea.Msg {
		return statusMsg{message: msg}
	}
}

type model struct {
	projects     []Project
	table        table.Model
	editMode     bool
	editRow      int
	editCol      int
	textInput    textinput.Model
	configFile   string
	width        int
	height       int
	statusMsg    string
	statusExpiry time.Time
}

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	configFile := filepath.Join(homeDir, ".project-launcher.json")

	m := model{
		projects:   loadProjects(configFile),
		configFile: configFile,
		width:      100,
		height:     24,
		editMode:   false,
		editRow:    -1,
		editCol:    -1,
	}

	// Initialize text input for editing
	m.textInput = textinput.New()
	m.textInput.CharLimit = 200

	// Initialize table like Portmon
	columns := []table.Column{
		{Title: "Name", Width: 25},
		{Title: "Path", Width: 40},
		{Title: "Command", Width: 25},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	m.table = t
	m.updateTable()

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}

func loadProjects(configFile string) []Project {
	var projects []Project
	data, err := os.ReadFile(configFile)
	if err != nil {
		return projects
	}
	json.Unmarshal(data, &projects)
	return projects
}

func (m *model) saveProjects() {
	data, err := json.MarshalIndent(m.projects, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(m.configFile, data, 0644)
}

func (m *model) updateTable() {
	var rows []table.Row
	for _, project := range m.projects {
		rows = append(rows, table.Row{project.Name, project.Path, project.Command})
	}
	m.table.SetRows(rows)
}

func (m *model) adjustLayout() {
	tableHeight := m.height - 6
	if tableHeight < 5 {
		tableHeight = 5
	}

	// Smart column sizing
	availableWidth := m.width - 6 // Account for borders
	nameWidth := 25
	cmdWidth := 25
	pathWidth := availableWidth - nameWidth - cmdWidth
	if pathWidth < 30 {
		pathWidth = 30
	}

	columns := []table.Column{
		{Title: "Name", Width: nameWidth},
		{Title: "Path", Width: pathWidth},
		{Title: "Command", Width: cmdWidth},
	}

	m.table.SetColumns(columns)
	m.table.SetHeight(tableHeight)
}

func (m *model) startEdit() {
	if len(m.projects) == 0 {
		return
	}

	m.editMode = true
	m.editRow = m.table.Cursor()
	m.editCol = 0 // Start with name column

	// Set the current value in the text input
	project := m.projects[m.editRow]
	switch m.editCol {
	case 0:
		m.textInput.SetValue(project.Name)
	case 1:
		m.textInput.SetValue(project.Path)
	case 2:
		m.textInput.SetValue(project.Command)
	}
	m.textInput.Focus()
}

func (m *model) saveEdit() {
	if !m.editMode || m.editRow < 0 || m.editRow >= len(m.projects) {
		return
	}

	value := m.textInput.Value()
	switch m.editCol {
	case 0:
		m.projects[m.editRow].Name = value
	case 1:
		m.projects[m.editRow].Path = value
	case 2:
		m.projects[m.editRow].Command = value
	}

	m.saveProjects()
	m.updateTable()
}

func (m *model) cancelEdit() {
	m.editMode = false
	m.editRow = -1
	m.editCol = -1
	m.textInput.Blur()
	m.textInput.SetValue("")
}

func (m model) Init() tea.Cmd {
	return tea.SetWindowTitle("Project Launcher")
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case statusMsg:
		m.statusMsg = msg.message
		m.statusExpiry = time.Now().Add(3 * time.Second)
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.adjustLayout()
		return m, nil

	case tea.KeyMsg:
		if m.editMode {
			return m.updateEdit(msg)
		}
		return m.updateNormal(msg)
	}

	// Let table handle mouse events when not editing
	if !m.editMode {
		m.table, cmd = m.table.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) updateEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.cancelEdit()
		return m, nil
	case "enter":
		m.saveEdit()
		m.cancelEdit()
		return m, showStatus("✅ Project updated")
	case "tab":
		// Save current field and move to next
		m.saveEdit()
		m.editCol = (m.editCol + 1) % 3
		project := m.projects[m.editRow]
		switch m.editCol {
		case 0:
			m.textInput.SetValue(project.Name)
		case 1:
			m.textInput.SetValue(project.Path)
		case 2:
			m.textInput.SetValue(project.Command)
		}
		return m, nil
	case "shift+tab":
		// Save current field and move to previous
		m.saveEdit()
		m.editCol = (m.editCol - 1 + 3) % 3
		project := m.projects[m.editRow]
		switch m.editCol {
		case 0:
			m.textInput.SetValue(project.Name)
		case 1:
			m.textInput.SetValue(project.Path)
		case 2:
			m.textInput.SetValue(project.Command)
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "e":
		m.startEdit()
		return m, nil
	case "n", "a":
		// Add new project
		m.projects = append(m.projects, Project{
			Name:    "New Project",
			Path:    "/path/to/project",
			Command: "command",
		})
		m.updateTable()
		m.saveProjects()
		// Start editing the new project
		m.table.SetCursor(len(m.projects) - 1)
		m.startEdit()
		return m, showStatus("➕ New project added")
	case "d", "delete":
		if len(m.projects) > 0 {
			idx := m.table.Cursor()
			projectName := m.projects[idx].Name
			m.projects = append(m.projects[:idx], m.projects[idx+1:]...)
			m.saveProjects()
			m.updateTable()
			return m, showStatus(fmt.Sprintf("🗑️ Deleted %s", projectName))
		}
		return m, nil
	case " ", "enter":
		if len(m.projects) > 0 {
			project := m.projects[m.table.Cursor()]
			return m, m.launchProject(project)
		}
		return m, nil
	case "r":
		m.projects = loadProjects(m.configFile)
		m.updateTable()
		return m, showStatus("🔄 Refreshed")
	case "o":
		return m, m.openLocalhost()
	default:
		// Let table handle arrow keys and other navigation
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) launchProject(project Project) tea.Cmd {
	logFile := fmt.Sprintf("%s.log", strings.ReplaceAll(project.Name, " ", "_"))
	logPath := filepath.Join(project.Path, logFile)

	// Check if this is a Windows path (starts with /mnt/c/)
	isWindowsPath := strings.HasPrefix(project.Path, "/mnt/c/")

	var cmd *exec.Cmd

	if isWindowsPath {
		windowsPath := strings.ReplaceAll(project.Path, "/mnt/c", "C:")
		windowsPath = strings.ReplaceAll(windowsPath, "/", "\\")

		// Use PowerShell for everything, but with different approaches
		if strings.HasSuffix(project.Command, ".exe") {
			// For .exe files, use Start-Process which is PowerShell's way to launch executables
			psCommand := fmt.Sprintf(`Set-Location '%s'; Start-Process '%s'`, windowsPath, project.Command)
			cmd = exec.Command("powershell.exe", "-Command", psCommand)
		} else {
			// For scripts like Python, use direct execution
			psCommand := fmt.Sprintf(`Set-Location '%s'; %s`, windowsPath, project.Command)
			cmd = exec.Command("powershell.exe", "-Command", psCommand)
		}
	} else {
		// For Linux/WSL apps, use bash
		cmdString := fmt.Sprintf(`cd '%s' && echo "Starting %s at $(date)" >> '%s' && %s >> '%s' 2>&1`,
			project.Path, project.Name, logPath, project.Command, logPath)

		cmd = exec.Command("bash", "-c", cmdString)
		cmd.Dir = project.Path

		// THIS IS THE KEY FIX: Set process in its own process group
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true, // Create new process group
			Pgid:    0,    // Use PID as PGID (makes it group leader)
		}
	}

	err := cmd.Start()

	if err != nil {
		return showStatus(fmt.Sprintf("❌ Failed to launch %s: %v", project.Name, err))
	}

	if isWindowsPath {
		method := "PowerShell"
		if strings.HasSuffix(project.Command, ".exe") {
			method = "PowerShell Start-Process"
		}
		return showStatus(fmt.Sprintf("🚀 Launched %s (Windows via %s)", project.Name, method))
	} else {
		return showStatus(fmt.Sprintf("🚀 Launched %s → Log: %s", project.Name, logPath))
	}
}

func (m model) openLocalhost() tea.Cmd {
	url := "http://localhost:3000/notes"

	// WSL2 - use cmd.exe to open default browser on Windows
	cmd := exec.Command("cmd.exe", "/c", "start", url)
	err := cmd.Start()

	if err != nil {
		return showStatus(fmt.Sprintf("❌ Failed to open browser: %v", err))
	}

	return showStatus("🌐 Opened in Windows browser")
}

func (m model) View() string {
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86")).
		Render("🚀 Project Launcher | [ o - to open Project Manager frontend ] ")

	if len(m.projects) == 0 {
		content := "\nNo projects configured yet.\n\nPress 'n' to add your first project!"
		footer := "n: add new project • q: quit"
		return fmt.Sprintf("%s\n%s\n\n%s", header, content, footer)
	}

	var statusMessage string
	if m.statusMsg != "" && time.Now().Before(m.statusExpiry) {
		// Color code based on message type
		color := lipgloss.Color("86") // default green
		if strings.Contains(m.statusMsg, "❌") || strings.Contains(m.statusMsg, "Failed") {
			color = lipgloss.Color("196") // red for errors
		}
		statusStyle := lipgloss.NewStyle().Foreground(color)
		statusMessage = " > " + statusStyle.Render(m.statusMsg)
	}

	// Show different footer based on mode
	var footer string
	if m.editMode {
		colName := []string{"Name", "Path", "Command"}[m.editCol]
		footer = fmt.Sprintf("Editing %s: %s | tab: next field • enter: save • esc: cancel", colName, m.textInput.View())
	} else {
		footer = fmt.Sprintf("↑↓: navigate • space/enter: launch • e: edit • n/a: add • d/delete: delete • r: refresh • q: quit\n%s", statusMessage)
	}

	// If editing, overlay the input on the table
	tableView := m.table.View()
	if m.editMode {
		// This is a simple approach - in a real implementation you'd want to position the input precisely
		tableView = m.table.View()
	}

	return fmt.Sprintf("%s\n\n%s\n\n%s", header, tableView, footer)
}
