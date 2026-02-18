package discovery

import "testing"

func TestReadGlobalSettingsDefaultsWhenConfigMissing(t *testing.T) {
	tmp := t.TempDir()
	cfg := tmp + "/missing.json"

	global, err := ReadGlobalSettings(cfg)
	if err != nil {
		t.Fatalf("read global settings failed: %v", err)
	}
	if global.Workers != DefaultWorkers {
		t.Fatalf("workers default mismatch: got %d want %d", global.Workers, DefaultWorkers)
	}
	if global.ProxyMode != DefaultProxyMode {
		t.Fatalf("proxy mode default mismatch: got %q want %q", global.ProxyMode, DefaultProxyMode)
	}
	if len(global.Proxies) != 0 {
		t.Fatalf("expected no default proxies, got %d", len(global.Proxies))
	}
}

func TestResolveRuntimeNetworkSettings(t *testing.T) {
	global := GlobalSettings{
		Workers:           4,
		DownloadLimitMBps: 12.5,
		ProxyMode:         ProxyModePerWorker,
		Proxies:           []string{"http://p1:8080", "http://p2:8080", "http://p3:8080", "http://p4:8080"},
	}
	project := Project{Workers: 2}

	out, err := ResolveRuntimeNetworkSettings(project, global, 0, nil)
	if err != nil {
		t.Fatalf("resolve runtime settings failed: %v", err)
	}
	if out.Workers != 2 {
		t.Fatalf("workers mismatch: got %d want %d", out.Workers, 2)
	}
	if out.DownloadLimitMBps != 12.5 {
		t.Fatalf("download limit mismatch: got %v want %v", out.DownloadLimitMBps, 12.5)
	}
	if out.ProxyMode != ProxyModePerWorker {
		t.Fatalf("proxy mode mismatch: got %q want %q", out.ProxyMode, ProxyModePerWorker)
	}

	override := 0.0
	out, err = ResolveRuntimeNetworkSettings(Project{}, global, 0, &override)
	if err != nil {
		t.Fatalf("resolve runtime settings with override failed: %v", err)
	}
	if out.DownloadLimitMBps != 0 {
		t.Fatalf("download limit override mismatch: got %v want 0", out.DownloadLimitMBps)
	}
}

func TestResolveRuntimeNetworkSettingsRequiresEnoughProxies(t *testing.T) {
	global := GlobalSettings{
		Workers:   5,
		ProxyMode: ProxyModePerWorker,
		Proxies:   []string{"http://p1:8080"},
	}
	_, err := ResolveRuntimeNetworkSettings(Project{}, global, 0, nil)
	if err == nil {
		t.Fatal("expected error when workers exceed proxy count")
	}
}
