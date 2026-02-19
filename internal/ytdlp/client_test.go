package ytdlp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFormatRateLimitMBps(t *testing.T) {
	if got := formatRateLimitMBps(10); got != "10M" {
		t.Fatalf("unexpected rate format: got %q want %q", got, "10M")
	}
	if got := formatRateLimitMBps(2.5); got != "2.5M" {
		t.Fatalf("unexpected rate format: got %q want %q", got, "2.5M")
	}
}

func TestAppendJSRuntimeArgsAutoUsesYTDLPDefault(t *testing.T) {
	args, err := appendJSRuntimeArgs([]string{"--flat-playlist"}, "")
	if err != nil {
		t.Fatalf("unexpected error for empty runtime: %v", err)
	}
	if len(args) != 1 || args[0] != "--flat-playlist" {
		t.Fatalf("expected unchanged args for auto runtime, got %#v", args)
	}
}

func TestAppendJSRuntimeArgsNodeForcesRuntime(t *testing.T) {
	args, err := appendJSRuntimeArgs([]string{"--flat-playlist"}, "node")
	if err != nil {
		t.Fatalf("unexpected error for node runtime: %v", err)
	}
	if len(args) != 4 {
		t.Fatalf("expected 4 args, got %#v", args)
	}
	if args[1] != "--no-js-runtimes" || args[2] != "--js-runtimes" || args[3] != "node" {
		t.Fatalf("unexpected js runtime args: %#v", args)
	}
}

func TestAppendJSRuntimeArgsSupportsFallbackChain(t *testing.T) {
	args, err := appendJSRuntimeArgs([]string{"--flat-playlist"}, "node,quickjs")
	if err != nil {
		t.Fatalf("unexpected error for runtime chain: %v", err)
	}
	if len(args) != 6 {
		t.Fatalf("expected 6 args, got %#v", args)
	}
	if args[1] != "--no-js-runtimes" || args[2] != "--js-runtimes" || args[3] != "node" || args[4] != "--js-runtimes" || args[5] != "quickjs" {
		t.Fatalf("unexpected runtime chain args: %#v", args)
	}
}

func TestAppendJSRuntimeArgsRejectsInvalidRuntime(t *testing.T) {
	if _, err := appendJSRuntimeArgs(nil, "spidermonkey"); err == nil {
		t.Fatal("expected invalid runtime error")
	}
}

func TestCheckJSRuntimeAutoSkipsDependencyLookup(t *testing.T) {
	got, err := CheckJSRuntime("auto")
	if err != nil {
		t.Fatalf("unexpected error for auto runtime: %v", err)
	}
	if got != "auto" {
		t.Fatalf("expected auto, got %q", got)
	}
}

func TestCheckJSRuntimeRejectsMissingDependency(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("PATH", tmp)
	_, err := CheckJSRuntime("node")
	if err == nil {
		t.Fatal("expected missing dependency error for node")
	}
	if !strings.Contains(err.Error(), "missing dependency") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckJSRuntimeQuickJSAcceptsQJSBinary(t *testing.T) {
	tmp := t.TempDir()
	qjs := filepath.Join(tmp, "qjs")
	if err := os.WriteFile(qjs, []byte("#!/usr/bin/env bash\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write qjs: %v", err)
	}
	t.Setenv("PATH", tmp)
	got, err := CheckJSRuntime("quickjs")
	if err != nil {
		t.Fatalf("unexpected error for quickjs/qjs: %v", err)
	}
	if got != "quickjs" {
		t.Fatalf("expected quickjs, got %q", got)
	}
}

func TestCheckJSRuntimeChainFiltersUnavailableEntries(t *testing.T) {
	tmp := t.TempDir()
	qjs := filepath.Join(tmp, "qjs")
	if err := os.WriteFile(qjs, []byte("#!/usr/bin/env bash\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write qjs: %v", err)
	}
	t.Setenv("PATH", tmp)
	got, err := CheckJSRuntime("node,quickjs")
	if err != nil {
		t.Fatalf("unexpected error for runtime chain with fallback: %v", err)
	}
	if got != "quickjs" {
		t.Fatalf("expected filtered runtime list quickjs, got %q", got)
	}
}
