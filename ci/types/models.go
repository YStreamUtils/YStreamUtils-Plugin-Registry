package types

type Permission string

type SourceConfig struct {
	Repository string `toml:"repository"`
	Owner      string `toml:"owner"`
}

type DocumentationConfig struct {
	Description string `toml:"description"`
}

type PluginManifest struct {
	Name          string              `toml:"name"`
	Version       string              `toml:"version"`
	EntryPoint    string              `toml:"entry_point"`
	Permissions   []Permission        `toml:"permissions"`
	Authors       []string            `toml:"authors"`
	Source        SourceConfig        `toml:"source"`
	Documentation DocumentationConfig `toml:"documentation"`
}

type RegistryDistribution struct {
	Name    string                    `toml:"name"`
	Source  SourceConfig              `toml:"source"`
	Plugins map[string]PluginManifest `toml:"plugins"`
}
