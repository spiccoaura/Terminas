package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"



	"github.com/alecthomas/chroma/v2/quick"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
)

type Config struct {
	ApiKey string `json:"apiKey"`
}

func getConfigPath() string {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".terminas")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "config.json")
}

func loadConfig() Config {
	var cfg Config
	path := getConfigPath()
	data, err := os.ReadFile(path)
	if err == nil {
		json.Unmarshal(data, &cfg)
	}
	return cfg
}

func saveConfig(cfg Config) {
	path := getConfigPath()
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(path, data, 0644)
}

// ViewState
type ViewState int

const (
	ViewStart ViewState = iota
	ViewEditor
)

// Modes (for Editor)
type Mode int

const (
	NormalMode Mode = iota
	InsertMode
	CommandMode
)

// SidebarMode
type SidebarMode int

const (
	SidebarExplorer SidebarMode = iota
	SidebarAI
)

// Async Run Message
type RunCompleteMsg struct {
	Output string
	Err    error
	File   string
}

// Palette Dracula-inspired & Antigravity-like
var (
	bgMain       = lipgloss.Color("#282a36")
	bgDarker     = lipgloss.Color("#1e1f29")
	fgPrimary    = lipgloss.Color("#f8f8f2")
	accentColor  = lipgloss.Color("#bd93f9") // Purple
	accentColor2 = lipgloss.Color("#ff79c6") // Pink
	successColor = lipgloss.Color("#50fa7b") // Green
	errorColor   = lipgloss.Color("#ff5555") // Red
	warningColor = lipgloss.Color("#f1fa8c") // Yellow
	commentColor = lipgloss.Color("#6272a4") // Gray
	cyanColor    = lipgloss.Color("#8be9fd") // Cyan

	cliTitleColor = lipgloss.Color("#5e81ac")

	// Styles
	dimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#4c566a"))

	explorerStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(dimStyle.GetForeground()).
			Padding(0, 0)

	editorStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(dimStyle.GetForeground()).
			Padding(0, 0)

	// Status Bar Styles
	statusBarStyle = lipgloss.NewStyle().
			Foreground(fgPrimary).
			Background(bgDarker).
			Padding(0, 1)

	statusBarHighlight = lipgloss.NewStyle().
				Foreground(bgDarker).
				Background(successColor).
				Padding(0, 1).
				Bold(true)

	statusBarCmdMode = lipgloss.NewStyle().
				Foreground(bgDarker).
				Background(warningColor).
				Padding(0, 1).
				Bold(true)

	statusBarInsertMode = lipgloss.NewStyle().
				Foreground(bgDarker).
				Background(accentColor2).
				Padding(0, 1).
				Bold(true)

	tabStyle = lipgloss.NewStyle().
			Foreground(bgMain).
			Background(accentColor2).
			Padding(0, 1).
			MarginLeft(2).
			Bold(true)

	promptBracketStyle = lipgloss.NewStyle().Foreground(accentColor).Bold(true)
	promptUserStyle    = lipgloss.NewStyle().Foreground(accentColor2).Bold(true)
	promptDirStyle     = lipgloss.NewStyle().Foreground(cyanColor).Bold(true)
	promptArrowStyle   = lipgloss.NewStyle().Foreground(cliTitleColor).Bold(true)
)

func renderLogo() string {
	lines := []string{
		` _________   `,
		`|\___   ___\ `,
		`\|___ \  \_| `,
		`     \ \  \  `,
		`      \ \  \ `,
		`       \ \__\`,
		`        \|__|`,
	}

	blockColors := []lipgloss.Color{
		lipgloss.Color("#8be9fd"),
		lipgloss.Color("#8be9fd"),
		lipgloss.Color("#bd93f9"),
		lipgloss.Color("#ff79c6"),
		lipgloss.Color("#ffb86c"),
		lipgloss.Color("#ffb86c"),
		lipgloss.Color("#ff5555"),
	}

	var renderedLines []string
	for i, line := range lines {
		var styledLine string
		for _, char := range line {
			charStr := string(char)
			if charStr == " " {
				styledLine += " "
			} else {
				styledLine += lipgloss.NewStyle().Foreground(blockColors[i]).Bold(true).Render(charStr)
			}
		}
		renderedLines = append(renderedLines, styledLine)
	}
	return strings.Join(renderedLines, "\n")
}

func renderHeaderInfo(username, cwd string) string {
	subtitle := lipgloss.NewStyle().Foreground(cliTitleColor).Bold(true).Render("Environment: " + username)
	engine := dimStyle.Render("Engine: Go TUI (High)")

	displayPath := cwd
	if len(displayPath) > 50 {
		displayPath = "..." + displayPath[len(displayPath)-47:]
	}
	dir := dimStyle.Render("Path: " + displayPath)

	return lipgloss.JoinVertical(lipgloss.Left, subtitle, engine, dir)
}

func getLanguageContext(filename string) (icon string, color lipgloss.Color) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".go":
		return "🐹", lipgloss.Color("#00ADD8")
	case ".py":
		return "🐍", lipgloss.Color("#FFD43B")
	case ".js", ".ts":
		return "🟨", lipgloss.Color("#F7DF1E")
	case ".html", ".css":
		return "🌐", lipgloss.Color("#E34F26")
	case ".md":
		return "📝", lipgloss.Color("#083fa1")
	case ".json":
		return "⚙️", lipgloss.Color("#F7DF1E")
	case ".sh", ".ps1":
		return "🐚", lipgloss.Color("#50fa7b")
	default:
		return "📄", accentColor2
	}
}

func highlightCode(code, filename string, maxWidth int) string {
	if code == "" {
		return lipgloss.NewStyle().Foreground(commentColor).Render(" 1 ")
	}

	var buf bytes.Buffer
	err := quick.Highlight(&buf, code, filename, "terminal256", "dracula")
	if err != nil {
		buf.WriteString(code)
	}

	coloredStr := buf.String()
	lines := strings.Split(coloredStr, "\n")

	var withNumbers strings.Builder
	for i, l := range lines {
		if i == len(lines)-1 && l == "" {
			break
		}
		num := lipgloss.NewStyle().Foreground(commentColor).Render(fmt.Sprintf("%2d ", i+1))
		
		maxCodeWidth := maxWidth - 4
		if maxCodeWidth < 1 {
			maxCodeWidth = 1
		}
		
		truncatedLine := lipgloss.NewStyle().MaxWidth(maxCodeWidth).Render(l)
		withNumbers.WriteString(num + truncatedLine + "\n")
	}

	return withNumbers.String()
}

func getAllWorkspaceContext(currentFile string, editorContent string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Sprintf("Current File: %s\nContent:\n%s", currentFile, editorContent)
	}

	var sb strings.Builder
	sb.WriteString("=== WORKSPACE FILES ===\n\n")

	filepath.WalkDir(cwd, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" || name == "bin" || name == "dist" {
				return filepath.SkipDir
			}
			return nil
		}
		
		name := d.Name()
		if strings.HasPrefix(name, ".") {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(name))
		
		// Skip binaries, images, etc.
		if ext == ".exe" || ext == ".dll" || ext == ".so" || ext == ".png" || ext == ".jpg" || ext == ".pdf" {
			return nil
		}
		
		relPath, _ := filepath.Rel(cwd, path)
		
		sb.WriteString(fmt.Sprintf("--- File: %s ---\n", relPath))
		
		if relPath == currentFile || filepath.Base(currentFile) == name {
			// Use the actively edited content for the current file!
			sb.WriteString(editorContent + "\n\n")
		} else {
			data, err := os.ReadFile(path)
			if err == nil {
				// Prevent dumping massive files
				if len(data) > 50000 {
					sb.WriteString("[File too large to display entirely]\n\n")
				} else {
					sb.WriteString(string(data) + "\n\n")
				}
			}
		}

		return nil
	})
	
	return sb.String()
}

type AiResponseMsg struct {
	Response string
	Err      error
}

func askAiCmd(prompt, apiKey, currentFile, fileContent string, useWorkspaceContext bool) tea.Cmd {
	return func() tea.Msg {
		if apiKey == "" {
			return AiResponseMsg{Err: fmt.Errorf("API key not set. Use login <key>")}
		}
		
		var workspaceContext string
		if useWorkspaceContext {
			workspaceContext = getAllWorkspaceContext(currentFile, fileContent)
		} else {
			workspaceContext = fmt.Sprintf("Current File: %s\nContent:\n%s", currentFile, fileContent)
		}
		
		fullPrompt := fmt.Sprintf("You are Terminas AI IDE Assistant.\nThe user is currently editing: %s\n\nHere is the full workspace context:\n%s\n\nIf you want to modify the CURRENT file, wrap the new code in:\n<WRITE>\n[the full new file code here]\n</WRITE>\n\nIf you want to create or edit ANY OTHER file or folder, use this format:\n<CREATE file=\"path/to/newfile.py\">\n[full file code here]\n</CREATE>\nThe IDE will automatically create the directories and save the file.\n\nALWAYS provide a short changelog (summary of changes) as normal chat text BEFORE using the <WRITE> or <CREATE> tags.\n\nDo not use markdown blocks for the code inside these tags.\n\nUser request: %s", currentFile, workspaceContext, prompt)
		
		var url string
		var reqBody []byte
		var headers map[string]string
		
		escapedBytes, _ := json.Marshal(fullPrompt)
		escapedPrompt := string(escapedBytes[1 : len(escapedBytes)-1])
		
		if strings.HasPrefix(apiKey, "AIza") {
			// Gemini
			url = "https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash:generateContent?key=" + apiKey
			reqBody = []byte(fmt.Sprintf(`{"contents":[{"parts":[{"text":"%s"}]}]}`, escapedPrompt))
			headers = map[string]string{"Content-Type": "application/json"}
		} else if strings.HasPrefix(apiKey, "sk-ant") {
			// Anthropic Claude
			url = "https://api.anthropic.com/v1/messages"
			reqBody = []byte(fmt.Sprintf(`{"model":"claude-3-haiku-20240307","max_tokens":1024,"messages":[{"role":"user","content":"%s"}]}`, escapedPrompt))
			headers = map[string]string{
				"Content-Type": "application/json",
				"x-api-key": apiKey,
				"anthropic-version": "2023-06-01",
			}
		} else if strings.HasPrefix(apiKey, "sk-proj-") {
			// OpenAI
			url = "https://api.openai.com/v1/chat/completions"
			reqBody = []byte(fmt.Sprintf(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"%s"}]}`, escapedPrompt))
			headers = map[string]string{
				"Content-Type": "application/json",
				"Authorization": "Bearer " + apiKey,
			}
		} else {
			// DeepSeek (default for sk- without proj)
			url = "https://api.deepseek.com/chat/completions"
			reqBody = []byte(fmt.Sprintf(`{"model":"deepseek-chat","messages":[{"role":"user","content":"%s"}]}`, escapedPrompt))
			headers = map[string]string{
				"Content-Type": "application/json",
				"Authorization": "Bearer " + apiKey,
			}
		}
		
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
		if err != nil {
			return AiResponseMsg{Err: err}
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return AiResponseMsg{Err: err}
		}
		defer resp.Body.Close()
		
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return AiResponseMsg{Err: err}
		}
		
		if resp.StatusCode != http.StatusOK {
			return AiResponseMsg{Err: fmt.Errorf("API Error (%d): %s", resp.StatusCode, string(body))}
		}
		
		var responseText string
		if strings.HasPrefix(apiKey, "AIza") {
			var result struct {
				Candidates []struct {
					Content struct {
						Parts []struct {
							Text string `json:"text"`
						} `json:"parts"`
					} `json:"content"`
				} `json:"candidates"`
			}
			if err := json.Unmarshal(body, &result); err == nil && len(result.Candidates) > 0 && len(result.Candidates[0].Content.Parts) > 0 {
				responseText = result.Candidates[0].Content.Parts[0].Text
			}
		} else if strings.HasPrefix(apiKey, "sk-ant") {
			var result struct {
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			}
			if err := json.Unmarshal(body, &result); err == nil && len(result.Content) > 0 {
				responseText = result.Content[0].Text
			}
		} else {
			var result struct {
				Choices []struct {
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
				} `json:"choices"`
			}
			if err := json.Unmarshal(body, &result); err == nil && len(result.Choices) > 0 {
				responseText = result.Choices[0].Message.Content
			}
		}
		
		if responseText == "" {
			return AiResponseMsg{Err: fmt.Errorf("Empty or unparsable response from AI")}
		}
		
		return AiResponseMsg{Response: responseText}
	}
}

type model struct {
	ready  bool
	width  int
	height int

	viewState ViewState

	consoleInput  textinput.Model
	consoleOutput []string
	username      string
	ghostText     string
	apiKey        string

	mode         Mode
	currentFile  string
	message      string
	ctrlCPressed bool
	isModified   bool
	isSaved      bool

	explorerViewport viewport.Model
	codeViewport     viewport.Model
	aiViewport       viewport.Model
	editor           textarea.Model
	commandInput     textinput.Model

	sidebarMode SidebarMode
	aiHistory   []string
	aiSpinner   spinner.Model
	isAiThinking bool
	gradientOffset int
	useWorkspaceContext bool
}

func initialModel() model {
	cfg := loadConfig()

	ti := textinput.New()
	ti.Prompt = lipgloss.NewStyle().Foreground(warningColor).Render(" / ")
	ti.Placeholder = "login <k> | ai <p> | open <f> | save <f> | run | cd <d> | help"
	ti.CharLimit = 156
	ti.Width = 80

	ci := textinput.New()
	ci.Prompt = promptArrowStyle.Render("╰─> ")
	ci.Placeholder = ""
	ci.CharLimit = 256
	ci.Width = 80
	ci.Focus()

	ta := textarea.New()
	ta.Placeholder = "Write your code here..."
	ta.Prompt = " "
	ta.ShowLineNumbers = true
	ta.SetHeight(10)
	ta.SetWidth(30)
	ta.Blur()

	cv := viewport.New(30, 10)
	cv.SetContent(highlightCode("", "Untitled", 30))

	username := os.Getenv("USERNAME")
	if username == "" {
		username = "user"
	}

	av := viewport.New(20, 10)
	av.SetContent(lipgloss.NewStyle().Foreground(dimStyle.GetForeground()).Render("Type /ai <prompt> to chat!"))

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(accentColor2)

	return model{
		viewState:     ViewStart,
		mode:          NormalMode,
		currentFile:   "Untitled",
		editor:        ta,
		codeViewport:  cv,
		commandInput:  ti,
		consoleInput:  ci,
		username:      username,
		consoleOutput: []string{},
		apiKey:        cfg.ApiKey,
		message:       "Ready.",
		ctrlCPressed:  false,
		isModified:    false,
		isSaved:       false,
		useWorkspaceContext: false,
		explorerViewport: cv,
		aiViewport:    av,
		sidebarMode:   SidebarExplorer,
		aiHistory:     []string{},
		aiSpinner:     s,
		isAiThinking:  false,
		gradientOffset: 0,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.aiSpinner.Tick)
}

func getDirContent(currentFile string, isModified, isSaved bool, maxWidth int) string {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	baseDir := filepath.Base(cwd)

	files, err := os.ReadDir(".")
	if err != nil {
		return " Read Error"
	}

	var sb strings.Builder
	titleStr := "📁 " + baseDir
	title := lipgloss.NewStyle().Foreground(accentColor).Bold(true).Padding(1, 0, 1, 1).MaxWidth(maxWidth - 2).Render(titleStr)
	sb.WriteString(title + "\n")

	folderStyle := lipgloss.NewStyle().Foreground(successColor).PaddingLeft(2).MaxWidth(maxWidth - 2)
	fileStyle := lipgloss.NewStyle().Foreground(fgPrimary).PaddingLeft(2).MaxWidth(maxWidth - 2)

	for _, f := range files {
		if f.IsDir() {
			sb.WriteString(folderStyle.Render("📁 "+f.Name()) + "\n")
		} else {
			icon, _ := getLanguageContext(f.Name())
			fileText := icon + " " + f.Name()

			style := fileStyle.Copy()
			if filepath.Base(currentFile) == f.Name() {
				if isModified {
					style = style.Foreground(errorColor).Bold(true)
					fileText += " *"
				} else if isSaved {
					style = style.Foreground(successColor).Bold(true)
				}
			}

			sb.WriteString(style.Render(fileText) + "\n")
		}
	}
	return sb.String()
}

func runScriptCmd(filename string) tea.Cmd {
	return func() tea.Msg {
		ext := strings.ToLower(filepath.Ext(filename))
		var cmd *exec.Cmd
		switch ext {
		case ".go":
			cmd = exec.Command("go", "run", filename)
		case ".py":
			cmd = exec.Command("python", filename)
		case ".js":
			cmd = exec.Command("node", filename)
		default:
			return RunCompleteMsg{Output: "", Err: fmt.Errorf("Run not supported for: %s", ext), File: filename}
		}

		out, err := cmd.CombinedOutput()
		return RunCompleteMsg{Output: string(out), Err: err, File: filename}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmds []tea.Cmd
		cmd  tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			if m.ctrlCPressed {
				return m, tea.Quit
			}
			m.ctrlCPressed = true
			m.message = lipgloss.NewStyle().Foreground(warningColor).Render("Press Ctrl+C again to quit.")
			return m, nil
		}

		if m.ctrlCPressed {
			m.ctrlCPressed = false
			m.message = "Mode: " + getModeString(m.mode)
		}

		if msg.String() == "ctrl+t" {
			if m.viewState == ViewEditor {
				m.viewState = ViewStart
				m.resize(m.width, m.height)
				return m, nil
			} else {
				m.viewState = ViewEditor
				m.resize(m.width, m.height)
				return m, nil
			}
		}

		if msg.String() == "ctrl+o" {
			if m.sidebarMode == SidebarExplorer {
				m.sidebarMode = SidebarAI
				m.message = "Sidebar: AI Chat"
			} else {
				m.sidebarMode = SidebarExplorer
				m.message = "Sidebar: File Explorer"
			}
			m.resize(m.width, m.height)
			return m, nil
		}

		if msg.String() == "ctrl+w" {
			m.useWorkspaceContext = !m.useWorkspaceContext
			if m.useWorkspaceContext {
				m.message = "Workspace Context: ON"
			} else {
				m.message = "Workspace Context: OFF"
			}
			return m, nil
		}

		if m.viewState == ViewStart {
			if msg.String() == "enter" {
				cmdStr := strings.TrimSpace(m.consoleInput.Value())
				if cmdStr != "" {
					cmd = m.executeConsoleCommand(cmdStr)
					if cmd != nil {
						cmds = append(cmds, cmd)
					}
				}
				m.consoleInput.SetValue("")
				m.ghostText = ""
				return m, tea.Batch(cmds...)
			} else if msg.String() == "tab" && m.ghostText != "" {
				m.consoleInput.SetValue(m.consoleInput.Value() + m.ghostText)
				m.consoleInput.SetCursor(len(m.consoleInput.Value()))
				m.ghostText = ""
				return m, nil
			}

			m.consoleInput, cmd = m.consoleInput.Update(msg)
			cmds = append(cmds, cmd)
			m.updateGhostText()
			return m, tea.Batch(cmds...)
		}

		// --- ViewEditor ---
		if m.mode == NormalMode && msg.String() == "q" {
			return m, tea.Quit
		}

		if msg.String() == "esc" {
			if m.mode == InsertMode {
				m.codeViewport.SetContent(highlightCode(m.editor.Value(), m.currentFile, m.codeViewport.Width))
			}
			m.mode = NormalMode
			m.editor.Blur()
			m.commandInput.Blur()
			m.message = "Mode: NORMAL"
			m.resize(m.width, m.height)
			return m, nil
		}

		switch m.mode {
		case NormalMode:
			if msg.String() == "i" {
				m.mode = InsertMode
				m.editor.Focus()
				m.message = "Mode: INSERT"
				m.resize(m.width, m.height)
				return m, textarea.Blink
			} else if msg.String() == "/" {
				m.mode = CommandMode
				m.commandInput.Focus()
				m.commandInput.SetValue("")
				m.message = "Mode: COMMAND"
				m.resize(m.width, m.height)
				return m, textinput.Blink
			} else {
				m.codeViewport, cmd = m.codeViewport.Update(msg)
				cmds = append(cmds, cmd)
			}

		case CommandMode:
			if msg.String() == "enter" {
				cmdStr := strings.TrimSpace(m.commandInput.Value())

				if runCmd := m.executeCommand(cmdStr); runCmd != nil {
					cmds = append(cmds, runCmd)
				}

				if m.viewState == ViewEditor {
					m.mode = NormalMode
					m.commandInput.Blur()
					m.commandInput.SetValue("")
					m.resize(m.width, m.height)
				}
				return m, tea.Batch(cmds...)
			} else {
				m.commandInput, cmd = m.commandInput.Update(msg)
				cmds = append(cmds, cmd)
			}

		case InsertMode:
			oldVal := m.editor.Value()
			m.editor, cmd = m.editor.Update(msg)

			if m.editor.Value() != oldVal {
				if !m.isModified {
					m.isModified = true
					m.isSaved = false
					m.explorerViewport.SetContent(getDirContent(m.currentFile, m.isModified, m.isSaved, m.explorerViewport.Width))
				}
			}
			cmds = append(cmds, cmd)
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.aiSpinner, cmd = m.aiSpinner.Update(msg)
		m.gradientOffset++
		return m, cmd

	case RunCompleteMsg:
		m.message = "Execution Finished."

		outputBox := strings.TrimSpace(msg.Output)
		if outputBox == "" {
			outputBox = "(No output)"
		}

		m.consoleOutput = append(m.consoleOutput, promptArrowStyle.Render("╰─> ")+lipgloss.NewStyle().Foreground(warningColor).Render("Async Executed: "+msg.File))
		if msg.Err != nil {
			m.consoleOutput = append(m.consoleOutput, lipgloss.NewStyle().Foreground(errorColor).Bold(true).Render("✖ ERROR: ")+msg.Err.Error())
		}
		m.consoleOutput = append(m.consoleOutput, outputBox)
		return m, nil
	case AiResponseMsg:
		m.isAiThinking = false
		if msg.Err != nil {
			errStr := msg.Err.Error()
			errStr = strings.ReplaceAll(errStr, "\n", " ")
			errStr = strings.ReplaceAll(errStr, "\r", "")
			if len(errStr) > 100 {
				errStr = errStr[:97] + "..."
			}
			m.message = lipgloss.NewStyle().Foreground(errorColor).Render("AI Error: " + errStr)
			m.consoleOutput = append(m.consoleOutput, lipgloss.NewStyle().Foreground(errorColor).Bold(true).Render("✖ AI ERROR: \n")+msg.Err.Error())
			
			m.aiHistory = append(m.aiHistory, lipgloss.NewStyle().Foreground(errorColor).Bold(true).Render("✖ AI Error: ")+errStr)
		} else {
			m.message = "AI finished."
			respTextRaw := msg.Response
			
			// Handle <WRITE> tag for auto-modification
			if startIdx := strings.Index(respTextRaw, "<WRITE>"); startIdx != -1 {
				// The tag might be <WRITE>\n so we should ideally find the end tag
				if endIdx := strings.Index(respTextRaw[startIdx:], "</WRITE>"); endIdx != -1 {
					absoluteEnd := startIdx + endIdx
					codeContent := respTextRaw[startIdx+7 : absoluteEnd]
					codeContent = strings.TrimPrefix(codeContent, "\n")
					codeContent = strings.TrimSuffix(codeContent, "\n")
					
					m.editor.SetValue(codeContent)
					m.isModified = true
					m.codeViewport.SetContent(highlightCode(m.editor.Value(), m.currentFile, m.codeViewport.Width))
					
					// Replace the big block of code in the chat log with a success message
					respTextRaw = respTextRaw[:startIdx] + "\n[✓ AI applied modifications to the file!]\n" + respTextRaw[absoluteEnd+8:]
				}
			}

			// Handle <CREATE file="..."> tag for creating files and folders
			if startIdx := strings.Index(respTextRaw, "<CREATE file=\""); startIdx != -1 {
				fileStart := startIdx + 14
				if fileEnd := strings.Index(respTextRaw[fileStart:], "\">"); fileEnd != -1 {
					absoluteFileEnd := fileStart + fileEnd
					filePath := respTextRaw[fileStart:absoluteFileEnd]
					
					if endIdx := strings.Index(respTextRaw[absoluteFileEnd:], "</CREATE>"); endIdx != -1 {
						absoluteEnd := absoluteFileEnd + endIdx
						
						codeContent := respTextRaw[absoluteFileEnd+2 : absoluteEnd]
						codeContent = strings.TrimPrefix(codeContent, "\n")
						codeContent = strings.TrimSuffix(codeContent, "\n")
						
						// Create directory and file
						os.MkdirAll(filepath.Dir(filePath), 0755)
						os.WriteFile(filePath, []byte(codeContent), 0644)
						
						// Auto open the created file
						m.currentFile = filePath
						m.editor.SetValue(codeContent)
						m.isModified = false
						m.isSaved = true
						m.codeViewport.SetContent(highlightCode(m.editor.Value(), m.currentFile, m.codeViewport.Width))
						m.viewState = ViewEditor
						m.mode = NormalMode
						
						// Refresh explorer
						m.explorerViewport.SetContent(getDirContent(m.currentFile, m.isModified, m.isSaved, m.explorerViewport.Width))
						
						respTextRaw = respTextRaw[:startIdx] + fmt.Sprintf("\n[✓ AI created/modified file: %s]\n", filePath) + respTextRaw[absoluteEnd+9:]
					}
				}
			}

			aiBox := lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(accentColor).
				Padding(1, 2).
				Render(lipgloss.NewStyle().Foreground(successColor).Bold(true).Render("🤖 AI Core Response:\n") + respTextRaw)
			
			m.consoleOutput = append(m.consoleOutput, aiBox)
			
			respText := lipgloss.NewStyle().Foreground(successColor).Bold(true).Render("🤖 AI:") + "\n" + respTextRaw
			m.aiHistory = append(m.aiHistory, respText)
		}
		
		m.refreshAiViewport()
		return m, nil
	}

	return m, tea.Batch(cmds...)
}

func getModeString(m Mode) string {
	switch m {
	case NormalMode:
		return "NORMAL"
	case InsertMode:
		return "INSERT"
	case CommandMode:
		return "COMMAND"
	}
	return ""
}

func (m *model) updateGhostText() {
	val := m.consoleInput.Value()
	m.ghostText = ""
	if val == "" {
		return
	}

	commands := []string{"ide", "cd ", "clear", "exit", "ls", "help", "mkdir ", "rm ", "touch ", "cat ", "about", "login ", "ai ", "chat"}
	for _, c := range commands {
		if strings.HasPrefix(c, val) {
			m.ghostText = c[len(val):]
			break
		}
	}
}

func (m *model) refreshAiViewport() {
	if m.aiViewport.Width <= 4 {
		return
	}
	content := strings.Join(m.aiHistory, "\n\n")
	wrapped := wordwrap.String(content, m.aiViewport.Width-4)
	m.aiViewport.SetContent(wrapped)
	m.aiViewport.GotoBottom()
}

func (m *model) resize(w, h int) {
	verticalMargins := 2
	statusBarHeight := 1

	commandBarHeight := 0
	if m.mode == CommandMode {
		commandBarHeight = 1
	}

	tabHeight := 1

	contentHeight := h - verticalMargins - statusBarHeight - commandBarHeight - tabHeight
	if contentHeight < 1 {
		contentHeight = 1
	}

	horizontalMarginsSidebar := 2
	horizontalMarginsEditor := 2

	sidebarWidth := (w * 30 / 100) - horizontalMarginsSidebar
	if sidebarWidth < 25 {
		sidebarWidth = 25
	}
	if sidebarWidth > 45 {
		sidebarWidth = 45
	}

	editorWidth := w - sidebarWidth - horizontalMarginsSidebar - horizontalMarginsEditor - 1
	if editorWidth < 10 {
		editorWidth = 10
	}

	if !m.ready {
		m.explorerViewport = viewport.New(sidebarWidth, contentHeight+tabHeight)
		m.codeViewport = viewport.New(editorWidth, contentHeight)
		m.aiViewport = viewport.New(sidebarWidth, contentHeight+tabHeight)
		m.ready = true
	} else {
		m.explorerViewport.Width = sidebarWidth
		m.explorerViewport.Height = contentHeight + tabHeight
		m.aiViewport.Width = sidebarWidth
		m.aiViewport.Height = contentHeight + tabHeight
		
		m.codeViewport.Width = editorWidth
		m.codeViewport.Height = contentHeight
	}

	m.editor.SetWidth(editorWidth)
	m.editor.SetHeight(contentHeight)

	m.explorerViewport.SetContent(getDirContent(m.currentFile, m.isModified, m.isSaved, m.explorerViewport.Width))
	m.refreshAiViewport()
}

func (m *model) executeConsoleCommand(cmdStr string) tea.Cmd {
	historyPrompt := promptArrowStyle.Render("╰─> ")
	m.consoleOutput = append(m.consoleOutput, historyPrompt+cmdStr)

	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return nil
	}

	action := parts[0]
	args := parts[1:]

	errorMsg := func(msg string) string {
		return lipgloss.NewStyle().Foreground(errorColor).Bold(true).Render("✖ Error:") + " " + msg
	}

	switch action {
	case "ide", "start":
		m.viewState = ViewEditor
		m.explorerViewport.SetContent(getDirContent(m.currentFile, m.isModified, m.isSaved, m.explorerViewport.Width))
		m.codeViewport.SetContent(highlightCode(m.editor.Value(), m.currentFile, m.codeViewport.Width))

	case "login":
		if len(args) == 0 {
			m.consoleOutput = append(m.consoleOutput, errorMsg("specify an API key (login <key>)"))
			return nil
		}
		m.apiKey = args[0]
		saveConfig(Config{ApiKey: m.apiKey})
		m.consoleOutput = append(m.consoleOutput, lipgloss.NewStyle().Foreground(successColor).Render("✔ API Key saved!"))

	case "ai":
		if len(args) == 0 {
			m.consoleOutput = append(m.consoleOutput, errorMsg("specify a prompt (ai <prompt>)"))
			return nil
		}
		prompt := strings.Join(args, " ")
		m.consoleOutput = append(m.consoleOutput, lipgloss.NewStyle().Foreground(accentColor).Render("🤖 AI is thinking..."))
		return askAiCmd(prompt, m.apiKey, m.currentFile, m.editor.Value(), m.useWorkspaceContext)

	case "cd":
		if len(args) == 0 {
			m.consoleOutput = append(m.consoleOutput, errorMsg("specify a directory."))
			return nil
		}
		err := os.Chdir(args[0])
		if err != nil {
			m.consoleOutput = append(m.consoleOutput, errorMsg(err.Error()))
		} else {
			m.explorerViewport.SetContent(getDirContent(m.currentFile, m.isModified, m.isSaved, m.explorerViewport.Width))
		}

	case "ls":
		files, err := os.ReadDir(".")
		if err != nil {
			m.consoleOutput = append(m.consoleOutput, errorMsg(err.Error()))
		} else {
			var out string
			for _, f := range files {
				if f.IsDir() {
					out += lipgloss.NewStyle().Foreground(successColor).Render("📁 "+f.Name()) + "  "
				} else {
					icon, _ := getLanguageContext(f.Name())

					// Apply color logic to ls command output too if it matches current file
					fileText := icon + " " + f.Name()
					style := lipgloss.NewStyle().Foreground(fgPrimary)

					if filepath.Base(m.currentFile) == f.Name() {
						if m.isModified {
							style = lipgloss.NewStyle().Foreground(errorColor).Bold(true)
							fileText += "*"
						} else if m.isSaved {
							style = lipgloss.NewStyle().Foreground(successColor).Bold(true)
						}
					}
					out += style.Render(fileText) + "  "
				}
			}
			m.consoleOutput = append(m.consoleOutput, out)
		}

	case "mkdir":
		if len(args) == 0 {
			m.consoleOutput = append(m.consoleOutput, errorMsg("specify folder name."))
			return nil
		}
		err := os.Mkdir(args[0], 0755)
		if err != nil {
			m.consoleOutput = append(m.consoleOutput, errorMsg(err.Error()))
		} else {
			m.consoleOutput = append(m.consoleOutput, lipgloss.NewStyle().Foreground(successColor).Render("✔ Folder created: ")+args[0])
			m.explorerViewport.SetContent(getDirContent(m.currentFile, m.isModified, m.isSaved, m.explorerViewport.Width))
		}

	case "touch":
		if len(args) == 0 {
			m.consoleOutput = append(m.consoleOutput, errorMsg("specify file name."))
			return nil
		}
		err := os.WriteFile(args[0], []byte(""), 0644)
		if err != nil {
			m.consoleOutput = append(m.consoleOutput, errorMsg(err.Error()))
		} else {
			m.consoleOutput = append(m.consoleOutput, lipgloss.NewStyle().Foreground(successColor).Render("✔ File created: ")+args[0])
			m.explorerViewport.SetContent(getDirContent(m.currentFile, m.isModified, m.isSaved, m.explorerViewport.Width))
		}

	case "cat":
		if len(args) == 0 {
			m.consoleOutput = append(m.consoleOutput, errorMsg("specify a file to read."))
			return nil
		}
		data, err := os.ReadFile(args[0])
		if err != nil {
			m.consoleOutput = append(m.consoleOutput, errorMsg(err.Error()))
		} else {
			highlighted := highlightCode(string(data), args[0], m.width-4)
			box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(dimStyle.GetForeground()).Padding(0, 1).Render(highlighted)
			m.consoleOutput = append(m.consoleOutput, box)
		}

	case "rm":
		if len(args) == 0 {
			m.consoleOutput = append(m.consoleOutput, errorMsg("specify what to remove."))
			return nil
		}
		err := os.RemoveAll(args[0])
		if err != nil {
			m.consoleOutput = append(m.consoleOutput, errorMsg(err.Error()))
		} else {
			m.consoleOutput = append(m.consoleOutput, lipgloss.NewStyle().Foreground(successColor).Render("✔ Removed: ")+args[0])
			m.explorerViewport.SetContent(getDirContent(m.currentFile, m.isModified, m.isSaved, m.explorerViewport.Width))
		}

	case "help":
		var sb strings.Builder
		sb.WriteString(lipgloss.NewStyle().Foreground(accentColor).Bold(true).Render("✨ Terminas CLI - Available Commands ✨\n"))

		cmds := []struct{ name, desc string }{
			{"ide / start", "Launch the visual IDE environment"},
			{"cat <file>", "Print file content with syntax highlighting"},
			{"touch <file>", "Create a new empty file"},
			{"ls", "List files and directories with icons"},
			{"cd <dir>", "Change working directory"},
			{"mkdir <dir>", "Create a new directory"},
			{"rm <file>", "Delete a file or directory"},
			{"about", "Discover the mastermind behind Terminas"},
			{"clear", "Clear the console screen"},
			{"exit / q", "Quit Terminas"},
		}

		for _, c := range cmds {
			name := lipgloss.NewStyle().Foreground(successColor).Width(15).Render(c.name)
			desc := lipgloss.NewStyle().Foreground(fgPrimary).Render(c.desc)
			sb.WriteString("  " + name + " " + desc + "\n")
		}
		m.consoleOutput = append(m.consoleOutput, sb.String())

	case "about":
		boxStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accentColor).
			Padding(1, 2)

		content := lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Foreground(accentColor2).Bold(true).Render("🚀 TERMINAS IDE - Engineered for Speed and Aesthetics"),
			"",
			dimStyle.Render("Created by ")+lipgloss.NewStyle().Foreground(successColor).Bold(true).Render("Spicco D'Aura"),
			"",
			lipgloss.NewStyle().Foreground(successColor).Render("1. Zero Learning Curve")+" - Everything is intuitive. No obscure manuals.",
			lipgloss.NewStyle().Foreground(successColor).Render("2. Breathtaking Design")+" - Reactive borders, dynamic icons, Dracula palette.",
			lipgloss.NewStyle().Foreground(successColor).Render("3. Integrated OS")+" - Editor and Terminal fused. Instant switch with Ctrl+T.",
			lipgloss.NewStyle().Foreground(successColor).Render("4. Async Live Execution")+" - /run executes Python/Go/JS effortlessly in background.",
			lipgloss.NewStyle().Foreground(successColor).Render("5. 100% Portable")+" - A single .exe file. No dependencies. Zero configuration.",
		)
		m.consoleOutput = append(m.consoleOutput, boxStyle.Render(content))

	case "clear":
		m.consoleOutput = []string{}

	case "exit", "quit", "q":
		os.Exit(0)

	default:
		m.consoleOutput = append(m.consoleOutput, errorMsg("Command not recognized: "+action))
	}

	return nil
}

func (m *model) executeCommand(cmdStr string) tea.Cmd {
	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return nil
	}

	action := parts[0]
	args := parts[1:]

	switch action {
	case "login":
		if len(args) == 0 {
			m.message = lipgloss.NewStyle().Foreground(errorColor).Render("Error: specify an API key")
			return nil
		}
		m.apiKey = args[0]
		saveConfig(Config{ApiKey: m.apiKey})
		m.message = lipgloss.NewStyle().Foreground(successColor).Render("✔ API Key saved!")
		return nil

	case "chat":
		if m.sidebarMode == SidebarExplorer {
			m.sidebarMode = SidebarAI
			m.message = "Sidebar: AI Chat"
		} else {
			m.sidebarMode = SidebarExplorer
			m.message = "Sidebar: File Explorer"
		}
		m.resize(m.width, m.height)
		return nil

	case "ai":
		if len(args) == 0 {
			m.message = lipgloss.NewStyle().Foreground(errorColor).Render("Error: specify a prompt (ai <prompt>)")
			return nil
		}
		prompt := strings.Join(args, " ")
		m.message = "Thinking..."
		
		m.isAiThinking = true
		if m.sidebarMode != SidebarAI {
			m.sidebarMode = SidebarAI
			m.resize(m.width, m.height)
		}
		
		userPrompt := lipgloss.NewStyle().Foreground(accentColor2).Bold(true).Render("You:") + " " + prompt
		m.aiHistory = append(m.aiHistory, userPrompt)
		m.refreshAiViewport()
		
		m.consoleOutput = append(m.consoleOutput, lipgloss.NewStyle().Foreground(accentColor).Render("🤖 AI is thinking..."))
		
		return tea.Batch(m.aiSpinner.Tick, askAiCmd(prompt, m.apiKey, m.currentFile, m.editor.Value(), m.useWorkspaceContext))

	case "open":
		if len(args) == 0 {
			m.message = lipgloss.NewStyle().Foreground(errorColor).Render("Error: specify a file.")
			return nil
		}
		filename := args[0]
		data, err := os.ReadFile(filename)
		if err != nil {
			m.message = lipgloss.NewStyle().Foreground(errorColor).Render("Error: " + err.Error())
		} else {
			if bytes.Contains(data, []byte{0}) {
				m.message = lipgloss.NewStyle().Foreground(errorColor).Render("Error: Cannot open binary file.")
				return nil
			}
			m.editor.SetValue(string(data))
			m.currentFile = filename
			m.isModified = false
			m.isSaved = false
			m.message = "File opened: " + filename
			m.explorerViewport.SetContent(getDirContent(m.currentFile, m.isModified, m.isSaved, m.explorerViewport.Width))
			m.codeViewport.SetContent(highlightCode(m.editor.Value(), m.currentFile, m.codeViewport.Width))
		}

	case "save":
		filename := m.currentFile
		if len(args) > 0 {
			filename = args[0]
		}
		if filename == "Untitled" {
			m.message = lipgloss.NewStyle().Foreground(errorColor).Render("Error: specify a name (/save file.go)")
			return nil
		}

		err := os.WriteFile(filename, []byte(m.editor.Value()), 0644)
		if err != nil {
			m.message = lipgloss.NewStyle().Foreground(errorColor).Render("Error: " + err.Error())
		} else {
			m.currentFile = filename
			m.isModified = false
			m.isSaved = true
			m.message = "Saved: " + filename
			m.explorerViewport.SetContent(getDirContent(m.currentFile, m.isModified, m.isSaved, m.explorerViewport.Width))
			m.codeViewport.SetContent(highlightCode(m.editor.Value(), m.currentFile, m.codeViewport.Width))
		}

	case "run":
		if m.currentFile == "Untitled" {
			m.message = lipgloss.NewStyle().Foreground(errorColor).Render("Save the file first to run it.")
			return nil
		}

		m.message = "Executing in background..."
		return runScriptCmd(m.currentFile)

	case "cd":
		if len(args) == 0 {
			m.message = lipgloss.NewStyle().Foreground(errorColor).Render("Error: specify directory.")
			return nil
		}
		err := os.Chdir(args[0])
		if err != nil {
			m.message = lipgloss.NewStyle().Foreground(errorColor).Render("Error: " + err.Error())
		} else {
			m.message = "Folder: " + args[0]
			m.explorerViewport.SetContent(getDirContent(m.currentFile, m.isModified, m.isSaved, m.explorerViewport.Width))
		}

	case "help":
		m.message = "Commands: open <f>, save <f>, run, cd <d>, help, q"

	case "q":
		os.Exit(0)

	default:
		m.message = lipgloss.NewStyle().Foreground(errorColor).Render("Unknown command: " + action)
	}
	return nil
}

func (m model) View() string {
	if !m.ready {
		return "\n  Initializing Terminas..."
	}

	if m.viewState == ViewStart {
		cwd, _ := os.Getwd()

		logo := renderLogo()
		headerInfo := renderHeaderInfo(m.username, cwd)
		headerBox := lipgloss.JoinHorizontal(lipgloss.Top, logo, "   ", headerInfo)
		hLine := dimStyle.Render(strings.Repeat("─", m.width))

		pUser := promptUserStyle.Render(m.username + "@terminas")
		pDir := promptDirStyle.Render(cwd)
		topPrompt := promptBracketStyle.Render("╭─[") + pUser + promptBracketStyle.Render("]─[") + pDir + promptBracketStyle.Render("]")

		var body string
		for {
			var consoleStr string
			if len(m.consoleOutput) > 0 {
				consoleStr = strings.Join(m.consoleOutput, "\n") + "\n"
			}

			inputLine := m.consoleInput.View()
			if m.ghostText != "" {
				inputLine += dimStyle.Render(m.ghostText)
			}

			body = lipgloss.JoinVertical(lipgloss.Left,
				headerBox,
				"",
				hLine,
				"",
				consoleStr+topPrompt+"\n"+inputLine,
			)

			bodyHeight := lipgloss.Height(body)
			if bodyHeight > m.height-2 && len(m.consoleOutput) > 0 {
				m.consoleOutput = m.consoleOutput[1:]
			} else {
				break
			}
		}

		footerLeft := dimStyle.Render("? Ctrl+T for console | Tab to autocomplete | Ctrl+O to toggle sidebar")
		footerRight := dimStyle.Render("Terminas IDE 1.0")
		spaces := m.width - lipgloss.Width(footerLeft) - lipgloss.Width(footerRight) - 1
		if spaces < 0 {
			spaces = 0
		}
		footer := footerLeft + strings.Repeat(" ", spaces) + footerRight

		bodyHeight := lipgloss.Height(body)
		paddingLines := m.height - bodyHeight - 2
		if paddingLines < 0 {
			paddingLines = 0
		}

		return body + strings.Repeat("\n", paddingLines+1) + footer
	}

	// --- EDITOR VIEW ---

	langIcon, langColor := getLanguageContext(m.currentFile)
	if m.currentFile == "Untitled" {
		langIcon = "📝"
		langColor = accentColor2
	}

	// Active border colors
	activeBorderColor := langColor
	if m.mode == CommandMode {
		activeBorderColor = warningColor
	}

	dynEditorStyle := editorStyle.Copy().BorderForeground(activeBorderColor)

	var sidebar string
	if m.sidebarMode == SidebarExplorer {
		dynExplorerStyle := explorerStyle.Copy().BorderForeground(cyanColor)
		explorerTab := tabStyle.Copy().Background(cyanColor).Foreground(bgMain).Render(" 📁 FILE EXPLORER ")
		explorerBody := dynExplorerStyle.
			Width(m.explorerViewport.Width).
			Height(m.explorerViewport.Height).
			Render(m.explorerViewport.View())
		sidebar = lipgloss.JoinVertical(lipgloss.Left, explorerTab, explorerBody)
	} else {
		// AI Chat 
		dynAiStyle := explorerStyle.Copy().BorderForeground(accentColor)
		
		titleStr := " 🤖 AI - CHAT "
		if m.isAiThinking {
			titleStr = " " + m.aiSpinner.View() + " Thinking... "
		}
		
		aiTab := tabStyle.Copy().Background(accentColor).Foreground(bgMain).Render(titleStr)
		aiBody := dynAiStyle.Width(m.aiViewport.Width).Height(m.aiViewport.Height).Render(m.aiViewport.View())
		sidebar = lipgloss.JoinVertical(lipgloss.Left, aiTab, aiBody)
	}

	// Coloring the Tab based on unsaved/saved state
	tabBgColor := langColor
	if m.isModified {
		tabBgColor = errorColor
	} else if m.isSaved {
		tabBgColor = successColor
	}

	dynTabStyle := tabStyle.Copy().Background(tabBgColor).Foreground(bgMain)

	fileNameDisplay := m.currentFile
	if m.isModified {
		fileNameDisplay += " *"
	}

	tab := dynTabStyle.Render(" " + langIcon + " " + fileNameDisplay + " ")

	var editorContent string
	if m.mode == InsertMode {
		taStyle := lipgloss.NewStyle().Foreground(langColor)
		m.editor.FocusedStyle.Text = taStyle
		m.editor.FocusedStyle.CursorLine = lipgloss.NewStyle().Background(bgDarker).Foreground(langColor)
		m.editor.Cursor.Style = lipgloss.NewStyle().Foreground(bgMain).Background(langColor)

		editorContent = m.editor.View()
	} else {
		editorContent = m.codeViewport.View()
	}

	editorBody := dynEditorStyle.
		Width(m.editor.Width()).
		Height(m.editor.Height()).
		Render(editorContent)

	editorBlock := lipgloss.JoinVertical(lipgloss.Left, tab, editorBody)

	mainLayout := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, editorBlock)

	var modeBlock string
	switch m.mode {
	case NormalMode:
		modeBlock = statusBarHighlight.Render(" NORMAL ")
	case InsertMode:
		modeBlock = statusBarInsertMode.Render(" INSERT ")
	case CommandMode:
		modeBlock = statusBarCmdMode.Render(" COMMAND ")
	}

	infoText := " " + m.message
	if m.mode != CommandMode {
		contextStatus := "OFF"
		if m.useWorkspaceContext {
			contextStatus = "ON"
		}
		infoText = fmt.Sprintf(" ^O Sidebar | ^T Console | ^W Ctx:%s |%s", contextStatus, infoText)
	}
	
	// Apply static color to footer text
	fileInfo := statusBarStyle.Width(m.width - lipgloss.Width(modeBlock) - 1).Foreground(fgPrimary).Render(infoText)
	statusBar := lipgloss.JoinHorizontal(lipgloss.Top, modeBlock, fileInfo)

	if m.mode == CommandMode {
		cmdLine := m.commandInput.View()
		return lipgloss.JoinVertical(lipgloss.Left, mainLayout, statusBar, cmdLine)
	}

	return lipgloss.JoinVertical(lipgloss.Left, mainLayout, statusBar)
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running Terminas: %v\n", err)
		os.Exit(1)
	}
}
