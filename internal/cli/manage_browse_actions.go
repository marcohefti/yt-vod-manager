package cli

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"yt-vod-manager/internal/discovery"
)

const (
	manageActionSyncActive = iota
)

var manageActions = []string{
	"Sync Active Projects",
}

func (m manageModel) renderActionsPanel(width int) string {
	lines := make([]string, 0, len(manageActions)+2)
	lines = append(lines, "Actions")
	lines = append(lines, "")
	for i, action := range manageActions {
		row := "[>] " + action
		if m.isActionCursor() && m.selectedActionIndex() == i {
			row = manageSelStyle.Width(maxInt(width-4, 6)).Render(truncateRunes(row, maxInt(width-6, 10)))
			lines = append(lines, row)
			continue
		}
		lines = append(lines, truncateRunes(row, maxInt(width-6, 10)))
	}
	return managePanelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func toggleProjectActiveCmd(configPath string, project discovery.Project) tea.Cmd {
	return func() tea.Msg {
		nextActive := !isProjectActive(project)
		opts := discovery.AddProjectOptions{
			ConfigPath:          configPath,
			Name:                project.Name,
			SourceURL:           project.SourceURL,
			Profile:             project.Profile,
			OutputDir:           project.OutputDir,
			CookiesPath:         project.CookiesPath,
			CookiesFromBrowser:  project.CookiesFromBrowser,
			Workers:             project.Workers,
			Fragments:           project.Fragments,
			Order:               project.Order,
			Quality:             project.Quality,
			DeliveryMode:        project.DeliveryMode,
			NoSubs:              project.NoSubs,
			SubLangs:            project.SubLangs,
			Active:              boolPtr(nextActive),
			ReplaceIfNameExists: true,
		}
		res, err := discovery.AddProject(opts)
		if err != nil {
			return manageSaveMsg{err: err}
		}
		return manageSaveMsg{message: fmt.Sprintf("project %s active: %s", res.Project.Name, yesNo(isProjectActive(res.Project)))}
	}
}

func (m manageModel) totalBrowseRows() int {
	return (len(m.projects) + 1) + len(manageActions)
}

func (m manageModel) isActionCursor() bool {
	return m.cursor >= len(m.projects)+1
}

func (m manageModel) selectedActionIndex() int {
	idx := m.cursor - (len(m.projects) + 1)
	if idx < 0 {
		return 0
	}
	if idx >= len(manageActions) {
		return len(manageActions) - 1
	}
	return idx
}
