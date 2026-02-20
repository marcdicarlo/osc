package logx

import (
	"net/url"
	"regexp"
	"strings"
)

var sensitiveKVPattern = regexp.MustCompile(`(?i)((?:token|password|secret|api[_-]?key|auth(?:orization)?)[^=\s]*=)([^&\s]+)`)
var sensitiveJSONPattern = regexp.MustCompile(`(?i)("(?:token|password|secret|api[_-]?key|auth(?:orization)?)"\s*:\s*")([^"]+)(")`)
var bearerPattern = regexp.MustCompile(`(?i)(authorization\s*[:=]\s*bearer\s+)([^\s]+)`)

var sensitiveHeaderKeys = map[string]struct{}{
	"authorization":   {},
	"x-auth-token":    {},
	"x-subject-token": {},
	"cookie":          {},
}

// RedactSensitive scrubs common credential/token markers from free-form strings.
func RedactSensitive(in string) string {
	if in == "" {
		return in
	}
	out := bearerPattern.ReplaceAllString(in, `${1}[REDACTED]`)
	out = sensitiveKVPattern.ReplaceAllString(out, `${1}[REDACTED]`)
	out = sensitiveJSONPattern.ReplaceAllString(out, `${1}[REDACTED]${3}`)
	return out
}

// RedactHeaderValue redacts values for known-sensitive headers.
func RedactHeaderValue(key, value string) string {
	if _, ok := sensitiveHeaderKeys[strings.ToLower(strings.TrimSpace(key))]; ok {
		return "[REDACTED]"
	}
	return RedactSensitive(value)
}

// RedactURL hides sensitive query-string values while preserving route context.
func RedactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return RedactSensitive(raw)
	}
	query := u.Query()
	for key := range query {
		lk := strings.ToLower(key)
		if strings.Contains(lk, "token") || strings.Contains(lk, "secret") || strings.Contains(lk, "password") || strings.Contains(lk, "auth") || strings.Contains(lk, "key") {
			query.Set(key, "[REDACTED]")
		}
	}
	u.RawQuery = query.Encode()
	return u.String()
}

// IsTextLikeContentType reports whether safe small previews are likely text.
func IsTextLikeContentType(contentType string) bool {
	ct := strings.ToLower(contentType)
	return strings.Contains(ct, "json") || strings.Contains(ct, "xml") || strings.Contains(ct, "text/") || strings.Contains(ct, "application/x-www-form-urlencoded")
}
