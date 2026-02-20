package logx

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

const maxBodyPreview = 512

// LoggingRoundTripper logs redacted request/response details in debug mode.
type LoggingRoundTripper struct {
	base http.RoundTripper
}

// NewLoggingRoundTripper wraps base with request/response logging.
func NewLoggingRoundTripper(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &LoggingRoundTripper{base: base}
}

func (l *LoggingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if !DebugEnabled() {
		return l.base.RoundTrip(req)
	}

	start := time.Now()
	Debugf("http_request method=%s url=%s headers=%q body_preview=%q",
		req.Method,
		RedactURL(req.URL.String()),
		flattenHeaders(req.Header),
		requestBodyPreview(req),
	)

	resp, err := l.base.RoundTrip(req)
	if err != nil {
		Debugf("http_error method=%s url=%s duration=%s error=%q",
			req.Method,
			RedactURL(req.URL.String()),
			time.Since(start),
			RedactSensitive(err.Error()),
		)
		return nil, err
	}

	preview := ""
	contentType := resp.Header.Get("Content-Type")
	if IsTextLikeContentType(contentType) && resp.Body != nil {
		prefix, _ := io.ReadAll(io.LimitReader(resp.Body, maxBodyPreview))
		preview = RedactSensitive(string(prefix))
		resp.Body = io.NopCloser(io.MultiReader(bytes.NewReader(prefix), resp.Body))
	}

	Debugf("http_response method=%s url=%s status=%d duration=%s content_type=%q body_preview=%q",
		req.Method,
		RedactURL(req.URL.String()),
		resp.StatusCode,
		time.Since(start),
		contentType,
		preview,
	)

	return resp, nil
}

func requestBodyPreview(req *http.Request) string {
	if req == nil || req.GetBody == nil {
		return ""
	}
	if !IsTextLikeContentType(req.Header.Get("Content-Type")) {
		return ""
	}
	body, err := req.GetBody()
	if err != nil {
		return ""
	}
	defer body.Close()

	prefix, err := io.ReadAll(io.LimitReader(body, maxBodyPreview))
	if err != nil {
		return ""
	}
	return RedactSensitive(string(prefix))
}

func flattenHeaders(h http.Header) string {
	if len(h) == 0 {
		return ""
	}
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		values := h.Values(k)
		redacted := make([]string, 0, len(values))
		for _, v := range values {
			redacted = append(redacted, RedactHeaderValue(k, v))
		}
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, strings.Join(redacted, ";")))
	}
	return strings.Join(pairs, ",")
}
