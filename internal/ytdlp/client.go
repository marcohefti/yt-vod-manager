package ytdlp

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

type OutputStream string

const (
	StreamStdout OutputStream = "stdout"
	StreamStderr OutputStream = "stderr"
)

type FlatPlaylistOptions struct {
	SourceURL          string
	CookiesPath        string
	CookiesFromBrowser string
	JSRuntime          string
}

type DownloadOptions struct {
	VideoURL           string
	OutputDir          string
	Fragments          int
	DownloadArchive    string
	CookiesPath        string
	CookiesFromBrowser string
	Quality            string
	DeliveryMode       string
	SubLangs           string
	DownloadLimitMBps  float64
	ProxyURL           string
	Stdout             io.Writer
	Stderr             io.Writer
	LogWriter          io.Writer
	EchoOutput         bool
	Progress           func(stream OutputStream, line string)
	JSRuntime          string
}

type DownloadResult struct {
	Command []string
}

type DependencyReport struct {
	YTDLPFound  bool   `json:"yt_dlp_found"`
	YTDLPPath   string `json:"yt_dlp_path,omitempty"`
	FFmpegFound bool   `json:"ffmpeg_found"`
	FFmpegPath  string `json:"ffmpeg_path,omitempty"`
}

func CheckJSRuntime(raw string) (string, error) {
	runtime, ok := normalizeJSRuntime(raw)
	if !ok {
		return "", fmt.Errorf("invalid js runtime %q (expected auto, deno, node, quickjs, or bun)", strings.TrimSpace(raw))
	}
	if runtime == "auto" {
		return runtime, nil
	}
	candidates := jsRuntimeBinaryCandidates(runtime)
	for _, bin := range candidates {
		if _, err := exec.LookPath(bin); err == nil {
			return runtime, nil
		}
	}
	return "", fmt.Errorf("missing dependency for js runtime %q: install one of [%s] or set js runtime to auto", runtime, strings.Join(candidates, ", "))
}

func DependencyStatus() DependencyReport {
	report := DependencyReport{}
	if path, err := exec.LookPath("yt-dlp"); err == nil {
		report.YTDLPFound = true
		report.YTDLPPath = path
	}
	if path, err := exec.LookPath("ffmpeg"); err == nil {
		report.FFmpegFound = true
		report.FFmpegPath = path
	}
	return report
}

func CheckDependencies() error {
	report := DependencyStatus()
	if !report.YTDLPFound {
		return fmt.Errorf("missing dependency: yt-dlp is not installed or not on PATH")
	}
	if !report.FFmpegFound {
		return fmt.Errorf("missing dependency: ffmpeg is required for many YouTube formats and was not found on PATH")
	}
	return nil
}

func FlatPlaylistJSON(opts FlatPlaylistOptions) ([]byte, error) {
	if strings.TrimSpace(opts.SourceURL) == "" {
		return nil, fmt.Errorf("source URL is required")
	}

	args := []string{"--flat-playlist", "-J"}
	if strings.TrimSpace(opts.CookiesPath) != "" {
		cookiesPath, err := resolveCookiesPath(opts.CookiesPath)
		if err != nil {
			return nil, err
		}
		args = append(args, "--cookies", cookiesPath)
	}
	if strings.TrimSpace(opts.CookiesFromBrowser) != "" {
		args = append(args, "--cookies-from-browser", opts.CookiesFromBrowser)
	}
	var err error
	args, err = appendJSRuntimeArgs(args, opts.JSRuntime)
	if err != nil {
		return nil, err
	}
	args = append(args, opts.SourceURL)

	cmd := exec.Command("yt-dlp", args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("yt-dlp failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	if stdout.Len() == 0 {
		return nil, fmt.Errorf("yt-dlp returned empty output")
	}
	return stdout.Bytes(), nil
}

func DownloadVideo(opts DownloadOptions) (DownloadResult, error) {
	if strings.TrimSpace(opts.VideoURL) == "" {
		return DownloadResult{}, fmt.Errorf("video URL is required")
	}
	if strings.TrimSpace(opts.OutputDir) == "" {
		return DownloadResult{}, fmt.Errorf("output directory is required")
	}
	fragments := opts.Fragments
	if fragments <= 0 {
		fragments = 10
	}

	args := []string{
		"--no-playlist",
		"--newline",
		"--restrict-filenames",
		"-N", fmt.Sprintf("%d", fragments),
		"-P", opts.OutputDir,
		"-o", "%(uploader)s/%(upload_date)s_%(title).200B_[%(id)s].%(ext)s",
		"--download-archive", opts.DownloadArchive,
	}
	switch strings.ToLower(strings.TrimSpace(opts.DeliveryMode)) {
	case "fragmented":
		args = append(args,
			"--hls-prefer-native",
			"--downloader", "m3u8:native",
			"-f", selectFormat(opts.Quality, true),
		)
	default:
		args = append(args, "-f", selectFormat(opts.Quality, false))
	}
	if strings.TrimSpace(opts.CookiesPath) != "" {
		cookiesPath, err := resolveCookiesPath(opts.CookiesPath)
		if err != nil {
			return DownloadResult{}, err
		}
		args = append(args, "--cookies", cookiesPath)
	}
	if strings.TrimSpace(opts.CookiesFromBrowser) != "" {
		args = append(args, "--cookies-from-browser", opts.CookiesFromBrowser)
	}
	if opts.DownloadLimitMBps > 0 {
		args = append(args, "--limit-rate", formatRateLimitMBps(opts.DownloadLimitMBps))
	}
	if strings.TrimSpace(opts.ProxyURL) != "" {
		args = append(args, "--proxy", strings.TrimSpace(opts.ProxyURL))
	}
	args, err := appendJSRuntimeArgs(args, opts.JSRuntime)
	if err != nil {
		return DownloadResult{}, err
	}
	args = append(args, opts.VideoURL)

	if err := runCommand(args, opts); err != nil {
		return DownloadResult{Command: append([]string{"yt-dlp"}, args...)}, err
	}
	return DownloadResult{Command: append([]string{"yt-dlp"}, args...)}, nil
}

func DownloadSubtitles(opts DownloadOptions) (DownloadResult, error) {
	if strings.TrimSpace(opts.VideoURL) == "" {
		return DownloadResult{}, fmt.Errorf("video URL is required")
	}
	if strings.TrimSpace(opts.OutputDir) == "" {
		return DownloadResult{}, fmt.Errorf("output directory is required")
	}
	lang := strings.TrimSpace(opts.SubLangs)
	lang = normalizeSubLangs(lang)

	args := []string{
		"--no-playlist",
		"--skip-download",
		"--newline",
		"--restrict-filenames",
		"-P", opts.OutputDir,
		"-o", "%(uploader)s/%(upload_date)s_%(title).200B_[%(id)s].%(ext)s",
		"--write-subs",
		"--write-auto-subs",
		"--sub-langs", lang,
		"--convert-subs", "vtt",
	}
	if strings.TrimSpace(opts.CookiesPath) != "" {
		cookiesPath, err := resolveCookiesPath(opts.CookiesPath)
		if err != nil {
			return DownloadResult{}, err
		}
		args = append(args, "--cookies", cookiesPath)
	}
	if strings.TrimSpace(opts.CookiesFromBrowser) != "" {
		args = append(args, "--cookies-from-browser", opts.CookiesFromBrowser)
	}
	if opts.DownloadLimitMBps > 0 {
		args = append(args, "--limit-rate", formatRateLimitMBps(opts.DownloadLimitMBps))
	}
	if strings.TrimSpace(opts.ProxyURL) != "" {
		args = append(args, "--proxy", strings.TrimSpace(opts.ProxyURL))
	}
	args, err := appendJSRuntimeArgs(args, opts.JSRuntime)
	if err != nil {
		return DownloadResult{}, err
	}
	args = append(args, opts.VideoURL)

	if err := runCommand(args, opts); err != nil {
		return DownloadResult{Command: append([]string{"yt-dlp"}, args...)}, err
	}
	return DownloadResult{Command: append([]string{"yt-dlp"}, args...)}, nil
}

func selectFormat(rawQuality string, fragmented bool) string {
	quality := strings.ToLower(strings.TrimSpace(rawQuality))
	switch quality {
	case "", "best":
		if fragmented {
			return "bv*[protocol*=m3u8]+ba[protocol*=m3u8]/b[protocol*=m3u8]"
		}
		return "bv*+ba/b"
	case "1080p", "1080", "hd":
		if fragmented {
			return "bv*[protocol*=m3u8][height<=1080]+ba[protocol*=m3u8]/b[protocol*=m3u8][height<=1080]"
		}
		return "bv*[height<=1080]+ba/b[height<=1080]"
	case "720p", "720", "sd", "small":
		if fragmented {
			return "bv*[protocol*=m3u8][height<=720]+ba[protocol*=m3u8]/b[protocol*=m3u8][height<=720]"
		}
		return "bv*[height<=720]+ba/b[height<=720]"
	default:
		if fragmented {
			return "bv*[protocol*=m3u8]+ba[protocol*=m3u8]/b[protocol*=m3u8]"
		}
		return "bv*+ba/b"
	}
}

func normalizeSubLangs(raw string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	switch v {
	case "", "english", "en":
		return "en.*,en,-live_chat"
	case "all":
		return "all,-live_chat"
	default:
		return raw
	}
}

func appendJSRuntimeArgs(args []string, rawRuntime string) ([]string, error) {
	runtime, ok := normalizeJSRuntime(rawRuntime)
	if !ok {
		return nil, fmt.Errorf("invalid js runtime %q (expected auto, deno, node, quickjs, or bun)", strings.TrimSpace(rawRuntime))
	}
	if runtime == "auto" {
		return args, nil
	}
	return append(args, "--no-js-runtimes", "--js-runtimes", runtime), nil
}

func normalizeJSRuntime(raw string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "auto":
		return "auto", true
	case "deno", "node", "quickjs", "bun":
		return strings.ToLower(strings.TrimSpace(raw)), true
	default:
		return "", false
	}
}

func jsRuntimeBinaryCandidates(runtime string) []string {
	switch runtime {
	case "quickjs":
		return []string{"quickjs", "qjs"}
	default:
		return []string{runtime}
	}
}

func runCommand(args []string, opts DownloadOptions) error {
	cmd := exec.Command("yt-dlp", args...)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("setup stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("setup stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start yt-dlp: %w", err)
	}

	var outBuf strings.Builder
	var errBuf strings.Builder
	var mu sync.Mutex
	var wg sync.WaitGroup

	read := func(stream OutputStream, r io.Reader, echoW io.Writer) {
		defer wg.Done()
		scanner := bufio.NewScanner(r)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 1024*1024)
		scanner.Split(splitByNewlineOrCR)
		for scanner.Scan() {
			line := scanner.Text()
			mu.Lock()
			appendLimited(&outBuf, &errBuf, stream, line)
			if opts.LogWriter != nil {
				_, _ = io.WriteString(opts.LogWriter, line+"\n")
			}
			mu.Unlock()

			if opts.EchoOutput && echoW != nil {
				_, _ = io.WriteString(echoW, line+"\n")
			}
			if opts.Progress != nil {
				opts.Progress(stream, line)
			}
		}
	}

	wg.Add(2)
	go read(StreamStdout, stdoutPipe, opts.Stdout)
	go read(StreamStderr, stderrPipe, opts.Stderr)
	wg.Wait()

	if err := cmd.Wait(); err != nil {
		mu.Lock()
		defer mu.Unlock()
		return fmt.Errorf("yt-dlp failed: %w\n%s\n%s", err, strings.TrimSpace(errBuf.String()), strings.TrimSpace(outBuf.String()))
	}
	return nil
}

func splitByNewlineOrCR(data []byte, atEOF bool) (advance int, token []byte, err error) {
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' || data[i] == '\r' {
			if i == 0 {
				return 1, nil, nil
			}
			return i + 1, data[:i], nil
		}
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func appendLimited(outBuf, errBuf *strings.Builder, stream OutputStream, line string) {
	const maxKeep = 8192
	b := outBuf
	if stream == StreamStderr {
		b = errBuf
	}
	if b.Len() >= maxKeep {
		return
	}
	toWrite := line + "\n"
	remain := maxKeep - b.Len()
	if len(toWrite) > remain {
		toWrite = toWrite[:remain]
	}
	b.WriteString(toWrite)
}

func formatRateLimitMBps(v float64) string {
	return fmt.Sprintf("%gM", v)
}

func resolveCookiesPath(path string) (string, error) {
	p := strings.TrimSpace(path)
	if p == "" {
		return "", nil
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("resolve cookies path %s: %w", p, err)
	}
	if _, err := os.Stat(abs); err != nil {
		return "", fmt.Errorf("cookies file %s: %w", abs, err)
	}
	return abs, nil
}
