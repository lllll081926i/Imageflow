package models

type AppSettings struct {
	MaxConcurrency          int    `json:"max_concurrency"`
	OutputPrefix            string `json:"output_prefix"`
	OutputTemplate          string `json:"output_template"`
	PreserveFolderStructure bool   `json:"preserve_folder_structure"`
	ConflictStrategy        string `json:"conflict_strategy"`
}

func DefaultAppSettings() AppSettings {
	return AppSettings{
		MaxConcurrency:          8,
		OutputPrefix:            "IF",
		OutputTemplate:          "{prefix}{basename}",
		PreserveFolderStructure: true,
		ConflictStrategy:        "rename",
	}
}
