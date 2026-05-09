package cliout

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestJSON_RoundTrip(t *testing.T) {
	type sample struct {
		Foo string `json:"foo"`
		Bar int    `json:"bar"`
	}
	in := sample{Foo: "hello", Bar: 42}
	var buf bytes.Buffer
	if err := JSON(&buf, in); err != nil {
		t.Fatalf("JSON: %v", err)
	}
	var out sample
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out != in {
		t.Errorf("round-trip mismatch: got %+v, want %+v", out, in)
	}
	// Indented output should contain newlines + spaces.
	if !strings.Contains(buf.String(), "\n  ") {
		t.Errorf("expected indented JSON, got: %q", buf.String())
	}
	// Trailing newline.
	if !strings.HasSuffix(buf.String(), "\n") {
		t.Errorf("expected trailing newline")
	}
}

func TestRegisterJSONFlag_DefaultFalse(t *testing.T) {
	cmd := &cobra.Command{Use: "x"}
	flag := RegisterJSONFlag(cmd)
	if *flag != false {
		t.Errorf("expected default false, got %v", *flag)
	}
	if f := cmd.Flag("json"); f == nil {
		t.Errorf("expected --json flag registered")
	}
}

func TestRender_TextMode(t *testing.T) {
	var buf bytes.Buffer
	humanCalled := false
	err := Render(false, &buf, func(w io.Writer) error {
		humanCalled = true
		w.Write([]byte("hello world"))
		return nil
	}, struct{}{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !humanCalled {
		t.Error("human closure was not invoked in text mode")
	}
	if !strings.Contains(buf.String(), "hello world") {
		t.Errorf("expected human text in output, got: %q", buf.String())
	}
}

func TestRender_JSONMode(t *testing.T) {
	type sample struct {
		Foo string `json:"foo"`
	}
	var buf bytes.Buffer
	err := Render(true, &buf, func(w io.Writer) error {
		t.Errorf("human closure should not run in JSON mode")
		return nil
	}, sample{Foo: "ok"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(buf.String(), `"foo": "ok"`) {
		t.Errorf("expected JSON output, got: %q", buf.String())
	}
}
