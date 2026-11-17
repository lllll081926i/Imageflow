//go:build !windows

package upscale

import "net/url"

func platformProxy(*url.URL) (*url.URL, error) { return nil, nil }
