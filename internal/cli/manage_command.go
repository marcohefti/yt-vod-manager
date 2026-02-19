package cli

import (
	"errors"
	"flag"
	"fmt"
	"strconv"
	"strings"

	"yt-vod-manager/internal/discovery"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type manageMode int

const (
	manageModeBrowse manageMode = iota
	manageModeForm
	manageModeDeleteConfirm
)

type manageFormKind int

const (
	manageFormKindProject manageFormKind = iota
	manageFormKindGlobal
)

type manageFieldKind int

const (
	manageFieldString manageFieldKind = iota
	manageFieldInt
	manageFieldBool
	manageFieldSelect
)

type manageFormField struct {
	Key      string
	Label    string
	Help     string
	Kind     manageFieldKind
	Value    string
	Options  []string
	Required bool
}

type manageForm struct {
	Kind        manageFormKind
	Title       string
	IsEdit      bool
	ProjectName string
	Fields      []manageFormField
	Index       int
	Input       textinput.Model
	Error       string
	Saving      bool
}

type manageModel struct {
	configPath string
	projects   []discovery.Project
	global     discovery.GlobalSettings
	cursor     int
	width      int
	height     int
	mode       manageMode
	form       *manageForm

	confirmDeleteName string
	statusMessage     string
	launchSyncActive  bool
	fatalErr          error
}

type manageLoadedMsg struct {
	projects []discovery.Project
	global   discovery.GlobalSettings
	err      error
}

type manageSaveMsg struct {
	message string
	err     error
}

type manageDeleteMsg struct {
	message string
	err     error
}

var (
	manageTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	manageMutedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	manageErrorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true)
	manageOKStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	managePanelStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	manageSelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("62")).Bold(true)
)

func runManage(args []string) error {
	fs := flag.NewFlagSet("manage", flag.ContinueOnError)
	config := fs.String("config", discovery.DefaultProjectsConfigPath, "project config path")
	fs.SetOutput(flag.CommandLine.Output())
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !stdinIsTTY() {
		return errors.New("manage requires an interactive terminal (TTY)")
	}

	m := manageModel{
		configPath: strings.TrimSpace(*config),
		mode:       manageModeBrowse,
		cursor:     0,
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "tty") {
			return errors.New("manage requires an interactive terminal (TTY)")
		}
		return err
	}
	if fm, ok := finalModel.(manageModel); ok {
		if fm.launchSyncActive {
			fmt.Println("sync active projects: refreshing sources...")
			return runSync([]string{
				"--all-projects",
				"--active-only",
				"--config", fm.configPath,
			})
		}
		return fm.fatalErr
	}
	return nil
}

func (m manageModel) Init() tea.Cmd {
	return loadProjectsCmd(m.configPath)
}

func (m manageModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.form != nil {
			m.form = resizeFormInput(m.form, m.width)
		}
		return m, nil
	case manageLoadedMsg:
		if msg.err != nil {
			m.fatalErr = msg.err
			return m, tea.Quit
		}
		m.projects = msg.projects
		m.global = msg.global
		if m.cursor < 0 {
			m.cursor = 0
		}
		total := m.totalBrowseRows()
		if total <= 0 {
			m.cursor = 0
		} else if m.cursor > total-1 {
			m.cursor = total - 1
		}
		return m, nil
	case manageSaveMsg:
		if msg.err != nil {
			if m.form != nil {
				m.form.Error = msg.err.Error()
				m.form.Saving = false
			}
			return m, nil
		}
		m.mode = manageModeBrowse
		m.form = nil
		m.statusMessage = msg.message
		return m, loadProjectsCmd(m.configPath)
	case manageDeleteMsg:
		if msg.err != nil {
			m.statusMessage = "error: " + msg.err.Error()
			m.mode = manageModeBrowse
			m.confirmDeleteName = ""
			return m, nil
		}
		m.mode = manageModeBrowse
		m.confirmDeleteName = ""
		m.statusMessage = msg.message
		return m, loadProjectsCmd(m.configPath)
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch m.mode {
	case manageModeBrowse:
		return m.updateBrowse(keyMsg)
	case manageModeForm:
		return m.updateForm(keyMsg)
	case manageModeDeleteConfirm:
		return m.updateDeleteConfirm(keyMsg)
	default:
		return m, nil
	}
}

func (m manageModel) updateBrowse(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	totalItems := m.totalBrowseRows()
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case "down", "j":
		if m.cursor < totalItems-1 {
			m.cursor++
		}
		return m, nil
	case " ", "space":
		if m.isActionCursor() {
			return m, nil
		}
		if m.cursor >= len(m.projects) {
			return m, nil
		}
		selected := m.projects[m.cursor]
		return m, toggleProjectActiveCmd(m.configPath, selected)
	case "n":
		m.mode = manageModeForm
		m.form = newManageForm(nil, m.width)
		m.statusMessage = ""
		return m, nil
	case "r":
		return m, loadProjectsCmd(m.configPath)
	case "enter", "e":
		if m.isActionCursor() {
			switch m.selectedActionIndex() {
			case manageActionSyncActive:
				m.statusMessage = "sync active projects: launching sync..."
				m.launchSyncActive = true
				return m, tea.Quit
			case manageActionGlobalSettings:
				m.mode = manageModeForm
				m.form = newManageGlobalForm(m.global, m.width)
				m.statusMessage = ""
				return m, nil
			}
			return m, nil
		}
		if m.cursor == len(m.projects) {
			m.mode = manageModeForm
			m.form = newManageForm(nil, m.width)
			m.statusMessage = ""
			return m, nil
		}
		if len(m.projects) == 0 {
			m.statusMessage = "no projects configured yet"
			return m, nil
		}
		selected := m.projects[m.cursor]
		m.mode = manageModeForm
		m.form = newManageForm(&selected, m.width)
		m.statusMessage = ""
		return m, nil
	case "d":
		if len(m.projects) == 0 || m.cursor >= len(m.projects) {
			m.statusMessage = "select a project to delete"
			return m, nil
		}
		m.mode = manageModeDeleteConfirm
		m.confirmDeleteName = m.projects[m.cursor].Name
		return m, nil
	}
	return m, nil
}

func (m manageModel) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.form == nil {
		m.mode = manageModeBrowse
		return m, nil
	}
	if m.form.Saving {
		return m, nil
	}

	key := strings.ToLower(msg.String())
	switch key {
	case "ctrl+c", "esc":
		m.mode = manageModeBrowse
		m.form = nil
		m.statusMessage = "wizard cancelled"
		return m, nil
	case "up", "shift+tab":
		m.form.commitInput()
		if m.form.Index > 0 {
			m.form.Index--
		}
		m.form.loadFieldIntoInput()
		return m, nil
	case "down", "tab":
		m.form.commitInput()
		if m.form.Index < len(m.form.Fields)-1 {
			m.form.Index++
		}
		m.form.loadFieldIntoInput()
		return m, nil
	case " ", "space":
		kind := m.form.currentField().Kind
		if kind == manageFieldBool {
			m.form.toggleBoolField()
			return m, nil
		}
		if kind == manageFieldSelect {
			m.form.nextSelectOption()
			return m, nil
		}
	case "left", "h":
		kind := m.form.currentField().Kind
		if kind == manageFieldBool {
			m.form.toggleBoolField()
			return m, nil
		}
		if kind == manageFieldSelect {
			m.form.prevSelectOption()
			return m, nil
		}
	case "right", "l":
		kind := m.form.currentField().Kind
		if kind == manageFieldBool {
			m.form.toggleBoolField()
			return m, nil
		}
		if kind == manageFieldSelect {
			m.form.nextSelectOption()
			return m, nil
		}
	case "y":
		if m.form.currentField().Kind == manageFieldBool {
			m.form.setBoolField(true)
			return m, nil
		}
	case "n":
		if m.form.currentField().Kind == manageFieldBool {
			m.form.setBoolField(false)
			return m, nil
		}
	case "enter", "ctrl+s":
		m.form.commitInput()
		if m.form.Index < len(m.form.Fields)-1 && key != "ctrl+s" {
			m.form.Index++
			m.form.loadFieldIntoInput()
			return m, nil
		}
		if m.form.Kind == manageFormKindGlobal {
			global, err := m.form.toGlobalSettings()
			if err != nil {
				m.form.Error = err.Error()
				return m, nil
			}
			m.form.Error = ""
			m.form.Saving = true
			return m, saveGlobalSettingsCmd(m.configPath, global)
		}
		opts, err := m.form.toAddProjectOptions(m.configPath)
		if err != nil {
			m.form.Error = err.Error()
			return m, nil
		}
		m.form.Error = ""
		m.form.Saving = true
		return m, saveProjectCmd(opts)
	}

	kind := m.form.currentField().Kind
	if kind == manageFieldBool || kind == manageFieldSelect {
		return m, nil
	}
	var cmd tea.Cmd
	m.form.Input, cmd = m.form.Input.Update(msg)
	m.form.Fields[m.form.Index].Value = m.form.Input.Value()
	return m, cmd
}

func (m manageModel) updateDeleteConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc", "n":
		m.mode = manageModeBrowse
		m.confirmDeleteName = ""
		m.statusMessage = "delete cancelled"
		return m, nil
	case "y", "enter":
		name := strings.TrimSpace(m.confirmDeleteName)
		if name == "" {
			m.mode = manageModeBrowse
			m.statusMessage = "delete cancelled"
			return m, nil
		}
		return m, deleteProjectCmd(m.configPath, name)
	}
	return m, nil
}

func (m manageModel) View() string {
	if m.fatalErr != nil {
		return manageErrorStyle.Render("fatal: " + m.fatalErr.Error())
	}
	if m.width <= 0 {
		m.width = 100
	}
	if m.height <= 0 {
		m.height = 30
	}

	switch m.mode {
	case manageModeForm:
		return m.viewForm()
	case manageModeDeleteConfirm:
		return m.viewDeleteConfirm()
	default:
		return m.viewBrowse()
	}
}

func (m manageModel) viewBrowse() string {
	header := manageTitleStyle.Render("yt-vod-manager manage") + "\n" +
		manageMutedStyle.Render("up/down: move | space: toggle active | enter/e: edit/run | n: new | d: delete | r: refresh | q: quit")

	if m.width < 90 {
		list := m.renderListPanel(m.width)
		actions := m.renderActionsPanel(m.width)
		details := m.renderDetailsPanel(m.width)
		body := lipgloss.JoinVertical(lipgloss.Left, list, actions, details)
		status := m.renderStatusLine(m.width)
		return lipgloss.JoinVertical(lipgloss.Left, header, body, status)
	}

	leftW := clampInt(m.width/2, 34, 56)
	rightW := m.width - leftW - 1
	list := m.renderListPanel(leftW)
	actions := m.renderActionsPanel(leftW)
	left := lipgloss.JoinVertical(lipgloss.Left, list, actions)
	right := m.renderDetailsPanel(rightW)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	status := m.renderStatusLine(m.width)
	return lipgloss.JoinVertical(lipgloss.Left, header, body, status)
}

func (m manageModel) renderListPanel(width int) string {
	total := len(m.projects) + 1
	maxRows := clampInt(m.height-14, 4, 18)
	listCursor := m.cursor
	if listCursor >= total {
		listCursor = total - 1
	}
	start, end := listWindow(total, listCursor, maxRows)

	lines := make([]string, 0, maxRows+3)
	if len(m.projects) == 0 {
		lines = append(lines, manageMutedStyle.Render("No projects yet."))
		lines = append(lines, manageMutedStyle.Render("Select '[+] New Project' and press Enter."))
	}
	if start > 0 {
		lines = append(lines, manageMutedStyle.Render("..."))
	}
	for i := start; i < end; i++ {
		line := ""
		if i == len(m.projects) {
			line = "[+] New Project (wizard)"
		} else {
			p := m.projects[i]
			activeMark := " "
			if isProjectActive(p) {
				activeMark = "x"
			}
			line = fmt.Sprintf("[%s] %s  %s", activeMark, p.Name, p.SourceURL)
		}
		line = truncateRunes(line, maxInt(width-6, 10))
		if i == m.cursor {
			line = manageSelStyle.Width(maxInt(width-4, 6)).Render(line)
		}
		lines = append(lines, line)
	}
	if end < total {
		lines = append(lines, manageMutedStyle.Render("..."))
	}

	content := strings.Join(lines, "\n")
	return managePanelStyle.Width(width).Render(content)
}

func (m manageModel) renderDetailsPanel(width int) string {
	lines := []string{}
	if m.isActionCursor() {
		lines = append(lines, "Action")
		lines = append(lines, "")
		switch m.selectedActionIndex() {
		case manageActionSyncActive:
			lines = append(lines, "Sync Active Projects")
			lines = append(lines, "")
			lines = append(lines, "Runs sync for all projects with active=yes.")
			lines = append(lines, "Press Enter to launch sync view.")
		case manageActionGlobalSettings:
			lines = append(lines, "Global Settings")
			lines = append(lines, kv("workers", strconv.Itoa(m.global.Workers)))
			lines = append(lines, kv("download_limit_mb_s", formatFloat(m.global.DownloadLimitMBps)))
			lines = append(lines, kv("proxy_mode", m.global.ProxyMode))
			lines = append(lines, kv("proxies", strconv.Itoa(len(m.global.Proxies))))
			lines = append(lines, "")
			lines = append(lines, "Press Enter to edit global defaults.")
		default:
			lines = append(lines, "Select an action.")
		}
	} else if m.cursor >= len(m.projects) {
		lines = append(lines, "New Project Wizard")
		lines = append(lines, "")
		lines = append(lines, "Press Enter or n to create a project.")
		lines = append(lines, "The wizard guides source URL, defaults, and settings.")
	} else if len(m.projects) > 0 {
		p := m.projects[m.cursor]
		lines = append(lines, "Project Details")
		lines = append(lines, "")
		lines = append(lines, kv("name", p.Name))
		lines = append(lines, kv("source", p.SourceURL))
		lines = append(lines, kv("active", yesNo(isProjectActive(p))))
		lines = append(lines, kv("quality", defaultIfEmpty(p.Quality, discovery.DefaultQuality)))
		lines = append(lines, kv("js_runtime", defaultIfEmpty(p.JSRuntime, discovery.DefaultJSRuntime)))
		lines = append(lines, kv("output_dir", defaultIfEmpty(p.OutputDir, "(run default)")))
		lines = append(lines, kv("browser_cookies", yesNo(strings.TrimSpace(p.CookiesFromBrowser) != "")))
		lines = append(lines, kv("cookies_file", yesNo(strings.TrimSpace(p.CookiesPath) != "")))
		lines = append(lines, kv("workers", formatWorkerOverride(p.Workers)))
		lines = append(lines, kv("fragments", formatIntDefault(p.Fragments)))
		lines = append(lines, kv("order", defaultIfEmpty(p.Order, discovery.DefaultOrder)))
		lines = append(lines, kv("delivery", defaultIfEmpty(p.DeliveryMode, "(sync default: auto)")))
		lines = append(lines, kv("subtitles", yesNo(!p.NoSubs)))
		lines = append(lines, kv("subtitle_language", normalizeSubtitleChoice(p.SubLangs)))
	} else {
		lines = append(lines, "No projects configured")
		lines = append(lines, "")
		lines = append(lines, "Press n to start the project wizard.")
	}

	for i := range lines {
		lines[i] = wrapOrTrim(lines[i], maxInt(width-6, 12))
	}
	return managePanelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (m manageModel) renderStatusLine(width int) string {
	msg := strings.TrimSpace(m.statusMessage)
	if msg == "" {
		msg = "Tip: space toggles project active; go down to Actions to sync active projects."
	}
	style := manageMutedStyle
	if strings.HasPrefix(strings.ToLower(msg), "error:") {
		style = manageErrorStyle
	} else if strings.HasPrefix(strings.ToLower(msg), "project ") || strings.HasPrefix(strings.ToLower(msg), "updated") {
		style = manageOKStyle
	}
	return style.Width(width).Render(truncateRunes(msg, maxInt(width-2, 10)))
}

func (m manageModel) viewForm() string {
	if m.form == nil {
		return ""
	}
	header := manageTitleStyle.Render(m.form.Title)
	hints := manageMutedStyle.Render("tab/shift+tab or up/down: move | left/right/space: toggle | y/n: set yes/no | enter: next/save | ctrl+s: save | esc: cancel")

	lines := make([]string, 0, len(m.form.Fields)+6)
	for i, f := range m.form.Fields {
		prefix := "  "
		if i == m.form.Index {
			prefix = "> "
		}
		display := strings.TrimSpace(f.Value)
		if f.Kind == manageFieldBool {
			v, _ := parseBool(display)
			display = yesNo(v)
		}
		if display == "" {
			display = manageMutedStyle.Render("(empty)")
		}
		if f.Kind == manageFieldSelect {
			display = "[" + display + "]"
		}
		line := fmt.Sprintf("%s%s: %s", prefix, f.Label, display)
		lines = append(lines, wrapOrTrim(line, maxInt(m.width-6, 20)))
	}

	curr := m.form.currentField()
	inputLabel := fmt.Sprintf("\n%s\n", curr.Label)
	inputHelp := ""
	if strings.TrimSpace(curr.Help) != "" {
		inputHelp = manageMutedStyle.Render(curr.Help) + "\n"
	}
	input := m.form.Input.View()
	status := ""
	if m.form.Saving {
		status = manageMutedStyle.Render("\nSaving...")
	}
	if strings.TrimSpace(m.form.Error) != "" {
		status = "\n" + manageErrorStyle.Render(m.form.Error)
	}

	panel := managePanelStyle.Width(maxInt(m.width, 40)).Render(strings.Join(lines, "\n") + inputLabel + inputHelp + input + status)
	return lipgloss.JoinVertical(lipgloss.Left, header, hints, panel)
}

func (m manageModel) viewDeleteConfirm() string {
	text := fmt.Sprintf(
		"Delete project '%s'?\n\nThis removes it from config only.\nRuns/downloads remain on disk.\n\nPress y or Enter to confirm, n or Esc to cancel.",
		m.confirmDeleteName,
	)
	boxW := clampInt(m.width-8, 36, 80)
	boxH := clampInt(m.height-6, 9, 14)
	panel := managePanelStyle.Width(boxW).Height(boxH).Render(text)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, panel)
}

func loadProjectsCmd(configPath string) tea.Cmd {
	return func() tea.Msg {
		reg, err := discovery.LoadProjects(configPath)
		if err != nil {
			return manageLoadedMsg{err: err}
		}
		return manageLoadedMsg{projects: reg.Projects, global: reg.Global}
	}
}

func saveProjectCmd(opts discovery.AddProjectOptions) tea.Cmd {
	return func() tea.Msg {
		res, err := discovery.AddProject(opts)
		if err != nil {
			return manageSaveMsg{err: err}
		}
		if res.Created {
			return manageSaveMsg{message: "project added: " + res.Project.Name}
		}
		return manageSaveMsg{message: "project updated: " + res.Project.Name}
	}
}

func deleteProjectCmd(configPath, name string) tea.Cmd {
	return func() tea.Msg {
		_, err := discovery.RemoveProject(discovery.RemoveProjectOptions{ConfigPath: configPath, Name: name})
		if err != nil {
			return manageDeleteMsg{err: err}
		}
		return manageDeleteMsg{message: "project removed: " + name}
	}
}

func newManageForm(existing *discovery.Project, width int) *manageForm {
	f := &manageForm{Kind: manageFormKindProject}
	if existing == nil {
		f.Title = "New Project Wizard"
		f.IsEdit = false
		f.Fields = []manageFormField{
			{Key: "source", Label: "Source URL", Help: "Playlist or channel URL", Kind: manageFieldString, Required: true},
			{Key: "name", Label: "Project Name", Help: "Optional; leave empty for auto-name", Kind: manageFieldString},
			{Key: "active", Label: "Active", Help: "Included in 'Sync Active Projects'", Kind: manageFieldBool, Value: "y"},
			{Key: "quality", Label: "Quality", Help: "Best, 1080p, or 720p", Kind: manageFieldSelect, Value: discovery.DefaultQuality, Options: []string{"best", "1080p", "720p"}},
			{Key: "js_runtime", Label: "JS Runtime", Help: "Extractor JavaScript runtime. Auto follows yt-dlp default.", Kind: manageFieldSelect, Value: discovery.DefaultJSRuntime, Options: []string{discovery.JSRuntimeAuto, discovery.JSRuntimeDeno, discovery.JSRuntimeNode, discovery.JSRuntimeQuickJS, discovery.JSRuntimeBun}},
			{Key: "workers", Label: "Workers", Help: "Project override; 0 inherits global/default", Kind: manageFieldInt, Value: "0"},
			{Key: "fragments", Label: "Fragments", Help: "How many chunks per video stream", Kind: manageFieldInt, Value: strconv.Itoa(discovery.DefaultFragments)},
			{Key: "order", Label: "Download Order", Help: "Oldest first is safest for large backfills", Kind: manageFieldSelect, Value: discovery.DefaultOrder, Options: []string{"oldest", "newest", "manifest"}},
			{Key: "subtitles", Label: "Subtitles", Help: "Download subtitles when available", Kind: manageFieldBool, Value: "y"},
			{Key: "sub_langs", Label: "Subtitle Language", Help: "English or all available languages", Kind: manageFieldSelect, Value: discovery.DefaultSubtitleLanguage, Options: []string{"english", "all"}},
			{Key: "use_browser_cookies", Label: "Browser Cookies", Help: browserCookiesFormHelp, Kind: manageFieldBool, Value: "n"},
			{Key: "cookies_path", Label: "Cookies File Path", Help: "Optional cookies.txt path", Kind: manageFieldString},
			{Key: "output_dir", Label: "Output Dir", Help: "Optional override", Kind: manageFieldString},
			{Key: "delivery", Label: "Delivery Mode", Help: "Auto is recommended", Kind: manageFieldSelect, Value: "auto", Options: []string{"auto", "fragmented"}},
		}
	} else {
		f.Title = "Edit Project: " + existing.Name
		f.IsEdit = true
		f.ProjectName = existing.Name
		f.Fields = []manageFormField{
			{Key: "source", Label: "Source URL", Help: "Playlist or channel URL", Kind: manageFieldString, Required: true, Value: existing.SourceURL},
			{Key: "active", Label: "Active", Help: "Included in 'Sync Active Projects'", Kind: manageFieldBool, Value: boolToYN(isProjectActive(*existing))},
			{Key: "quality", Label: "Quality", Help: "Best, 1080p, or 720p", Kind: manageFieldSelect, Value: defaultIfEmpty(existing.Quality, discovery.DefaultQuality), Options: []string{"best", "1080p", "720p"}},
			{Key: "js_runtime", Label: "JS Runtime", Help: "Extractor JavaScript runtime. Auto follows yt-dlp default.", Kind: manageFieldSelect, Value: defaultIfEmpty(existing.JSRuntime, discovery.DefaultJSRuntime), Options: []string{discovery.JSRuntimeAuto, discovery.JSRuntimeDeno, discovery.JSRuntimeNode, discovery.JSRuntimeQuickJS, discovery.JSRuntimeBun}},
			{Key: "workers", Label: "Workers", Help: "Project override; 0 inherits global/default", Kind: manageFieldInt, Value: strconv.Itoa(existing.Workers)},
			{Key: "fragments", Label: "Fragments", Help: "How many chunks per video stream", Kind: manageFieldInt, Value: strconv.Itoa(maxInt(existing.Fragments, discovery.DefaultFragments))},
			{Key: "order", Label: "Download Order", Help: "Oldest first is safest for large backfills", Kind: manageFieldSelect, Value: defaultIfEmpty(existing.Order, discovery.DefaultOrder), Options: []string{"oldest", "newest", "manifest"}},
			{Key: "subtitles", Label: "Subtitles", Help: "Download subtitles when available", Kind: manageFieldBool, Value: boolToYN(!existing.NoSubs)},
			{Key: "sub_langs", Label: "Subtitle Language", Help: "English or all available languages", Kind: manageFieldSelect, Value: normalizeSubtitleChoice(existing.SubLangs), Options: []string{"english", "all"}},
			{Key: "use_browser_cookies", Label: "Browser Cookies", Help: browserCookiesFormHelp, Kind: manageFieldBool, Value: boolToYN(strings.TrimSpace(existing.CookiesFromBrowser) != "")},
			{Key: "cookies_path", Label: "Cookies File Path", Help: "Optional cookies.txt path", Kind: manageFieldString, Value: existing.CookiesPath},
			{Key: "output_dir", Label: "Output Dir", Help: "Optional override", Kind: manageFieldString, Value: existing.OutputDir},
			{Key: "delivery", Label: "Delivery Mode", Help: "Auto is recommended", Kind: manageFieldSelect, Value: defaultIfEmpty(existing.DeliveryMode, "auto"), Options: []string{"auto", "fragmented"}},
		}
	}

	input := textinput.New()
	input.Prompt = "> "
	input.CharLimit = 1024
	input.Width = clampInt(width-8, 20, 120)
	f.Input = input
	f.loadFieldIntoInput()
	f.Input.Focus()
	return f
}

func (f *manageForm) currentField() manageFormField {
	if len(f.Fields) == 0 {
		return manageFormField{}
	}
	if f.Index < 0 {
		f.Index = 0
	}
	if f.Index >= len(f.Fields) {
		f.Index = len(f.Fields) - 1
	}
	return f.Fields[f.Index]
}

func (f *manageForm) commitInput() {
	if f == nil || len(f.Fields) == 0 {
		return
	}
	f.Fields[f.Index].Value = strings.TrimSpace(f.Input.Value())
}

func (f *manageForm) loadFieldIntoInput() {
	if f == nil || len(f.Fields) == 0 {
		return
	}
	f.Input.SetValue(f.Fields[f.Index].Value)
	f.Input.CursorEnd()
}

func (f *manageForm) toggleBoolField() {
	if f == nil || len(f.Fields) == 0 {
		return
	}
	curr := f.Fields[f.Index]
	if curr.Kind != manageFieldBool {
		return
	}
	v, ok := parseBool(curr.Value)
	if !ok {
		v = false
	}
	curr.Value = boolToYN(!v)
	f.Fields[f.Index] = curr
	f.loadFieldIntoInput()
}

func (f *manageForm) setBoolField(v bool) {
	if f == nil || len(f.Fields) == 0 {
		return
	}
	curr := f.Fields[f.Index]
	if curr.Kind != manageFieldBool {
		return
	}
	curr.Value = boolToYN(v)
	f.Fields[f.Index] = curr
	f.loadFieldIntoInput()
}

func (f *manageForm) nextSelectOption() {
	if f == nil || len(f.Fields) == 0 {
		return
	}
	curr := f.Fields[f.Index]
	if curr.Kind != manageFieldSelect || len(curr.Options) == 0 {
		return
	}
	current := strings.TrimSpace(curr.Value)
	pos := 0
	for i, opt := range curr.Options {
		if strings.EqualFold(opt, current) {
			pos = i
			break
		}
	}
	pos = (pos + 1) % len(curr.Options)
	curr.Value = curr.Options[pos]
	f.Fields[f.Index] = curr
	f.loadFieldIntoInput()
}

func (f *manageForm) prevSelectOption() {
	if f == nil || len(f.Fields) == 0 {
		return
	}
	curr := f.Fields[f.Index]
	if curr.Kind != manageFieldSelect || len(curr.Options) == 0 {
		return
	}
	current := strings.TrimSpace(curr.Value)
	pos := 0
	for i, opt := range curr.Options {
		if strings.EqualFold(opt, current) {
			pos = i
			break
		}
	}
	pos = (pos - 1 + len(curr.Options)) % len(curr.Options)
	curr.Value = curr.Options[pos]
	f.Fields[f.Index] = curr
	f.loadFieldIntoInput()
}

func (f *manageForm) toAddProjectOptions(configPath string) (discovery.AddProjectOptions, error) {
	if f == nil {
		return discovery.AddProjectOptions{}, errors.New("internal form error")
	}
	vals := make(map[string]string, len(f.Fields))
	for _, field := range f.Fields {
		v := strings.TrimSpace(field.Value)
		if field.Required && v == "" {
			return discovery.AddProjectOptions{}, fmt.Errorf("%s is required", strings.ToLower(field.Label))
		}
		switch field.Kind {
		case manageFieldInt:
			if v == "" {
				v = "0"
			}
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				return discovery.AddProjectOptions{}, fmt.Errorf("%s must be an integer >= 0", strings.ToLower(field.Label))
			}
		case manageFieldBool:
			if _, ok := parseBool(v); !ok {
				return discovery.AddProjectOptions{}, fmt.Errorf("%s must be y or n", strings.ToLower(field.Label))
			}
		case manageFieldSelect:
			if len(field.Options) == 0 {
				break
			}
			matched := false
			for _, opt := range field.Options {
				if strings.EqualFold(opt, v) {
					v = opt
					matched = true
					break
				}
			}
			if !matched {
				return discovery.AddProjectOptions{}, fmt.Errorf("%s has invalid value", strings.ToLower(field.Label))
			}
		}
		vals[field.Key] = v
	}

	workers, _ := strconv.Atoi(defaultIfEmpty(vals["workers"], "0"))
	fragments, _ := strconv.Atoi(defaultIfEmpty(vals["fragments"], "0"))
	subtitlesOn, _ := parseBool(defaultIfEmpty(vals["subtitles"], "y"))
	active, _ := parseBool(defaultIfEmpty(vals["active"], "y"))
	useBrowserCookies, _ := parseBool(defaultIfEmpty(vals["use_browser_cookies"], "n"))
	cookiesFromBrowser := ""
	if useBrowserCookies {
		cookiesFromBrowser = discovery.DefaultBrowserCookieAgent
	}
	subLangs := normalizeSubtitleValue(vals["sub_langs"])

	name := strings.TrimSpace(vals["name"])
	replace := false
	if f.IsEdit {
		name = f.ProjectName
		replace = true
	}

	return discovery.AddProjectOptions{
		ConfigPath:          configPath,
		Name:                name,
		SourceURL:           strings.TrimSpace(vals["source"]),
		Profile:             discovery.DefaultProfileName,
		OutputDir:           strings.TrimSpace(vals["output_dir"]),
		CookiesPath:         strings.TrimSpace(vals["cookies_path"]),
		CookiesFromBrowser:  cookiesFromBrowser,
		Workers:             workers,
		Fragments:           fragments,
		Order:               strings.TrimSpace(vals["order"]),
		Quality:             strings.TrimSpace(vals["quality"]),
		JSRuntime:           strings.TrimSpace(vals["js_runtime"]),
		DeliveryMode:        strings.TrimSpace(vals["delivery"]),
		NoSubs:              !subtitlesOn,
		SubLangs:            subLangs,
		Active:              boolPtr(active),
		ReplaceIfNameExists: replace,
	}, nil
}
