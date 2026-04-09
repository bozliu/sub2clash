package sub2clash

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

func ConvertRawSubscription(raw []byte, headers map[string][]string, autoTestURL string) (*ConvertResult, error) {
	info := headerValue(headers, "subscription-userinfo")
	normalizedText, bannerInfo := normalizeBody(raw)
	if info == "" {
		info = bannerInfo
	}

	proxies, warnings, err := extractProxies(normalizedText)
	if err != nil {
		return nil, err
	}
	yamlBytes, err := BuildClashYAML(proxies, autoTestURL)
	if err != nil {
		return nil, err
	}

	return &ConvertResult{
		YAML:             yamlBytes,
		Warnings:         warnings,
		SubscriptionInfo: info,
	}, nil
}

func normalizeBody(raw []byte) (string, string) {
	text := strings.TrimSpace(strings.ReplaceAll(string(raw), "\r\n", "\n"))
	if looksLikeClashYAML(text) {
		return text, ""
	}
	if decoded, ok := decodeMaybeBase64(text); ok {
		text = decoded
	}
	text = strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	return text, parseStatusBanner(text)
}

func extractProxies(text string) ([]map[string]any, []string, error) {
	if looksLikeClashYAML(text) {
		return parseClashYAML(text)
	}
	tokens := collectURITokens(text)
	if len(tokens) == 0 {
		return nil, nil, fmt.Errorf("no supported subscription entries found")
	}

	var proxies []map[string]any
	var warnings []string
	for _, token := range tokens {
		proxy, err := parseURIProxy(token)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", summarizeToken(token), err))
			continue
		}
		proxies = append(proxies, proxy)
	}
	if len(proxies) == 0 {
		return nil, warnings, fmt.Errorf("no supported proxies were parsed")
	}
	ensureUniqueProxyNames(proxies)
	return proxies, warnings, nil
}

func parseClashYAML(text string) ([]map[string]any, []string, error) {
	var payload struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	if err := yaml.Unmarshal([]byte(text), &payload); err != nil {
		return nil, nil, fmt.Errorf("invalid clash yaml: %w", err)
	}
	if len(payload.Proxies) == 0 {
		return nil, nil, fmt.Errorf("clash yaml did not contain proxies")
	}
	for _, proxy := range payload.Proxies {
		if _, ok := proxy["name"].(string); !ok {
			return nil, nil, fmt.Errorf("proxy entry missing name")
		}
	}
	ensureUniqueProxyNames(payload.Proxies)
	return payload.Proxies, nil, nil
}

func collectURITokens(text string) []string {
	var tokens []string
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return r == '\n' || r == '\r' || r == '\t' || r == ' '
	})
	for _, field := range fields {
		field = strings.TrimSpace(field)
		field = strings.Trim(field, "\"'")
		if !strings.Contains(field, "://") {
			continue
		}
		tokens = append(tokens, field)
	}
	return slices.Compact(tokens)
}

func parseURIProxy(token string) (map[string]any, error) {
	switch {
	case strings.HasPrefix(token, "ss://"):
		return parseSS(token)
	case strings.HasPrefix(token, "ssr://"):
		return parseSSR(token)
	case strings.HasPrefix(token, "vmess://"):
		return parseVMess(token)
	case strings.HasPrefix(token, "vless://"):
		return parseVLESS(token)
	case strings.HasPrefix(token, "trojan://"):
		return parseTrojan(token)
	case strings.HasPrefix(token, "hy2://"), strings.HasPrefix(token, "hysteria2://"):
		return parseHysteria2(token)
	case strings.HasPrefix(token, "tuic://"):
		return parseTUIC(token)
	default:
		return nil, fmt.Errorf("unsupported scheme")
	}
}

func parseSS(raw string) (map[string]any, error) {
	body := stripScheme(raw)
	name := decodeFragment(raw)
	if idx := strings.Index(body, "#"); idx >= 0 {
		body = body[:idx]
	}
	if idx := strings.Index(body, "?"); idx >= 0 {
		body = body[:idx]
	}

	userInfo := body
	hostPort := ""
	if idx := strings.LastIndex(body, "@"); idx >= 0 {
		userInfo = body[:idx]
		hostPort = body[idx+1:]
	}

	decodedInfo, _ := decodeBase64Loose(userInfo)
	if hostPort == "" && strings.Contains(decodedInfo, "@") {
		parts := strings.SplitN(decodedInfo, "@", 2)
		decodedInfo, hostPort = parts[0], parts[1]
	} else if hostPort == "" {
		decodedInfo = userInfo
	}
	if decodedInfo == "" {
		decodedInfo = userInfo
	}

	method, password, ok := cutFirst(decodedInfo, ":")
	if !ok {
		return nil, fmt.Errorf("invalid ss userinfo")
	}
	host, port, err := splitHostPort(hostPort)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"name":     firstNonEmpty(name, hostPort),
		"type":     "ss",
		"server":   host,
		"port":     port,
		"cipher":   method,
		"password": password,
		"udp":      true,
	}, nil
}

func parseSSR(raw string) (map[string]any, error) {
	encoded := stripScheme(raw)
	decoded, ok := decodeBase64Loose(encoded)
	if !ok {
		return nil, fmt.Errorf("invalid ssr payload")
	}
	main, paramsText, _ := strings.Cut(decoded, "/?")
	parts := strings.Split(main, ":")
	if len(parts) < 6 {
		return nil, fmt.Errorf("invalid ssr format")
	}
	port, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, err
	}
	password, _ := decodeBase64Loose(parts[5])
	params, _ := url.ParseQuery(paramsText)
	name, _ := decodeBase64Loose(params.Get("remarks"))

	proxy := map[string]any{
		"name":     firstNonEmpty(name, parts[0]),
		"type":     "ssr",
		"server":   parts[0],
		"port":     port,
		"cipher":   parts[3],
		"password": password,
		"protocol": parts[2],
		"obfs":     parts[4],
		"udp":      true,
	}
	if value, _ := decodeBase64Loose(params.Get("obfsparam")); value != "" {
		proxy["obfs-param"] = value
	}
	if value, _ := decodeBase64Loose(params.Get("protoparam")); value != "" {
		proxy["protocol-param"] = value
	}
	return proxy, nil
}

func parseVMess(raw string) (map[string]any, error) {
	encoded := stripScheme(raw)
	decoded, ok := decodeBase64Loose(encoded)
	if !ok {
		return nil, fmt.Errorf("invalid vmess payload")
	}
	var payload map[string]string
	if err := json.Unmarshal([]byte(decoded), &payload); err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(firstNonEmpty(payload["port"], payload["Port"]))
	if err != nil {
		return nil, fmt.Errorf("invalid vmess port")
	}
	name := firstNonEmpty(payload["ps"], payload["remark"], payload["add"])
	server := firstNonEmpty(payload["add"], payload["host"], payload["server"])
	proxy := map[string]any{
		"name":    name,
		"type":    "vmess",
		"server":  server,
		"port":    port,
		"uuid":    firstNonEmpty(payload["id"], payload["uuid"]),
		"alterId": intValue(firstNonEmpty(payload["aid"], payload["alterId"]), 0),
		"cipher":  firstNonEmpty(payload["scy"], "auto"),
		"udp":     true,
		"network": normalizeNetwork(payload["net"]),
	}
	applyTLSOptions(proxy, firstNonEmpty(payload["tls"], payload["security"]), firstNonEmpty(payload["sni"], payload["servername"]), payload["allowInsecure"])
	applyTransportOptions(proxy, proxy["network"].(string), payload["path"], firstNonEmpty(payload["host"], payload["Host"]), payload["serviceName"])
	return proxy, nil
}

func parseVLESS(raw string) (map[string]any, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return nil, fmt.Errorf("invalid vless port")
	}
	query := u.Query()
	proxy := map[string]any{
		"name":    firstNonEmpty(decodeFragment(raw), u.Host),
		"type":    "vless",
		"server":  u.Hostname(),
		"port":    port,
		"uuid":    u.User.Username(),
		"udp":     true,
		"network": normalizeNetwork(query.Get("type")),
	}
	if flow := query.Get("flow"); flow != "" {
		proxy["flow"] = flow
	}
	security := firstNonEmpty(query.Get("security"), query.Get("tls"))
	applyTLSOptions(proxy, security, firstNonEmpty(query.Get("sni"), query.Get("peer"), query.Get("servername")), query.Get("allowInsecure"))
	if strings.EqualFold(security, "reality") {
		ropts := map[string]any{}
		if key := firstNonEmpty(query.Get("pbk"), query.Get("public-key")); key != "" {
			ropts["public-key"] = key
		}
		if sid := firstNonEmpty(query.Get("sid"), query.Get("short-id")); sid != "" {
			ropts["short-id"] = sid
		}
		if len(ropts) > 0 {
			proxy["reality-opts"] = ropts
		}
		if fp := query.Get("fp"); fp != "" {
			proxy["client-fingerprint"] = fp
		}
	}
	applyTransportOptions(proxy, proxy["network"].(string), query.Get("path"), query.Get("host"), query.Get("serviceName"))
	return proxy, nil
}

func parseTrojan(raw string) (map[string]any, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return nil, fmt.Errorf("invalid trojan port")
	}
	query := u.Query()
	proxy := map[string]any{
		"name":     firstNonEmpty(decodeFragment(raw), u.Host),
		"type":     "trojan",
		"server":   u.Hostname(),
		"port":     port,
		"password": firstNonEmpty(u.User.Username(), query.Get("password")),
		"udp":      true,
		"network":  normalizeNetwork(query.Get("type")),
	}
	applyTLSOptions(proxy, firstNonEmpty(query.Get("security"), "tls"), firstNonEmpty(query.Get("sni"), query.Get("peer"), query.Get("servername")), query.Get("allowInsecure"))
	applyTransportOptions(proxy, proxy["network"].(string), query.Get("path"), query.Get("host"), query.Get("serviceName"))
	return proxy, nil
}

func parseHysteria2(raw string) (map[string]any, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return nil, fmt.Errorf("invalid hysteria2 port")
	}
	query := u.Query()
	password := firstNonEmpty(u.User.Username(), query.Get("auth"), query.Get("password"))
	proxy := map[string]any{
		"name":     firstNonEmpty(decodeFragment(raw), u.Host),
		"type":     "hysteria2",
		"server":   u.Hostname(),
		"port":     port,
		"password": password,
		"udp":      true,
	}
	if sni := firstNonEmpty(query.Get("sni"), query.Get("peer")); sni != "" {
		proxy["sni"] = sni
	}
	if query.Get("insecure") == "1" || strings.EqualFold(query.Get("insecure"), "true") {
		proxy["skip-cert-verify"] = true
	}
	if obfs := query.Get("obfs"); obfs != "" {
		proxy["obfs"] = obfs
	}
	if value := firstNonEmpty(query.Get("obfs-password"), query.Get("obfs-password1")); value != "" {
		proxy["obfs-password"] = value
	}
	if value := firstNonEmpty(query.Get("upmbps"), query.Get("up")); value != "" {
		proxy["up"] = intValue(value, 0)
	}
	if value := firstNonEmpty(query.Get("downmbps"), query.Get("down")); value != "" {
		proxy["down"] = intValue(value, 0)
	}
	return proxy, nil
}

func parseTUIC(raw string) (map[string]any, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return nil, fmt.Errorf("invalid tuic port")
	}
	password, _ := u.User.Password()
	query := u.Query()
	proxy := map[string]any{
		"name":                  firstNonEmpty(decodeFragment(raw), u.Host),
		"type":                  "tuic",
		"server":                u.Hostname(),
		"port":                  port,
		"uuid":                  u.User.Username(),
		"password":              password,
		"udp":                   true,
		"congestion-controller": firstNonEmpty(query.Get("congestion_control"), "bbr"),
	}
	if sni := firstNonEmpty(query.Get("sni"), query.Get("peer")); sni != "" {
		proxy["sni"] = sni
	}
	if query.Get("insecure") == "1" || strings.EqualFold(query.Get("insecure"), "true") {
		proxy["skip-cert-verify"] = true
	}
	if value := query["alpn"]; len(value) > 0 {
		proxy["alpn"] = value
	}
	return proxy, nil
}

func ensureUniqueProxyNames(proxies []map[string]any) {
	counts := map[string]int{}
	for _, proxy := range proxies {
		name, _ := proxy["name"].(string)
		name = strings.TrimSpace(name)
		if name == "" {
			name = "unnamed"
		}
		counts[name]++
		if counts[name] > 1 {
			name = fmt.Sprintf("%s %d", name, counts[name])
		}
		proxy["name"] = name
	}
}

func parseStatusBanner(text string) string {
	line := ""
	for _, candidate := range strings.Split(text, "\n") {
		candidate = strings.TrimSpace(candidate)
		if strings.HasPrefix(candidate, "STATUS=") {
			line = strings.TrimPrefix(candidate, "STATUS=")
			break
		}
	}
	if line == "" {
		return ""
	}

	re := regexp.MustCompile(`↑:([0-9.]+[KMGTP]?B),↓:([0-9.]+[KMGTP]?B),TOT:([0-9.]+[KMGTP]?B).*?Expires:([0-9]{4}-[0-9]{2}-[0-9]{2})`)
	match := re.FindStringSubmatch(line)
	if len(match) != 5 {
		return ""
	}
	expire, err := time.Parse("2006-01-02", match[4])
	if err != nil {
		return ""
	}
	return fmt.Sprintf(
		"upload=%d; download=%d; total=%d; expire=%d",
		humanBytes(match[1]),
		humanBytes(match[2]),
		humanBytes(match[3]),
		expire.Add(23*time.Hour+59*time.Minute+59*time.Second).Unix(),
	)
}

func decodeMaybeBase64(raw string) (string, bool) {
	squashed := strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', '\t', ' ':
			return -1
		default:
			return r
		}
	}, raw)
	for _, encoding := range []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	} {
		decoded, err := encoding.DecodeString(squashed)
		if err != nil {
			continue
		}
		text := strings.TrimSpace(string(decoded))
		if text == "" {
			continue
		}
		if strings.Contains(text, "://") || strings.Contains(text, "STATUS=") || looksLikeClashYAML(text) {
			return text, true
		}
	}
	return "", false
}

func decodeBase64Loose(raw string) (string, bool) {
	squashed := strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', '\t', ' ':
			return -1
		default:
			return r
		}
	}, raw)
	for _, encoding := range []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	} {
		decoded, err := encoding.DecodeString(squashed)
		if err == nil {
			return strings.TrimSpace(string(decoded)), true
		}
	}
	return "", false
}

func looksLikeClashYAML(text string) bool {
	trimmed := strings.TrimSpace(text)
	return strings.HasPrefix(trimmed, "proxies:") || strings.Contains(trimmed, "\nproxies:")
}

func decodeFragment(raw string) string {
	fragment := ""
	if idx := strings.Index(raw, "#"); idx >= 0 {
		fragment = raw[idx+1:]
	}
	if decoded, err := url.QueryUnescape(fragment); err == nil && decoded != "" {
		return decoded
	}
	return fragment
}

func stripScheme(raw string) string {
	if idx := strings.Index(raw, "://"); idx >= 0 {
		return raw[idx+3:]
	}
	return raw
}

func cutFirst(value, sep string) (string, string, bool) {
	idx := strings.Index(value, sep)
	if idx < 0 {
		return "", "", false
	}
	return value[:idx], value[idx+len(sep):], true
}

func splitHostPort(value string) (string, int, error) {
	host, portText, err := net.SplitHostPort(value)
	if err != nil {
		if strings.Count(value, ":") == 1 {
			parts := strings.SplitN(value, ":", 2)
			port, err := strconv.Atoi(parts[1])
			if err != nil {
				return "", 0, err
			}
			return parts[0], port, nil
		}
		return "", 0, err
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return "", 0, err
	}
	return host, port, nil
}

func normalizeNetwork(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "", "tcp":
		return "tcp"
	case "ws", "websocket":
		return "ws"
	case "grpc":
		return "grpc"
	case "http", "h2", "http2":
		return "http"
	default:
		return value
	}
}

func applyTLSOptions(proxy map[string]any, security, sni, allowInsecure string) {
	security = strings.TrimSpace(strings.ToLower(security))
	switch security {
	case "tls", "xtls", "reality", "true", "1":
		proxy["tls"] = true
	}
	if sni != "" {
		proxy["servername"] = sni
	}
	if allowInsecure == "1" || strings.EqualFold(allowInsecure, "true") {
		proxy["skip-cert-verify"] = true
	}
}

func applyTransportOptions(proxy map[string]any, network, path, host, serviceName string) {
	switch network {
	case "ws":
		ws := map[string]any{}
		if path != "" {
			ws["path"] = path
		}
		if host != "" {
			ws["headers"] = map[string]any{"Host": host}
		}
		if len(ws) > 0 {
			proxy["ws-opts"] = ws
		}
	case "grpc":
		if serviceName != "" {
			proxy["grpc-opts"] = map[string]any{"grpc-service-name": serviceName}
		}
	case "http":
		httpOpts := map[string]any{}
		if path != "" {
			httpOpts["path"] = []string{path}
		}
		if host != "" {
			httpOpts["headers"] = map[string]any{"Host": []string{host}}
		}
		if len(httpOpts) > 0 {
			proxy["http-opts"] = httpOpts
		}
	}
}

func intValue(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func humanBytes(raw string) int64 {
	raw = strings.TrimSpace(strings.ToUpper(raw))
	re := regexp.MustCompile(`^([0-9.]+)([KMGTP]?B)$`)
	match := re.FindStringSubmatch(raw)
	if len(match) != 3 {
		return 0
	}
	value, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0
	}
	scale := map[string]float64{
		"KB": 1_000,
		"MB": 1_000_000,
		"GB": 1_000_000_000,
		"TB": 1_000_000_000_000,
		"PB": 1_000_000_000_000_000,
		"B":  1,
	}
	return int64(value * scale[match[2]])
}

func summarizeToken(token string) string {
	if len(token) <= 32 {
		return token
	}
	return token[:32] + "..."
}

func headerValue(headers map[string][]string, key string) string {
	for headerKey, values := range headers {
		if strings.EqualFold(headerKey, key) && len(values) > 0 {
			return values[0]
		}
	}
	return ""
}

func prettyYAML(value any) string {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	_ = enc.Encode(value)
	return buf.String()
}
