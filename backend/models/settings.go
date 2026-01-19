package models

type AppSettings struct {
	MaxConcurrency int `json:"max_concurrency"`
}

func DefaultAppSettings() AppSettings {
	return AppSettings{
		MaxConcurrency: 8,
	}
}

