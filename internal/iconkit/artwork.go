package iconkit

// artwork.go embeds the source illustration.
//
// It lives here rather than in internal/brand because only the build-time
// generator reads it: the running app draws the pre-rendered sizes next to this
// package, and carrying a 1.5 MB picture it never scales is weight the installer
// would pay for on every platform.
//
// To replace the logo, drop the new artwork in at assets/logo.png and run
// `go run ./cmd/imageflow-marks` then `go run ./cmd/imageflow-icon`.
import (
	_ "embed"
	"image"
)

//go:embed assets/logo.png
var artwork []byte

// Artwork is the source illustration, byte for byte. A generator or a test can
// decode exactly what this build carries.
func Artwork() []byte { return artwork }

// Source is the artwork, decoded.
func Source() (image.Image, error) { return Decode(artwork) }
