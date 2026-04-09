package sub2clash

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestParseSupportedProtocols(t *testing.T) {
	ssPayload := base64.StdEncoding.EncodeToString([]byte("aes-128-gcm:secret"))
	ssrPayload := base64.RawURLEncoding.EncodeToString([]byte("example.com:443:auth_sha1_v4:aes-256-cfb:plain:" + base64.RawURLEncoding.EncodeToString([]byte("secret")) + "/?remarks=" + base64.RawURLEncoding.EncodeToString([]byte("SSR Node"))))
	vmessPayload := base64.StdEncoding.EncodeToString([]byte(`{"v":"2","ps":"VMess Node","add":"vmess.example.com","port":"443","id":"11111111-1111-1111-1111-111111111111","aid":"0","scy":"auto","net":"ws","host":"vmess.example.com","path":"/ws","tls":"tls","sni":"vmess.example.com"}`))

	cases := []struct {
		name   string
		uri    string
		expect string
	}{
		{"ss", "ss://" + ssPayload + "@ss.example.com:8388#SS%20Node", "ss"},
		{"ssr", "ssr://" + ssrPayload, "ssr"},
		{"vmess", "vmess://" + vmessPayload, "vmess"},
		{"vless", "vless://11111111-1111-1111-1111-111111111111@vless.example.com:443?security=tls&type=ws&host=vless.example.com&path=%2Fws&sni=vless.example.com#VLESS%20Node", "vless"},
		{"trojan", "trojan://secret@trojan.example.com:443?security=tls&sni=trojan.example.com#Trojan%20Node", "trojan"},
		{"hysteria2", "hy2://secret@hy2.example.com:8443?sni=hy2.example.com#HY2%20Node", "hysteria2"},
		{"tuic", "tuic://11111111-1111-1111-1111-111111111111:secret@tuic.example.com:443?sni=tuic.example.com#TUIC%20Node", "tuic"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			proxy, err := parseURIProxy(tc.uri)
			if err != nil {
				t.Fatalf("parseURIProxy() error = %v", err)
			}
			if got := proxy["type"]; got != tc.expect {
				t.Fatalf("proxy type = %v, want %s", got, tc.expect)
			}
			if _, ok := proxy["name"].(string); !ok {
				t.Fatalf("proxy missing name: %#v", proxy)
			}
		})
	}
}

func TestConvertRawSubscriptionParsesStatusBannerAndBase64Bundle(t *testing.T) {
	text := strings.Join([]string{
		"STATUS=🚀↑:1GB,↓:2GB,TOT:10GB💒Expires:2026-06-26",
		"ss://" + base64.StdEncoding.EncodeToString([]byte("aes-128-gcm:secret")) + "@ss.example.com:8388#SS%20Node",
	}, "\n")
	raw := []byte(base64.StdEncoding.EncodeToString([]byte(text)))

	result, err := ConvertRawSubscription(raw, nil, "http://example.com/generate_204")
	if err != nil {
		t.Fatalf("ConvertRawSubscription() error = %v", err)
	}
	if !strings.Contains(string(result.YAML), "proxy-groups:") {
		t.Fatalf("expected proxy-groups in output yaml, got:\n%s", string(result.YAML))
	}
	if !strings.Contains(result.SubscriptionInfo, "total=") {
		t.Fatalf("expected subscription info to be derived, got %q", result.SubscriptionInfo)
	}
}

func TestBuildClashYAMLQuotesNamesWithColons(t *testing.T) {
	yamlBytes, err := BuildClashYAML([]map[string]any{
		{
			"name":     "node:1",
			"type":     "ss",
			"server":   "example.com",
			"port":     8388,
			"cipher":   "aes-128-gcm",
			"password": "secret",
			"udp":      true,
		},
	}, "http://example.com/generate_204")
	if err != nil {
		t.Fatalf("BuildClashYAML() error = %v", err)
	}
	output := string(yamlBytes)
	var roundTrip struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	if err := yaml.Unmarshal(yamlBytes, &roundTrip); err != nil {
		t.Fatalf("expected generated yaml to parse cleanly: %v\n%s", err, output)
	}
	if got := roundTrip.Proxies[0]["name"]; got != "node:1" {
		t.Fatalf("expected name to round-trip safely, got %v", got)
	}
}

func BenchmarkConvertRawSubscription(b *testing.B) {
	text := fmt.Sprintf("ss://%s@ss.example.com:8388#SS%%20Node", base64.StdEncoding.EncodeToString([]byte("aes-128-gcm:secret")))
	for i := 0; i < b.N; i++ {
		_, _ = ConvertRawSubscription([]byte(text), nil, "http://example.com/generate_204")
	}
}
