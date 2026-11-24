// Package user exposes read-only user-facing endpoints (lookup, me) and the
// masking rules shared by every API response that shows someone ELSE's
// identity. The server never returns another user's raw name/phone —
// masking happens here, not in clients (m3-api-support plan §3).
package user

import "strings"

// MaskPhone keeps the first 3 and last 3 digits: "0912345678" -> "091****678".
// Inputs shorter than 7 runes are fully masked — never partially revealed.
func MaskPhone(phone string) string {
	r := []rune(phone)
	if len(r) < 7 {
		return strings.Repeat("*", len(r))
	}
	return string(r[:3]) + strings.Repeat("*", len(r)-6) + string(r[len(r)-3:])
}

// MaskName keeps the family name (first word) intact and reduces every later
// word to its first rune plus '*' padding: "Nguyễn Văn An" -> "Nguyễn V** A*".
// A single-word name keeps only its first rune: "Madonna" -> "M******".
func MaskName(name string) string {
	words := strings.Fields(name)
	if len(words) == 0 {
		return ""
	}
	if len(words) == 1 {
		return maskWord(words[0])
	}
	parts := []string{words[0]}
	for _, w := range words[1:] {
		parts = append(parts, maskWord(w))
	}
	return strings.Join(parts, " ")
}

func maskWord(w string) string {
	r := []rune(w)
	if len(r) <= 1 {
		return string(r)
	}
	return string(r[0]) + strings.Repeat("*", len(r)-1)
}
