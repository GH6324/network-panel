package controller

import (
	"strings"
	"testing"
)

func TestBuildShadowrocketRuleConfigEmbedsNodesAndRules(t *testing.T) {
	items := []subProxy{
		{Name: "🇺🇸 US-A", Type: "ss", Server: "us.example.com", Port: 443, Cipher: "aes-256-gcm", Password: "example-pass"},
		{Name: "🇭🇰 HK-B", Type: "anytls", Server: "hk.example.com", Port: 8443, Password: "u1:example-pass", Params: map[string]interface{}{"sni": "example.com", "skip-cert-verify": false}},
	}
	got, err := buildShadowrocketRuleConfig(items)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"[General]", "[Proxy]\n🇺🇸 US-A = ss, us.example.com, 443, method=aes-256-gcm, password=example-pass",
		"🇭🇰 HK-B = anytls, hk.example.com, 8443, password=example-pass", "sni=example.com",
		"[Proxy Group]", "policy-regex-filter=", "RULE-SET,https://raw.githubusercontent.com/LingJingMaster/Shadowrocket-Rules/", "GEOIP,CN,🔒 国内服务", "FINAL,🐟 漏网之鱼", "[Host]",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(got, "update-url = https://raw.githubusercontent.com/LingJingMaster/Shadowrocket-Rules/") {
		t.Fatal("upstream update-url would replace personalized profile with a node-free template")
	}
	if strings.Contains(got, "u1:example-pass") {
		t.Fatal("AnyTLS user prefix leaked")
	}
	if strings.Index(got, "[Proxy]\n") > strings.Index(got, "[Proxy Group]") {
		t.Fatal("proxy section after group")
	}
}

func TestBuildShadowrocketRuleConfigRejectsEmptyOrUnsupportedNodes(t *testing.T) {
	for _, items := range [][]subProxy{nil, {{Name: "opaque", Type: "unknown", Server: "example.com", Port: 443}}} {
		if _, err := buildShadowrocketRuleConfig(items); err == nil {
			t.Errorf("accepted unusable nodes: %#v", items)
		}
	}
}

func TestBuildShadowrocketRuleConfigRejectsConfigInjection(t *testing.T) {
	items := []subProxy{{Name: "ok\n[Rule]\nFINAL,DIRECT", Type: "ss", Server: "example.com", Port: 443, Cipher: "aes-256-gcm", Password: "secret"}}
	if _, err := buildShadowrocketRuleConfig(items); err == nil {
		t.Fatal("accepted newline in profile field")
	}
}

func TestBuildShadowrocketRuleConfigRejectsPartialNodeList(t *testing.T) {
	items := []subProxy{
		{Name: "good", Type: "ss", Server: "example.com", Port: 443, Cipher: "aes-256-gcm", Password: "secret"},
		{Name: "missing", Type: "vmess", Server: "example.net", Port: 443},
	}
	if _, err := buildShadowrocketRuleConfig(items); err == nil {
		t.Fatal("silently dropped a node from a seemingly complete profile")
	}
}
