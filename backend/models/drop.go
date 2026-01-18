package models

type DroppedFile struct {
	InputPath     string `json:"input_path"`
	SourceRoot    string `json:"source_root"`
	RelativePath  string `json:"relative_path"`
	IsFromDirDrop bool   `json:"is_from_dir_drop"`
}

type ExpandDroppedPathsResult struct {
	Files        []DroppedFile `json:"files"`
	HasDirectory bool          `json:"has_directory"`
}

