package models

type AppSettings struct {
	MaxConcurrency          int      `json:"max_concurrency"`
	OutputPrefix            string   `json:"output_prefix"`
	OutputTemplate          string   `json:"output_template"`
	PreserveFolderStructure bool     `json:"preserve_folder_structure"`
	ConflictStrategy        string   `json:"conflict_strategy"`
	DefaultOutputDir        string   `json:"default_output_dir"`
	RecentInputDirs         []string `json:"recent_input_dirs"`
	RecentOutputDirs        []string `json:"recent_output_dirs"`
}

func DefaultAppSettings() AppSettings {
	return AppSettings{
		MaxConcurrency:          8,
		OutputPrefix:            "IF",
		OutputTemplate:          "{prefix}{basename}",
		PreserveFolderStructure: true,
		ConflictStrategy:        "rename",
		DefaultOutputDir:        "",
		RecentInputDirs:         []string{},
		RecentOutputDirs:        []string{},
	}
}

type RecentPathsUpdateRequest struct {
	InputDir  string `json:"input_dir"`
	OutputDir string `json:"output_dir"`
}
