package logx

import (
	"bytes"
	"errors"
	"log"
	"strings"
	"testing"
	"time"
)

func captureLogs(t *testing.T) (*bytes.Buffer, func()) {
	t.Helper()
	buf := &bytes.Buffer{}
	oldWriter := log.Writer()
	oldFlags := log.Flags()
	log.SetOutput(buf)
	log.SetFlags(0)
	return buf, func() {
		log.SetOutput(oldWriter)
		log.SetFlags(oldFlags)
	}
}

func TestDebugfHonorsFlag(t *testing.T) {
	buf, restore := captureLogs(t)
	defer restore()

	SetDebug(false)
	Debugf("off")
	if buf.Len() != 0 {
		t.Fatalf("expected no logs when debug disabled, got: %s", buf.String())
	}

	SetDebug(true)
	Debugf("on")
	if !strings.Contains(buf.String(), "DEBUG on") {
		t.Fatalf("expected debug output, got: %s", buf.String())
	}
}

func TestStepDoneWithErrorIncludesPhaseContext(t *testing.T) {
	buf, restore := captureLogs(t)
	defer restore()

	SetDebug(true)
	step := StepStart("fetch_servers", "phase", "fetch_servers")
	time.Sleep(1 * time.Millisecond)
	step.DoneWithError(errors.New("token=abc123"), "project", "prod")

	out := buf.String()
	if !strings.Contains(out, "step_start name=fetch_servers") {
		t.Fatalf("missing step_start: %s", out)
	}
	if !strings.Contains(out, "phase=fetch_servers") {
		t.Fatalf("missing phase context: %s", out)
	}
	if strings.Contains(out, "abc123") {
		t.Fatalf("sensitive token value should be redacted: %s", out)
	}
}

func TestStartWatchdogEmitsWaiting(t *testing.T) {
	buf, restore := captureLogs(t)
	defer restore()

	SetDebug(true)
	stop := StartWatchdog("list_projects", 5*time.Millisecond)
	time.Sleep(16 * time.Millisecond)
	stop()

	if !strings.Contains(buf.String(), "step_waiting name=list_projects") {
		t.Fatalf("expected waiting watchdog log, got: %s", buf.String())
	}
}

func TestRedactionHelpers(t *testing.T) {
	redacted := RedactSensitive("Authorization=Bearer topsecret token=abcdef password=hunter2")
	if strings.Contains(redacted, "topsecret") || strings.Contains(redacted, "abcdef") || strings.Contains(redacted, "hunter2") {
		t.Fatalf("expected sensitive fields to be redacted, got: %s", redacted)
	}

	header := RedactHeaderValue("X-Auth-Token", "real-token")
	if header != "[REDACTED]" {
		t.Fatalf("expected auth header redaction, got: %s", header)
	}

	u := RedactURL("https://api.example.test/v2/servers?token=abc&name=foo")
	if strings.Contains(u, "abc") {
		t.Fatalf("expected redacted url query, got: %s", u)
	}
}
