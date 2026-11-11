package ui

// project.go re-exports the project's public identity from the brand package, so
// the UI reads one set of constants while the icon generator and the packaging
// config read the same values.
import "github.com/lllll081926i/imageflow-go/internal/brand"

const (
	// projectRepoURL is the canonical GitHub repository the About page opens.
	projectRepoURL = brand.RepoURL
	// projectRepoLabel is how that repository reads in the About row.
	projectRepoLabel = brand.RepoLabel
)
