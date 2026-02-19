package discovery

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"yt-vod-manager/internal/runstore"
)

const (
	DefaultProjectsConfigPath = "config/projects.json"
	projectSchemaVersion      = 2
)

var (
	ErrNoProjectsConfigured  = errors.New("no projects configured")
	ErrProjectSelectRequired = errors.New("project selection required")
)

type Project struct {
	Name               string `json:"name"`
	SourceURL          string `json:"source_url"`
	Active             *bool  `json:"active,omitempty"`
	Profile            string `json:"profile,omitempty"`
	OutputDir          string `json:"output_dir,omitempty"`
	CookiesPath        string `json:"cookies_path,omitempty"`
	CookiesFromBrowser string `json:"cookies_from_browser,omitempty"`
	Workers            int    `json:"workers,omitempty"`
	Fragments          int    `json:"fragments,omitempty"`
	Order              string `json:"order,omitempty"`
	Quality            string `json:"quality,omitempty"`
	JSRuntime          string `json:"js_runtime,omitempty"`
	DeliveryMode       string `json:"delivery_mode,omitempty"`
	NoSubs             bool   `json:"no_subs,omitempty"`
	SubLangs           string `json:"sub_langs,omitempty"`
}

type ProjectRegistry struct {
	SchemaVersion int            `json:"schema_version"`
	UpdatedAt     string         `json:"updated_at"`
	Global        GlobalSettings `json:"global,omitempty"`
	Projects      []Project      `json:"projects"`
}

type AddProjectOptions struct {
	ConfigPath          string
	Name                string
	SourceURL           string
	Profile             string
	OutputDir           string
	CookiesPath         string
	CookiesFromBrowser  string
	Workers             int
	Fragments           int
	Order               string
	Quality             string
	JSRuntime           string
	DeliveryMode        string
	NoSubs              bool
	SubLangs            string
	Active              *bool
	ReplaceIfNameExists bool
}

type AddProjectResult struct {
	Project Project
	Created bool
}

type RemoveProjectOptions struct {
	ConfigPath string
	Name       string
}

type RemoveProjectResult struct {
	Project Project
	Removed bool
}

type ListProjectsOptions struct {
	ConfigPath string
}

type ListProjectsResult struct {
	ConfigPath string
	Projects   []Project
}

func normalizeConfigPath(path string) string {
	p := strings.TrimSpace(path)
	if p == "" {
		return DefaultProjectsConfigPath
	}
	return p
}

func EnsureProjectRegistry(configPath string) (ProjectRegistry, bool, error) {
	path := normalizeConfigPath(configPath)
	reg, err := loadProjectRegistry(path)
	if err == nil {
		return reg, false, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return ProjectRegistry{}, false, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	reg = ProjectRegistry{
		SchemaVersion: projectSchemaVersion,
		UpdatedAt:     now,
		Global:        defaultGlobalSettings(),
		Projects:      []Project{},
	}
	if err := saveProjectRegistry(path, reg); err != nil {
		return ProjectRegistry{}, false, err
	}
	return reg, true, nil
}

func AddProject(opts AddProjectOptions) (AddProjectResult, error) {
	configPath := normalizeConfigPath(opts.ConfigPath)
	reg, _, err := EnsureProjectRegistry(configPath)
	if err != nil {
		return AddProjectResult{}, err
	}

	sourceURL := strings.TrimSpace(opts.SourceURL)
	if sourceURL == "" {
		return AddProjectResult{}, fmt.Errorf("source URL is required")
	}
	if opts.Workers < 0 {
		return AddProjectResult{}, fmt.Errorf("workers must be >= 0")
	}
	if opts.Fragments < 0 {
		return AddProjectResult{}, fmt.Errorf("fragments must be >= 0")
	}
	jsRuntime, ok := parseJSRuntime(opts.JSRuntime)
	if !ok {
		return AddProjectResult{}, fmt.Errorf("js runtime must be auto or a comma-separated list of: deno, node, quickjs, bun")
	}
	canonicalSource := normalizeSourceURL(sourceURL)
	for _, p := range reg.Projects {
		if normalizeSourceURL(p.SourceURL) == canonicalSource && !equalsFoldAndTrim(p.Name, opts.Name) {
			return AddProjectResult{}, fmt.Errorf("source already tracked by project %q", p.Name)
		}
	}

	explicitName := canonicalProjectName(opts.Name)
	name := explicitName
	if name == "" {
		name = suggestProjectName(sourceURL)
	}
	if explicitName == "" {
		name = ensureUniqueProjectName(name, reg.Projects, opts.ReplaceIfNameExists)
	}
	if name == "" {
		return AddProjectResult{}, fmt.Errorf("project name is required")
	}

	project := Project{
		Name:               name,
		SourceURL:          sourceURL,
		Active:             opts.Active,
		Profile:            strings.TrimSpace(opts.Profile),
		OutputDir:          strings.TrimSpace(opts.OutputDir),
		CookiesPath:        strings.TrimSpace(opts.CookiesPath),
		CookiesFromBrowser: strings.TrimSpace(opts.CookiesFromBrowser),
		Workers:            opts.Workers,
		Fragments:          opts.Fragments,
		Order:              strings.TrimSpace(opts.Order),
		Quality:            strings.TrimSpace(opts.Quality),
		JSRuntime:          jsRuntime,
		DeliveryMode:       strings.TrimSpace(opts.DeliveryMode),
		NoSubs:             opts.NoSubs,
		SubLangs:           strings.TrimSpace(opts.SubLangs),
	}
	if project.Profile == "" {
		project.Profile = DefaultProfileName
	}
	if project.Active == nil {
		project.Active = boolPtr(true)
	}
	if project.Fragments <= 0 {
		project.Fragments = DefaultFragments
	}
	if project.Order == "" {
		project.Order = DefaultOrder
	}
	if project.Quality == "" {
		project.Quality = DefaultQuality
	}
	if project.JSRuntime == "" {
		project.JSRuntime = DefaultJSRuntime
	}
	if project.SubLangs == "" {
		project.SubLangs = DefaultSubtitleLanguage
	}

	created := true
	replaced := false
	for i := range reg.Projects {
		if strings.EqualFold(reg.Projects[i].Name, name) {
			if !opts.ReplaceIfNameExists {
				return AddProjectResult{}, fmt.Errorf("project %q already exists (use --replace)", name)
			}
			reg.Projects[i] = project
			created = false
			replaced = true
			break
		}
	}
	if !replaced {
		reg.Projects = append(reg.Projects, project)
	}

	sort.Slice(reg.Projects, func(i, j int) bool {
		return reg.Projects[i].Name < reg.Projects[j].Name
	})
	reg.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := saveProjectRegistry(configPath, reg); err != nil {
		return AddProjectResult{}, err
	}

	return AddProjectResult{
		Project: project,
		Created: created,
	}, nil
}

func RemoveProject(opts RemoveProjectOptions) (RemoveProjectResult, error) {
	configPath := normalizeConfigPath(opts.ConfigPath)
	reg, _, err := EnsureProjectRegistry(configPath)
	if err != nil {
		return RemoveProjectResult{}, err
	}

	name := canonicalProjectName(opts.Name)
	if name == "" {
		return RemoveProjectResult{}, fmt.Errorf("project name is required")
	}

	for i := range reg.Projects {
		if strings.EqualFold(reg.Projects[i].Name, name) {
			removed := reg.Projects[i]
			reg.Projects = append(reg.Projects[:i], reg.Projects[i+1:]...)
			reg.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			if err := saveProjectRegistry(configPath, reg); err != nil {
				return RemoveProjectResult{}, err
			}
			return RemoveProjectResult{Project: removed, Removed: true}, nil
		}
	}

	return RemoveProjectResult{}, fmt.Errorf("project %q not found", name)
}

func ListProjects(opts ListProjectsOptions) (ListProjectsResult, error) {
	configPath := normalizeConfigPath(opts.ConfigPath)
	reg, _, err := EnsureProjectRegistry(configPath)
	if err != nil {
		return ListProjectsResult{}, err
	}

	projects := append([]Project(nil), reg.Projects...)
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Name < projects[j].Name
	})

	return ListProjectsResult{
		ConfigPath: configPath,
		Projects:   projects,
	}, nil
}

func LoadProjects(configPath string) (ProjectRegistry, error) {
	reg, _, err := EnsureProjectRegistry(configPath)
	if err != nil {
		return ProjectRegistry{}, err
	}
	return reg, nil
}

func FindProjectByName(configPath, name string) (Project, error) {
	reg, _, err := EnsureProjectRegistry(configPath)
	if err != nil {
		return Project{}, err
	}
	target := canonicalProjectName(name)
	if target == "" {
		return Project{}, fmt.Errorf("project name is required")
	}

	for _, p := range reg.Projects {
		if strings.EqualFold(p.Name, target) {
			return p, nil
		}
	}
	return Project{}, fmt.Errorf("project %q not found", target)
}

func ResolveProjectSelection(configPath string, projectName string, all bool) ([]Project, error) {
	return ResolveProjectSelectionFiltered(configPath, projectName, all, false)
}

func ResolveProjectSelectionFiltered(configPath string, projectName string, all bool, activeOnly bool) ([]Project, error) {
	reg, _, err := EnsureProjectRegistry(configPath)
	if err != nil {
		return nil, err
	}
	if len(reg.Projects) == 0 {
		return nil, fmt.Errorf("%w in %s", ErrNoProjectsConfigured, normalizeConfigPath(configPath))
	}

	if all {
		projects := make([]Project, 0, len(reg.Projects))
		for _, p := range reg.Projects {
			if activeOnly && !isProjectActive(p) {
				continue
			}
			projects = append(projects, p)
		}
		if len(projects) == 0 {
			if activeOnly {
				return nil, fmt.Errorf("no active projects selected")
			}
			return nil, fmt.Errorf("no projects selected")
		}
		sort.Slice(projects, func(i, j int) bool {
			return projects[i].Name < projects[j].Name
		})
		return projects, nil
	}

	names := splitAndClean(projectName)
	if len(names) == 0 {
		return nil, fmt.Errorf("%w (--project <name> or --all-projects)", ErrProjectSelectRequired)
	}

	index := make(map[string]Project, len(reg.Projects))
	for _, p := range reg.Projects {
		index[strings.ToLower(strings.TrimSpace(p.Name))] = p
	}
	selected := make([]Project, 0, len(names))
	seen := make(map[string]bool)
	for _, n := range names {
		key := strings.ToLower(n)
		if seen[key] {
			continue
		}
		p, ok := index[key]
		if !ok {
			return nil, fmt.Errorf("project %q not found", n)
		}
		if activeOnly && !isProjectActive(p) {
			continue
		}
		selected = append(selected, p)
		seen[key] = true
	}
	if len(selected) == 0 {
		if activeOnly {
			return nil, fmt.Errorf("no active projects selected")
		}
		return nil, fmt.Errorf("no projects selected")
	}
	return selected, nil
}

func loadProjectRegistry(path string) (ProjectRegistry, error) {
	var reg ProjectRegistry
	if err := runstore.ReadJSON(path, &reg); err != nil {
		return ProjectRegistry{}, err
	}
	if reg.SchemaVersion == 0 {
		reg.SchemaVersion = projectSchemaVersion
	}
	reg.Global = normalizeGlobalSettings(reg.Global)
	if reg.Projects == nil {
		reg.Projects = []Project{}
	}
	normalized := make([]Project, 0, len(reg.Projects))
	for _, p := range reg.Projects {
		p.Name = canonicalProjectName(p.Name)
		p.SourceURL = strings.TrimSpace(p.SourceURL)
		p.Profile = strings.TrimSpace(p.Profile)
		p.OutputDir = strings.TrimSpace(p.OutputDir)
		p.CookiesPath = strings.TrimSpace(p.CookiesPath)
		p.CookiesFromBrowser = strings.TrimSpace(p.CookiesFromBrowser)
		p.Order = strings.TrimSpace(p.Order)
		p.Quality = strings.TrimSpace(p.Quality)
		p.JSRuntime = strings.TrimSpace(p.JSRuntime)
		p.DeliveryMode = strings.TrimSpace(p.DeliveryMode)
		p.SubLangs = strings.TrimSpace(p.SubLangs)
		if p.Profile == "" {
			p.Profile = DefaultProfileName
		}
		if p.Active == nil {
			p.Active = boolPtr(true)
		}
		if p.Fragments <= 0 {
			p.Fragments = DefaultFragments
		}
		if p.Order == "" {
			p.Order = DefaultOrder
		}
		if p.Quality == "" {
			p.Quality = DefaultQuality
		}
		if v, ok := parseJSRuntime(p.JSRuntime); ok {
			p.JSRuntime = v
		} else {
			p.JSRuntime = DefaultJSRuntime
		}
		if p.SubLangs == "" {
			p.SubLangs = DefaultSubtitleLanguage
		}
		if p.Name == "" || p.SourceURL == "" {
			continue
		}
		normalized = append(normalized, p)
	}
	reg.Projects = normalized
	return reg, nil
}

func isProjectActive(p Project) bool {
	if p.Active == nil {
		return true
	}
	return *p.Active
}

func boolPtr(v bool) *bool {
	b := v
	return &b
}

func saveProjectRegistry(path string, reg ProjectRegistry) error {
	reg.SchemaVersion = projectSchemaVersion
	if strings.TrimSpace(reg.UpdatedAt) == "" {
		reg.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	reg.Global = normalizeGlobalSettings(reg.Global)
	if reg.Projects == nil {
		reg.Projects = []Project{}
	}
	if err := runstore.Mkdir(filepath.Dir(path)); err != nil {
		return err
	}
	return runstore.WriteJSON(path, reg)
}

func splitAndClean(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := canonicalProjectName(p)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func parseJSRuntime(raw string) (string, bool) {
	runtimes, ok := parseJSRuntimeList(raw)
	if !ok {
		return "", false
	}
	return strings.Join(runtimes, ","), true
}

func parseJSRuntimeList(raw string) ([]string, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return []string{DefaultJSRuntime}, true
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]bool, len(parts))
	for _, p := range parts {
		v := strings.ToLower(strings.TrimSpace(p))
		if v == "" {
			continue
		}
		if v != JSRuntimeAuto && v != JSRuntimeDeno && v != JSRuntimeNode && v != JSRuntimeQuickJS && v != JSRuntimeBun {
			return nil, false
		}
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	if len(out) == 0 {
		return []string{DefaultJSRuntime}, true
	}
	if len(out) > 1 && seen[JSRuntimeAuto] {
		return nil, false
	}
	return out, true
}

func suggestProjectName(sourceURL string) string {
	u := strings.TrimSpace(sourceURL)
	if u == "" {
		return "project"
	}
	if strings.Contains(u, "list=") {
		idx := strings.Index(u, "list=")
		v := u[idx+len("list="):]
		if cut := strings.Index(v, "&"); cut >= 0 {
			v = v[:cut]
		}
		if name := canonicalProjectName(v); name != "" {
			return name
		}
	}
	if idx := strings.Index(u, "/@"); idx >= 0 {
		v := u[idx+2:]
		if cut := strings.Index(v, "/"); cut >= 0 {
			v = v[:cut]
		}
		if name := canonicalProjectName(v); name != "" {
			return name
		}
	}
	base := strings.TrimSpace(filepath.Base(strings.TrimRight(u, "/")))
	if name := canonicalProjectName(base); name != "" {
		return name
	}
	return "project"
}

func canonicalProjectName(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	if s == "" {
		return ""
	}
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		isAlphaNum := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if isAlphaNum {
			b.WriteRune(r)
			prevDash = false
			continue
		}
		if !prevDash {
			b.WriteRune('-')
			prevDash = true
		}
	}
	clean := strings.Trim(b.String(), "-")
	if clean == "" {
		return ""
	}
	return clean
}

func ensureUniqueProjectName(base string, existing []Project, allowExisting bool) string {
	name := canonicalProjectName(base)
	if name == "" {
		return ""
	}
	if allowExisting {
		return name
	}
	set := make(map[string]bool, len(existing))
	for _, p := range existing {
		set[strings.ToLower(strings.TrimSpace(p.Name))] = true
	}
	if !set[name] {
		return name
	}
	for i := 2; i < 10000; i++ {
		candidate := fmt.Sprintf("%s-%d", name, i)
		if !set[candidate] {
			return candidate
		}
	}
	return ""
}

func equalsFoldAndTrim(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}
