package controller

import (
	"fmt"
	"strings"

	tpl "network-panel/golang-backend/template"
)

// The upstream profile is pinned in template/shadowrocket.conf; never inherit
// its update-url, which would replace this per-user profile with an empty one.
func buildShadowrocketRuleConfig(items []subProxy) (string, error) {
	base, ok := tpl.Load("shadowrocket.conf")
	if !ok || !strings.Contains(base, "[Proxy]\n") || !strings.Contains(base, "\n[Proxy Group]") || !strings.Contains(base, "\n[Rule]") {
		return "", fmt.Errorf("rule template is missing required sections")
	}
	lines := make([]string, 0, len(items))
	for _, it := range items {
		if it.Name == "" || it.Server == "" || it.Port <= 0 || it.Port > 65535 ||
			strings.ContainsAny(it.Name, "\r\n,=") || strings.ContainsAny(it.Server, "\r\n,") {
			return "", fmt.Errorf("invalid node name or server")
		}
		params := it.Params
		if params == nil {
			params = map[string]interface{}{}
		}
		name := it.Name
		server := it.Server
		var line string
		switch normalizeProtocol(it.Type) {
		case "ss":
			cipher, pass := it.Cipher, it.Password
			if cipher == "" {
				cipher = paramString(params, "cipher")
			}
			if pass == "" {
				pass = paramString(params, "password")
			}
			if cipher != "" && pass != "" && !strings.ContainsAny(cipher+pass, "\r\n,") {
				line = fmt.Sprintf("%s = ss, %s, %d, method=%s, password=%s", name, server, it.Port, cipher, pass)
			}
		case "anytls":
			pass := it.Password
			if pass == "" {
				pass = paramString(params, "password")
			}
			pass = stripUserPrefix(pass)
			if pass != "" && !strings.ContainsAny(pass, "\r\n,") {
				line = fmt.Sprintf("%s = anytls, %s, %d, password=%s, skip-cert-verify=%t", name, server, it.Port, pass, anyTLSSkipCertVerify(params))
				if sni := effectiveAnyTLSSNI(params); sni != "" {
					if strings.ContainsAny(sni, "\r\n,") {
						return "", fmt.Errorf("invalid SNI")
					}
					line += ", sni=" + sni
				}
			}
		case "socks5", "http", "https":
			line = fmt.Sprintf("%s = %s, %s, %d", name, normalizeProtocol(it.Type), server, it.Port)
		}
		if line == "" {
			return "", fmt.Errorf("unsupported or incomplete node")
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return "", fmt.Errorf("no supported nodes")
	}
	base = strings.Replace(base,
		"update-url = https://raw.githubusercontent.com/LingJingMaster/Shadowrocket-Rules/refs/heads/main/Shadowrocket.conf",
		"# update-url disabled: personalized nodes are served by this subscription URL", 1)
	return strings.Replace(base, "[Proxy]\n", "[Proxy]\n"+strings.Join(lines, "\n")+"\n", 1), nil
}
