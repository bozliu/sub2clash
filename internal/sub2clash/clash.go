package sub2clash

import (
	"bytes"
	"errors"

	"gopkg.in/yaml.v3"
)

func BuildClashYAML(proxies []map[string]any, autoTestURL string) ([]byte, error) {
	if len(proxies) == 0 {
		return nil, errors.New("cannot build clash yaml without proxies")
	}

	names := make([]string, 0, len(proxies))
	for _, proxy := range proxies {
		name, _ := proxy["name"].(string)
		if name == "" {
			return nil, errors.New("proxy name cannot be empty")
		}
		names = append(names, name)
	}

	proxyGroupMembers := append([]string{"AUTO", "DIRECT"}, names...)
	autoMembers := append([]string{}, names...)
	if len(autoMembers) == 0 {
		autoMembers = []string{"DIRECT"}
	}

	document := map[string]any{
		"mixed-port": 7890,
		"socks-port": 7891,
		"allow-lan":  false,
		"mode":       "Rule",
		"log-level":  "info",
		"proxies":    proxies,
		"proxy-groups": []map[string]any{
			{
				"name":    "PROXY",
				"type":    "select",
				"proxies": proxyGroupMembers,
			},
			{
				"name":      "AUTO",
				"type":      "url-test",
				"url":       autoTestURL,
				"interval":  300,
				"tolerance": 50,
				"proxies":   autoMembers,
			},
			{
				"name":    "DIRECT",
				"type":    "select",
				"proxies": []string{"DIRECT"},
			},
		},
		"rules": []string{
			"MATCH,PROXY",
		},
	}

	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(document); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
