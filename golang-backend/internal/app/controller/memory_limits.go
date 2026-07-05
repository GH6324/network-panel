package controller

import (
	"fmt"
	"strings"
)

var (
	nodeOpLogTextMax       = getBufMax("NODE_OP_LOG_TEXT_MAX_BYTES", 64*1024)
	nodeDiagTextMax        = getBufMax("NODE_DIAG_TEXT_MAX_BYTES", 256*1024)
	gostConfigReadMaxBytes = getBufMax("GOST_CONFIG_READ_MAX_BYTES", 64*1024)
)

func truncateTailString(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	marker := fmt.Sprintf("[truncated: kept last %d of %d bytes]\n", max, len(s))
	if len(marker) >= max {
		return marker
	}
	return marker + s[len(s)-(max-len(marker)):]
}

func truncateStringPtr(p *string, max int) *string {
	if p == nil {
		return nil
	}
	s := truncateTailString(*p, max)
	return &s
}

func appendBoundedText(base, chunk string, max int) string {
	if chunk == "" {
		return truncateTailString(base, max)
	}
	if base != "" && !strings.HasSuffix(base, "\n") {
		base += "\n"
	}
	return truncateTailString(base+chunk, max)
}

func buildGostConfigReadScript(maxBytes int) string {
	if maxBytes <= 0 {
		maxBytes = 64 * 1024
	}
	return fmt.Sprintf(`#!/bin/sh
set +e
limit=%d
for p in /etc/gost/gost.json /usr/local/gost/gost.json ./gost.json; do
  if [ -f "$p" ]; then
    echo "PATH:$p"
    size=$(wc -c < "$p" 2>/dev/null | tr -d ' ')
    head -c %d "$p"
    if [ -n "$size" ] && [ "$size" -gt "$limit" ] 2>/dev/null; then
      printf '\n[TRUNCATED: gost config kept first %d of %%s bytes]\n' "$size"
    fi
    exit 0
  fi
done
echo 'PATH:NOT_FOUND'
exit 0
`, maxBytes, maxBytes, maxBytes)
}
