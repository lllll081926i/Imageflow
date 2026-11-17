//go:build windows

package upscale

import (
	"net/url"
	"strings"

	"golang.org/x/sys/windows/registry"
)

func platformProxy(target *url.URL) (*url.URL, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if err != nil {
		return nil, nil
	}
	defer key.Close()
	enabled, _, err := key.GetIntegerValue("ProxyEnable")
	if err != nil || enabled == 0 {
		return nil, nil
	}
	server, _, err := key.GetStringValue("ProxyServer")
	if err != nil || server == "" {
		return nil, nil
	}
	if strings.Contains(server, "=") {
		selected := ""
		for _, part := range strings.Split(server, ";") {
			protocol, address, found := strings.Cut(strings.TrimSpace(part), "=")
			if found && protocol == target.Scheme {
				selected = address
				break
			}
			if found && protocol == "socks" {
				selected = "socks5://" + address
			}
		}
		server = selected
	}
	if server == "" {
		return nil, nil
	}
	if !strings.Contains(server, "://") {
		server = "http://" + server
	}
	return parseProxy(server)
}
