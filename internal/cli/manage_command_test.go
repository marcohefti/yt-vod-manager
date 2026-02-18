package cli

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestManageBoolFieldSupportsYN(t *testing.T) {
	m := manageModel{
		mode: manageModeForm,
		form: newManageForm(nil, 80),
	}
	if m.form == nil {
		t.Fatal("expected form")
	}
	m.form.Index = findFieldIndexByKey(m.form, "subtitles")
	if m.form.Index < 0 {
		t.Fatal("subtitles field not found")
	}

	model, _ := m.updateForm(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m2 := model.(manageModel)
	if got := m2.form.currentField().Value; got != "n" {
		t.Fatalf("expected subtitles value n after 'n', got %q", got)
	}

	model, _ = m2.updateForm(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m3 := model.(manageModel)
	if got := m3.form.currentField().Value; got != "y" {
		t.Fatalf("expected subtitles value y after 'y', got %q", got)
	}
}

func TestManageBoolFieldSupportsArrowAndSpace(t *testing.T) {
	m := manageModel{
		mode: manageModeForm,
		form: newManageForm(nil, 80),
	}
	if m.form == nil {
		t.Fatal("expected form")
	}
	m.form.Index = findFieldIndexByKey(m.form, "subtitles")
	if m.form.Index < 0 {
		t.Fatal("subtitles field not found")
	}

	model, _ := m.updateForm(tea.KeyMsg{Type: tea.KeyLeft})
	m2 := model.(manageModel)
	if got := m2.form.currentField().Value; got != "n" {
		t.Fatalf("expected subtitles value n after left, got %q", got)
	}

	model, _ = m2.updateForm(tea.KeyMsg{Type: tea.KeyRight})
	m3 := model.(manageModel)
	if got := m3.form.currentField().Value; got != "y" {
		t.Fatalf("expected subtitles value y after right, got %q", got)
	}

	model, _ = m3.updateForm(tea.KeyMsg{Type: tea.KeySpace})
	m4 := model.(manageModel)
	if got := m4.form.currentField().Value; got != "n" {
		t.Fatalf("expected subtitles value n after space, got %q", got)
	}
}

func TestManageBrowseSyncActiveSetsLaunchingStatus(t *testing.T) {
	m := manageModel{
		mode:   manageModeBrowse,
		cursor: 1, // len(projects)=0 => row 0 is [+] New Project, row 1 is first Action.
	}

	model, _ := m.updateBrowse(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := model.(manageModel)
	if !m2.launchSyncActive {
		t.Fatal("expected launchSyncActive=true")
	}
	if m2.statusMessage == "" {
		t.Fatal("expected non-empty status message")
	}
}

func findFieldIndexByKey(f *manageForm, key string) int {
	if f == nil {
		return -1
	}
	for i, field := range f.Fields {
		if field.Key == key {
			return i
		}
	}
	return -1
}
