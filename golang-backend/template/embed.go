package template

import _ "embed"

//go:embed clash.yaml
var clashTemplate string

//go:embed surge.surgeconfig
var surgeTemplate string

//go:embed shadowrocket.conf
var shadowrocketTemplate string

func Load(name string) (string, bool) {
	switch name {
	case "clash.yaml":
		return clashTemplate, clashTemplate != ""
	case "surge.surgeconfig":
		return surgeTemplate, surgeTemplate != ""
	case "shadowrocket.conf":
		return shadowrocketTemplate, shadowrocketTemplate != ""
	default:
		return "", false
	}
}
