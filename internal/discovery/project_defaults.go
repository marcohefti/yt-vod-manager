package discovery

const (
	DefaultProfileName        = "default"
	DefaultWorkers            = 5
	DefaultFragments          = 10
	DefaultOrder              = "oldest"
	DefaultQuality            = "best"
	DefaultJSRuntime          = JSRuntimeAuto
	DefaultSubtitleLanguage   = "english"
	DefaultBrowserCookieAgent = "chrome"
	DefaultDownloadLimitMBps  = 0
	DefaultProxyMode          = ProxyModeOff

	JSRuntimeAuto    = "auto"
	JSRuntimeDeno    = "deno"
	JSRuntimeNode    = "node"
	JSRuntimeQuickJS = "quickjs"
	JSRuntimeBun     = "bun"
)
