package ytdlp

import "testing"

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

func TestAppendJSRuntimeArgsRejectsInvalidRuntime(t *testing.T) {
	if _, err := appendJSRuntimeArgs(nil, "spidermonkey"); err == nil {
		t.Fatal("expected invalid runtime error")
	}
}
