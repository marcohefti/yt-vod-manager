package discovery

import "testing"

func TestLoadProjectRegistryAppliesUserDefaults(t *testing.T) {
	tmp := t.TempDir()
	cfg := tmp + "/projects.json"

	_, err := AddProject(AddProjectOptions{
		ConfigPath: cfg,
		Name:       "demo",
		SourceURL:  "https://example.com/src",
	})
	if err != nil {
		t.Fatalf("add project failed: %v", err)
	}

	reg, err := LoadProjects(cfg)
	if err != nil {
		t.Fatalf("load projects failed: %v", err)
	}
	if len(reg.Projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(reg.Projects))
	}
	p := reg.Projects[0]
	if p.Workers != DefaultWorkers {
		t.Fatalf("workers default mismatch: got %d want %d", p.Workers, DefaultWorkers)
	}
	if p.Fragments != DefaultFragments {
		t.Fatalf("fragments default mismatch: got %d want %d", p.Fragments, DefaultFragments)
	}
	if p.Order != DefaultOrder {
		t.Fatalf("order default mismatch: got %q want %q", p.Order, DefaultOrder)
	}
	if p.Quality != DefaultQuality {
		t.Fatalf("quality default mismatch: got %q want %q", p.Quality, DefaultQuality)
	}
	if p.SubLangs != DefaultSubtitleLanguage {
		t.Fatalf("subLangs default mismatch: got %q want %q", p.SubLangs, DefaultSubtitleLanguage)
	}
	if !isProjectActive(p) {
		t.Fatalf("active default mismatch: got inactive, want active")
	}
}

func TestResolveProjectSelectionFilteredActiveOnly(t *testing.T) {
	tmp := t.TempDir()
	cfg := tmp + "/projects.json"

	_, err := AddProject(AddProjectOptions{
		ConfigPath: cfg,
		Name:       "active-one",
		SourceURL:  "https://example.com/a",
		Active:     boolPtr(true),
	})
	if err != nil {
		t.Fatalf("add active project failed: %v", err)
	}
	_, err = AddProject(AddProjectOptions{
		ConfigPath: cfg,
		Name:       "inactive-one",
		SourceURL:  "https://example.com/b",
		Active:     boolPtr(false),
	})
	if err != nil {
		t.Fatalf("add inactive project failed: %v", err)
	}

	selected, err := ResolveProjectSelectionFiltered(cfg, "", true, true)
	if err != nil {
		t.Fatalf("resolve active-only selection failed: %v", err)
	}
	if len(selected) != 1 {
		t.Fatalf("expected 1 active project, got %d", len(selected))
	}
	if selected[0].Name != "active-one" {
		t.Fatalf("expected active-one, got %q", selected[0].Name)
	}
}
