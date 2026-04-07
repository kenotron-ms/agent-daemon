package amplifier

import (
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ProjectSettings mirrors the schema of <project>/.amplifier/settings.yaml.
// All fields are pointers/omitempty so absent keys round-trip cleanly.
type ProjectSettings struct {
	Bundle    *BundleSettings          `yaml:"bundle,omitempty"    json:"bundle,omitempty"`
	Config    *ProjectConfigSettings   `yaml:"config,omitempty"    json:"config,omitempty"`
	Modules   *ModulesSettings         `yaml:"modules,omitempty"   json:"modules,omitempty"`
	Overrides map[string]OverrideEntry `yaml:"overrides,omitempty" json:"overrides,omitempty"`
	Sources   *SourcesSettings         `yaml:"sources,omitempty"   json:"sources,omitempty"`
	Routing   *RoutingSettings         `yaml:"routing,omitempty"   json:"routing,omitempty"`
}

type BundleSettings struct {
	Active string            `yaml:"active,omitempty" json:"active,omitempty"`
	App    []string          `yaml:"app,omitempty"    json:"app,omitempty"`
	Added  map[string]string `yaml:"added,omitempty"  json:"added,omitempty"`
}

type ProjectConfigSettings struct {
	Providers     []ProviderEntry      `yaml:"providers,omitempty"     json:"providers,omitempty"`
	Notifications *NotificationsConfig `yaml:"notifications,omitempty" json:"notifications,omitempty"`
}

type ProviderEntry struct {
	Module string                 `yaml:"module"           json:"module"`
	Source string                 `yaml:"source,omitempty" json:"source,omitempty"`
	Config map[string]interface{} `yaml:"config,omitempty" json:"config,omitempty"`
}

type NotificationsConfig struct {
	Desktop *DesktopNotifConfig `yaml:"desktop,omitempty" json:"desktop,omitempty"`
	Push    *PushNotifConfig    `yaml:"push,omitempty"    json:"push,omitempty"`
}

type DesktopNotifConfig struct {
	Enabled            *bool  `yaml:"enabled,omitempty"               json:"enabled,omitempty"`
	ShowDevice         *bool  `yaml:"show_device,omitempty"           json:"show_device,omitempty"`
	ShowProject        *bool  `yaml:"show_project,omitempty"          json:"show_project,omitempty"`
	ShowPreview        *bool  `yaml:"show_preview,omitempty"          json:"show_preview,omitempty"`
	PreviewLength      *int   `yaml:"preview_length,omitempty"        json:"preview_length,omitempty"`
	Subtitle           string `yaml:"subtitle,omitempty"              json:"subtitle,omitempty"`
	SuppressIfFocused  *bool  `yaml:"suppress_if_focused,omitempty"   json:"suppress_if_focused,omitempty"`
	MinIterations      *int   `yaml:"min_iterations,omitempty"        json:"min_iterations,omitempty"`
	ShowIterationCount *bool  `yaml:"show_iteration_count,omitempty"  json:"show_iteration_count,omitempty"`
	Sound              string `yaml:"sound,omitempty"                 json:"sound,omitempty"`
	Debug              *bool  `yaml:"debug,omitempty"                 json:"debug,omitempty"`
}

type PushNotifConfig struct {
	Enabled  *bool    `yaml:"enabled,omitempty"  json:"enabled,omitempty"`
	Server   string   `yaml:"server,omitempty"   json:"server,omitempty"`
	Priority string   `yaml:"priority,omitempty" json:"priority,omitempty"`
	Tags     []string `yaml:"tags,omitempty"     json:"tags,omitempty"`
	Debug    *bool    `yaml:"debug,omitempty"    json:"debug,omitempty"`
}

type ModulesSettings struct {
	Tools []ToolModuleEntry `yaml:"tools,omitempty" json:"tools,omitempty"`
}

type ToolModuleEntry struct {
	Module string      `yaml:"module"           json:"module"`
	Config *ToolConfig `yaml:"config,omitempty" json:"config,omitempty"`
}

type ToolConfig struct {
	AllowedWritePaths []string `yaml:"allowed_write_paths,omitempty" json:"allowed_write_paths,omitempty"`
	AllowedReadPaths  []string `yaml:"allowed_read_paths,omitempty"  json:"allowed_read_paths,omitempty"`
	DeniedWritePaths  []string `yaml:"denied_write_paths,omitempty"  json:"denied_write_paths,omitempty"`
}

type OverrideEntry struct {
	Source string                 `yaml:"source,omitempty" json:"source,omitempty"`
	Config map[string]interface{} `yaml:"config,omitempty" json:"config,omitempty"`
}

type SourcesSettings struct {
	Modules map[string]string `yaml:"modules,omitempty" json:"modules,omitempty"`
	Bundles map[string]string `yaml:"bundles,omitempty" json:"bundles,omitempty"`
}

type RoutingSettings struct {
	Matrix    string            `yaml:"matrix,omitempty"    json:"matrix,omitempty"`
	Overrides map[string]string `yaml:"overrides,omitempty" json:"overrides,omitempty"`
}

// ReadProjectSettings reads <projectPath>/.amplifier/settings.yaml.
// Returns empty ProjectSettings (no error) if the file does not exist.
func ReadProjectSettings(projectPath string) (ProjectSettings, error) {
	settingsPath := filepath.Join(projectPath, ".amplifier", "settings.yaml")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ProjectSettings{}, nil
		}
		return ProjectSettings{}, err
	}
	var s ProjectSettings
	if err := yaml.Unmarshal(data, &s); err != nil {
		return ProjectSettings{}, err
	}
	return s, nil
}

// WriteProjectSettings writes settings to <projectPath>/.amplifier/settings.yaml,
// creating the .amplifier directory if needed.
func WriteProjectSettings(projectPath string, s ProjectSettings) error {
	amplifierDir := filepath.Join(projectPath, ".amplifier")
	if err := os.MkdirAll(amplifierDir, 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(amplifierDir, "settings.yaml"), data, 0644)
}
