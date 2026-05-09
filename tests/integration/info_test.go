//go:build integration

package integration

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInfo_HumanText(t *testing.T) {
	td := testdir(t)
	src := filepath.Join(td, "svs", "CMU-1-Small-Region.svs")
	bin := buildOnce(t)

	out, err := exec.Command(bin, "info", src).CombinedOutput()
	if err != nil {
		t.Fatalf("info: %v\n%s", err, out)
	}
	got := string(out)
	for _, want := range []string{
		"Format:  svs",
		"Levels:",
		"L0",
		"Associated images:",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output:\n%s", want, got)
		}
	}
}

func TestInfo_JSON(t *testing.T) {
	td := testdir(t)
	src := filepath.Join(td, "svs", "CMU-1-Small-Region.svs")
	bin := buildOnce(t)

	out, err := exec.Command(bin, "info", "--json", src).CombinedOutput()
	if err != nil {
		t.Fatalf("info --json: %v\n%s", err, out)
	}

	var got struct {
		Path      string `json:"path"`
		SizeBytes int64  `json:"size_bytes"`
		Format    string `json:"format"`
		Levels    []struct {
			Index       int    `json:"index"`
			Width       int    `json:"width"`
			Height      int    `json:"height"`
			Compression string `json:"compression"`
		} `json:"levels"`
		Associated []struct {
			Kind        string `json:"kind"`
			Compression string `json:"compression"`
		} `json:"associated_images"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	if got.Format != "svs" {
		t.Errorf("Format = %q, want svs", got.Format)
	}
	if got.SizeBytes <= 0 {
		t.Errorf("SizeBytes = %d, want > 0", got.SizeBytes)
	}
	if len(got.Levels) == 0 {
		t.Errorf("no levels in JSON output")
	}
	if len(got.Associated) == 0 {
		t.Errorf("no associated images in JSON output")
	}
}
