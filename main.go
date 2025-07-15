package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Project struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Command  string `json:"command"`
	Link     string `json:"link"`
	Category string `json:"category"`
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
	projects       []Project
	table          table.Model
	editMode       bool
	editRow        int
	editCol        int
	textInput      textinput.Model
	configFile     string
	width          int
	height         int
	statusMsg      string
	statusExpiry   time.Time
	scrollOffset   int   // For horizontal scrolling
	maxCols        int   // Maximum visible columns
	projectIndices []int // Maps display row to actual project index (-1 for headers)
}

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	configFile := filepath.Join(homeDir, ".local/bin/project-launcher.json")

	m := model{
		projects:     loadProjects(configFile),
		configFile:   configFile,
		width:        100,
		height:       24,
		editMode:     false,
		editRow:      -1,
		editCol:      -1,
		scrollOffset: 0,
		maxCols:      5, // Updated to 5 columns (Name, Path, Command, Link, Category)
	}

	// Initialize text input for editing
	m.textInput = textinput.New()
	m.textInput.CharLimit = 200

	// Initialize table like Portmon
	columns := []table.Column{
		{Title: "Name", Width: 30},
		{Title: "Path", Width: 40},
		{Title: "Command", Width: 30},
		{Title: "Category", Width: 15},
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
	sortedProjects := m.getSortedProjects()

	var rows []table.Row
	m.projectIndices = []int{} // Reset project indices mapping

	var lastCategory string
	projectIndex := 0

	for _, project := range sortedProjects {
		// Handle empty category display
		displayCategory := project.Category
		if displayCategory == "" {
			displayCategory = "N/A"
		}

		// Add category header if this is a new category
		if displayCategory != lastCategory {
			// Create category header row
			categoryHeader := fmt.Sprintf("📂 %s", displayCategory)

			// Apply horizontal scrolling to header
			visibleCols := len(m.table.Columns())
			headerRow := make(table.Row, visibleCols)

			startCol := m.scrollOffset

			for i := 0; i < visibleCols; i++ {
				colIndex := startCol + i
				if colIndex == 0 { // Show category in first visible column
					headerRow[i] = categoryHeader
				} else {
					headerRow[i] = ""
				}
			}

			rows = append(rows, headerRow)
			m.projectIndices = append(m.projectIndices, -1) // -1 indicates header row
			lastCategory = displayCategory
		}

		// Create project row
		fullRow := []string{project.Name, project.Path, project.Command, displayCategory, project.Link}

		// Apply horizontal scrolling to show only visible columns
		visibleCols := len(m.table.Columns())
		startCol := m.scrollOffset
		endCol := startCol + visibleCols
		if endCol > len(fullRow) {
			endCol = len(fullRow)
		}

		var visibleRow table.Row
		for i := startCol; i < endCol && i < len(fullRow); i++ {
			visibleRow = append(visibleRow, fullRow[i])
		}

		rows = append(rows, visibleRow)
		m.projectIndices = append(m.projectIndices, projectIndex)
		projectIndex++
	}
	m.table.SetRows(rows)
}

func (m *model) adjustLayout() {
	tableHeight := m.height - 6
	if tableHeight < 5 {
		tableHeight = 5
	}

	// Calculate available width for columns
	availableWidth := m.width - 6 // Account for borders

	// Define all possible columns
	allColumns := []table.Column{
		{Title: "Name", Width: 30},
		{Title: "Path", Width: 35},
		{Title: "Command", Width: 35},
		{Title: "Category", Width: 15},
		{Title: "Link", Width: 30},
	}

	// Calculate how many columns can fit
	totalWidth := 0
	visibleCols := 0
	for i, col := range allColumns {
		if totalWidth+col.Width <= availableWidth {
			totalWidth += col.Width
			visibleCols++
		} else {
			break
		}
		if i >= len(allColumns)-1 {
			break
		}
	}

	// Ensure we show at least one column
	if visibleCols == 0 {
		visibleCols = 1
		// Adjust the width of the first column to fit
		allColumns[0].Width = availableWidth
	}

	// Apply horizontal scrolling offset
	startCol := m.scrollOffset
	endCol := startCol + visibleCols
	if endCol > len(allColumns) {
		endCol = len(allColumns)
		startCol = endCol - visibleCols
		if startCol < 0 {
			startCol = 0
		}
		m.scrollOffset = startCol
	}

	// Select visible columns
	var visibleColumns []table.Column
	for i := startCol; i < endCol && i < len(allColumns); i++ {
		visibleColumns = append(visibleColumns, allColumns[i])
	}

	// If we have extra space, distribute it among visible columns
	if len(visibleColumns) > 0 {
		usedWidth := 0
		for _, col := range visibleColumns {
			usedWidth += col.Width
		}
		if extraWidth := availableWidth - usedWidth; extraWidth > 0 {
			// Distribute extra width to the last column (usually Command or Path)
			visibleColumns[len(visibleColumns)-1].Width += extraWidth
		}
	}

	m.table.SetColumns(visibleColumns)
	m.table.SetHeight(tableHeight)
	m.maxCols = len(allColumns)
}

func (m *model) startEdit() {
	if len(m.projects) == 0 {
		return
	}

	m.editMode = true
	displayIndex := m.table.Cursor()
	m.editRow = m.getOriginalIndexByDisplayIndex(displayIndex)
	if m.editRow == -1 {
		return // Invalid index
	}
	m.editCol = 0 // Start with name column

	// Set the current value in the text input
	project := m.projects[m.editRow]
	var initialValue string
	switch m.editCol {
	case 0:
		initialValue = project.Name
	case 1:
		initialValue = project.Path
	case 2:
		initialValue = project.Command
	case 3:
		initialValue = project.Link
	case 4:
		initialValue = project.Category
	}
	m.textInput.SetValue(initialValue)
	m.textInput.SetCursor(len(initialValue)) // Move cursor to end
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
	case 3:
		m.projects[m.editRow].Link = value
	case 4:
		m.projects[m.editRow].Category = value
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
		m.editCol = (m.editCol + 1) % 5
		project := m.projects[m.editRow]
		var newValue string
		switch m.editCol {
		case 0:
			newValue = project.Name
		case 1:
			newValue = project.Path
		case 2:
			newValue = project.Command
		case 3:
			newValue = project.Link
		case 4:
			newValue = project.Category
		}
		m.textInput.SetValue(newValue)
		m.textInput.SetCursor(len(newValue)) // Move cursor to end
		return m, nil
	case "shift+tab":
		// Save current field and move to previous
		m.saveEdit()
		m.editCol = (m.editCol - 1 + 5) % 5
		project := m.projects[m.editRow]
		var newValue string
		switch m.editCol {
		case 0:
			newValue = project.Name
		case 1:
			newValue = project.Path
		case 2:
			newValue = project.Command
		case 3:
			newValue = project.Link
		case 4:
			newValue = project.Category
		}
		m.textInput.SetValue(newValue)
		m.textInput.SetCursor(len(newValue)) // Move cursor to end
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
			Name:     "New Project",
			Path:     "/path/to/project",
			Command:  "command",
			Link:     "",
			Category: "", // Empty category will display as "N/A"
		})
		m.updateTable()
		m.saveProjects()
		// Start editing the new project
		m.table.SetCursor(len(m.projects) - 1)
		m.startEdit()
		return m, showStatus("➕ New project added")
	case "d", "delete":
		if len(m.projects) > 0 {
			displayIndex := m.table.Cursor()
			originalIndex := m.getOriginalIndexByDisplayIndex(displayIndex)
			if originalIndex == -1 {
				return m, nil
			}
			projectName := m.projects[originalIndex].Name
			m.projects = append(m.projects[:originalIndex], m.projects[originalIndex+1:]...)
			m.saveProjects()
			m.updateTable()
			return m, showStatus(fmt.Sprintf("🗑️ Deleted %s", projectName))
		}
		return m, nil
	case " ", "enter":
		if len(m.projects) > 0 {
			displayIndex := m.table.Cursor()
			project := m.getProjectByDisplayIndex(displayIndex)
			if project != nil {
				return m, m.launchProject(*project)
			}
		}
		return m, nil
	case "r":
		m.projects = loadProjects(m.configFile)
		m.updateTable()
		return m, showStatus("🔄 Refreshed")
	case "o":
		if len(m.projects) > 0 {
			displayIndex := m.table.Cursor()
			project := m.getProjectByDisplayIndex(displayIndex)
			if project != nil {
				return m, m.openProjectLink(*project)
			}
		}
		return m, nil
	case "left":
		// Horizontal scroll left
		if m.scrollOffset > 0 {
			m.scrollOffset--
			m.adjustLayout()
			m.updateTable()
		}
		return m, nil
	case "right":
		// Horizontal scroll right
		maxOffset := m.maxCols - len(m.table.Columns())
		if maxOffset < 0 {
			maxOffset = 0
		}
		if m.scrollOffset < maxOffset {
			m.scrollOffset++
			m.adjustLayout()
			m.updateTable()
		}
		return m, nil
	default:
		// Let table handle arrow keys and other navigation
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) launchProject(project Project) tea.Cmd {
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
		cmdString := fmt.Sprintf(`cd '%s' && %s`, project.Path, project.Command)

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
		return showStatus(fmt.Sprintf("🚀 Launched %s", project.Name))
	}
}

func (m model) openProjectLink(project Project) tea.Cmd {
	if project.Link == "" {
		return showStatus("📭 No Link Associated")
	}

	// WSL2 - use cmd.exe to open default browser on Windows
	cmd := exec.Command("cmd.exe", "/c", "start", project.Link)
	err := cmd.Start()

	if err != nil {
		return showStatus(fmt.Sprintf("❌ Failed to open link: %v", err))
	}

	return showStatus(fmt.Sprintf("🌐 Opened %s link in browser", project.Name))
}

func (m *model) getSortedProjects() []Project {
	// Create a copy of projects for sorting without modifying the original order
	sortedProjects := make([]Project, len(m.projects))
	copy(sortedProjects, m.projects)

	// Sort projects by category first, then by name within each category (case-insensitive)
	sort.Slice(sortedProjects, func(i, j int) bool {
		// Handle empty categories by treating them as "N/A"
		categoryI := sortedProjects[i].Category
		if categoryI == "" {
			categoryI = "N/A"
		}
		categoryJ := sortedProjects[j].Category
		if categoryJ == "" {
			categoryJ = "N/A"
		}

		// First sort by category
		if !strings.EqualFold(categoryI, categoryJ) {
			return strings.ToLower(categoryI) < strings.ToLower(categoryJ)
		}

		// If categories are the same, sort by name
		return strings.ToLower(sortedProjects[i].Name) < strings.ToLower(sortedProjects[j].Name)
	})

	return sortedProjects
}

func (m *model) getProjectByDisplayIndex(displayIndex int) *Project {
	// Check if the display index is valid and not a header row
	if displayIndex < 0 || displayIndex >= len(m.projectIndices) {
		return nil
	}

	// Get the actual project index (-1 means header row)
	projectIndex := m.projectIndices[displayIndex]
	if projectIndex == -1 {
		return nil // This is a header row, no project associated
	}

	sortedProjects := m.getSortedProjects()
	if projectIndex >= len(sortedProjects) {
		return nil
	}

	// Find the original project in m.projects that matches the sorted project
	sortedProject := sortedProjects[projectIndex]
	for i := range m.projects {
		if m.projects[i].Name == sortedProject.Name &&
			m.projects[i].Path == sortedProject.Path &&
			m.projects[i].Command == sortedProject.Command {
			return &m.projects[i]
		}
	}
	return nil
}

func (m *model) getOriginalIndexByDisplayIndex(displayIndex int) int {
	// Check if the display index is valid and not a header row
	if displayIndex < 0 || displayIndex >= len(m.projectIndices) {
		return -1
	}

	// Get the actual project index (-1 means header row)
	projectIndex := m.projectIndices[displayIndex]
	if projectIndex == -1 {
		return -1 // This is a header row, no project associated
	}

	sortedProjects := m.getSortedProjects()
	if projectIndex >= len(sortedProjects) {
		return -1
	}

	// Find the original index in m.projects that matches the sorted project
	sortedProject := sortedProjects[projectIndex]
	for i := range m.projects {
		if m.projects[i].Name == sortedProject.Name &&
			m.projects[i].Path == sortedProject.Path &&
			m.projects[i].Command == sortedProject.Command {
			return i
		}
	}
	return -1
}

func (m model) View() string {
	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86")).
		Render("🚀 Project Launcher")

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
		colName := []string{"Name", "Path", "Command", "Link", "Category"}[m.editCol]
		// Color the keys in edit mode
		keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39")) // Blue color for keys
		footer = fmt.Sprintf("Editing %s: %s | %s: next field • %s: save • %s: cancel",
			colName,
			m.textInput.View(),
			keyStyle.Render("tab"),
			keyStyle.Render("enter"),
			keyStyle.Render("esc"))
	} else {
		// Color styles
		keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))     // Blue color for keys
		actionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("86"))  // Green color for action text
		bulletStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240")) // Gray color for bullets

		scrollHint := ""
		if m.maxCols > len(m.table.Columns()) {
			scrollHint = " " + bulletStyle.Render("•") + " " + keyStyle.Render("←→") + ": " + actionStyle.Render("scroll columns")
		}

		footer = fmt.Sprintf("%s: %s%s %s %s/%s: %s %s %s: %s\n%s/%s: %s %s %s/%s: %s %s %s: %s %s %s: %s %s %s: %s\n%s",
			keyStyle.Render("↑↓"),
			actionStyle.Render("navigate"),
			scrollHint,
			bulletStyle.Render("•"),
			keyStyle.Render("space"),
			keyStyle.Render("enter"),
			actionStyle.Render("launch"),
			bulletStyle.Render("•"),
			keyStyle.Render("e"),
			actionStyle.Render("edit"),
			keyStyle.Render("n"),
			keyStyle.Render("a"),
			actionStyle.Render("add"),
			bulletStyle.Render("•"),
			keyStyle.Render("d"),
			keyStyle.Render("delete"),
			actionStyle.Render("delete"),
			bulletStyle.Render("•"),
			keyStyle.Render("r"),
			actionStyle.Render("refresh"),
			bulletStyle.Render("•"),
			keyStyle.Render("o"),
			actionStyle.Render("open link"),
			bulletStyle.Render("•"),
			keyStyle.Render("q"),
			actionStyle.Render("quit"),
			statusMessage)
	}

	// If editing, overlay the input on the table
	tableView := m.table.View()
	if m.editMode {
		// This is a simple approach - in a real implementation you'd want to position the input precisely
		tableView = m.table.View()
	}

	return fmt.Sprintf("%s\n\n%s\n\n%s", header, tableView, footer)
}
