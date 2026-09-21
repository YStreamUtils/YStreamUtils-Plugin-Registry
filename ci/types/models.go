package types

type Permission string

type SourceConfig struct {
	Repository string `json:"repository"`
	Owner      string `json:"owner"`
}

type DocumentationConfig struct {
	Description string `json:"description"`
}

type PluginManifest struct {
	Name          string              `json:"name"`
	Version       string              `json:"version"`
	EntryPoint    string              `json:"entryPoint"`
	Permissions   []Permission        `json:"permissions"`
	Authors       []string            `json:"authors"`
	Source        SourceConfig        `json:"source"`
	Documentation DocumentationConfig `json:"documentation"`
}

type RegistryDistribution struct {
	Name    string           `json:"name"`
	Source  SourceConfig     `json:"source"`
	Plugins []PluginManifest `json:"plugins"`
}
