package logx

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"strings"
	"testing"
)

type stubRoundTripper struct {
	resp *http.Response
	err  error
}

func (s stubRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return s.resp, s.err
}

func TestLoggingRoundTripperRedactsAndPreservesBody(t *testing.T) {
	buf := &bytes.Buffer{}
	oldWriter := log.Writer()
	oldFlags := log.Flags()
	log.SetOutput(buf)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(oldWriter)
		log.SetFlags(oldFlags)
	}()

	SetDebug(true)

	respBody := `{"token":"very-secret","status":"ok"}`
	rt := NewLoggingRoundTripper(stubRoundTripper{resp: &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(respBody)),
	}})

	req, err := http.NewRequest(http.MethodPost, "https://api.example.test/v2/servers?token=abc", strings.NewReader(`{"password":"mypw"}`))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer super-secret")

	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("roundtrip failed: %v", err)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	if string(bodyBytes) != respBody {
		t.Fatalf("response body changed after logging wrapper, got: %s", string(bodyBytes))
	}

	logs := buf.String()
	if !strings.Contains(logs, "http_request") || !strings.Contains(logs, "http_response") {
		t.Fatalf("missing request/response logs: %s", logs)
	}
	if strings.Contains(logs, "super-secret") || strings.Contains(logs, "mypw") || strings.Contains(logs, "very-secret") || strings.Contains(logs, "token=abc") {
		t.Fatalf("sensitive data should be redacted, logs: %s", logs)
	}
}
