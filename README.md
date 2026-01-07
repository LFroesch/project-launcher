# Project-Launcher 🚀

A sleek project launcher and manager built with Go that provides an interactive terminal interface for organizing, launching, and managing your development projects.

## Features

- **Interactive TUI** - Beautiful terminal interface built with Bubble Tea
- **Project Management** - Add, edit, and delete projects with ease
- **Smart Launching** - Supports both Linux/WSL and Windows environments
- **Live Editing** - Edit project details directly in the interface
- **Persistent Storage** - Projects saved in JSON configuration file
- **Cross-Platform** - Works seamlessly in WSL2 with Windows integration
- **Quick Access** - Launch your favorite projects with a single keystroke

## Installation

```bash
go install github.com/LFroesch/project-launcher@latest
```

Make sure `$GOPATH/bin` (usually `~/go/bin`) is in your PATH:
```bash
export PATH="$HOME/go/bin:$PATH"
```

### Verify Installation

```bash
project-launcher
```

## Usage

### Basic Commands

```bash
# Launch

project-launcher

# Access from anywhere after installation
project-launcher

```

## Project Configuration

Projects are stored in `~/.local/bin/project-launcher.json` with the following structure:

```json
[
  {
    "name": "My React App",
    "path": "/home/user/projects/my-react-app",
    "command": "npm start"
  },
  {
    "name": "Python API",
    "path": "/home/user/projects/api",
    "command": "python main.py"
  },
  {
    "name": "Windows App",
    "path": "/mnt/c/Users/user/projects/windows-app",
    "command": "myapp.exe"
  }
]
```

### Configuration Fields

- **Name** - Display name for your project
- **Path** - Full path to project directory
- **Command** - Command to execute when launching

## Cross-Platform Support

### Linux/WSL Projects
- Executed in bash with proper process isolation
- Logs created in project directory (`ProjectName.log`)
- Supports all standard Linux commands

### Windows Projects (via WSL2)
- Detects Windows paths (starting with `/mnt/c/`)
- Uses PowerShell for execution
- Supports both `.exe` files and scripts
- Automatic path conversion (WSL → Windows format)

## Examples

### Development Server Setup
```json
{
  "name": "Frontend Dev",
  "path": "/home/user/projects/my-app",
  "command": "npm run dev"
}
```

### Python Application
```json
{
  "name": "Flask API",
  "path": "/home/user/api",
  "command": "python -m flask run --host=0.0.0.0 --port=5000"
}
```

### Windows Application (from WSL)
```json
{
  "name": "Windows Tool",
  "path": "/mnt/c/Users/user/tools",
  "command": "mytool.exe"
}
```

### Docker Project
```json
{
  "name": "Docker Stack",
  "path": "/home/user/docker-project",
  "command": "docker-compose up"
}
```

## Interface Preview

```
🚀 Project Launcher

┌─────────┬──────────────────────────────────────────┬─────────────────────┐
│ Name    │ Path                                     │ Command             │
├─────────┼──────────────────────────────────────────┼─────────────────────┤
│ React   │ /home/user/projects/frontend             │ npm start           │
│ API     │ /home/user/projects/backend              │ python main.py      │
│ Docker  │ /home/user/projects/microservices        │ docker-compose up   │
└─────────┴──────────────────────────────────────────┴─────────────────────┘

↑↓: navigate • space/enter: launch • e: edit • n/a: add • d/delete: delete • r: refresh • q: quit
> 🚀 Launched React → Log: /home/user/projects/frontend/React.log
```

## Smart Process Management

Project Launcher intelligently handles process launching:

- **Process Isolation** - Each project runs in its own process group
- **Background Execution** - Projects continue running after Project Launcher exits
- **Logging** - Automatic log file creation for monitoring
- **Error Handling** - Clear error messages for failed launches

## Configuration Management

### Manual Editing
```bash
# Edit configuration directly
nano ~/.local/bin/project-launcher.json

# Refresh Project Launcher after manual edits
# Press 'r' in the interface
```

## Troubleshooting

**Windows projects not working**
- Ensure WSL2 is properly configured
- Verify PowerShell is available
- Check Windows path accessibility from WSL

**Configuration file issues**
```bash
# Check configuration file location
ls -la ~/.local/bin/project-launcher.json

# Validate JSON syntax
cat ~/.local/bin/project-launcher.json | jq .
```

### Performance Tips

- Keep project paths short and accessible
- Use relative commands when possible
- Regularly clean up old log files
- Limit number of concurrent projects

## Dependencies

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Bubbles](https://github.com/charmbracelet/bubbles) - TUI components  
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) - Terminal styling