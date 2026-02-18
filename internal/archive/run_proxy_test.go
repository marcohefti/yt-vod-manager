package archive

import "testing"

func TestProxyForWorker(t *testing.T) {
	proxies := []string{"http://p1:8080", "http://p2:8080"}
	if got := proxyForWorker(1, proxyModePerWorker, proxies); got != proxies[0] {
		t.Fatalf("worker 1 proxy mismatch: got %q want %q", got, proxies[0])
	}
	if got := proxyForWorker(2, proxyModePerWorker, proxies); got != proxies[1] {
		t.Fatalf("worker 2 proxy mismatch: got %q want %q", got, proxies[1])
	}
	if got := proxyForWorker(1, proxyModeOff, proxies); got != "" {
		t.Fatalf("expected empty proxy for off mode, got %q", got)
	}
}
