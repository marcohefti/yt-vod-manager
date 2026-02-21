package cli

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	selfUpdateRepoOwner = "marcohefti"
	selfUpdateRepoName  = "yt-vod-manager"
	selfUpdateBinary    = "yt-vod-manager"
)

type selfUpdateRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

type selfUpdateResult struct {
	Tag          string `json:"tag"`
	Asset        string `json:"asset"`
	InstallDir   string `json:"install_dir"`
	InstalledExe string `json:"installed_exe"`
}

func runSelfUpdate(args []string) error {
	fs := flag.NewFlagSet("self-update", flag.ContinueOnError)
	version := fs.String("version", "", "target release version (for example v0.2.0 or 0.2.0); defaults to latest stable")
	installDir := fs.String("install-dir", "", "install directory (defaults to user-local install path)")
	jsonOut := fs.Bool("json", false, "print JSON output")
	fs.SetOutput(flag.CommandLine.Output())
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}

	release, err := fetchRelease(*version)
	if err != nil {
		return err
	}
	assetName, binaryName, err := releaseAssetForRuntime(release.TagName)
	if err != nil {
		return err
	}

	checksumAsset := fmt.Sprintf("%s_%s_checksums.txt", selfUpdateBinary, release.TagName)
	archiveURL, ok := releaseAssetURL(release, assetName)
	if !ok {
		return fmt.Errorf("release %s does not contain expected asset %q", release.TagName, assetName)
	}
	checksumURL, ok := releaseAssetURL(release, checksumAsset)
	if !ok {
		return fmt.Errorf("release %s does not contain checksum asset %q", release.TagName, checksumAsset)
	}

	targetDir := strings.TrimSpace(*installDir)
	if targetDir == "" {
		targetDir, err = defaultInstallDir()
		if err != nil {
			return err
		}
	}
	targetExe := filepath.Join(targetDir, binaryName)

	currentExe, currentExeErr := os.Executable()
	if currentExeErr == nil && runtime.GOOS == "windows" && samePath(currentExe, targetExe) {
		return fmt.Errorf("current executable is already running from target path %q; rerun with --install-dir to install side-by-side", targetExe)
	}

	result, err := performSelfUpdate(release.TagName, assetName, archiveURL, checksumURL, targetExe)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}

	fmt.Printf("updated to %s\n", result.Tag)
	fmt.Printf("installed: %s\n", result.InstalledExe)
	if currentExeErr == nil && !samePath(currentExe, result.InstalledExe) {
		fmt.Printf("run this binary to use the updated version now:\n  %s\n", result.InstalledExe)
	}
	if runtime.GOOS == "windows" {
		fmt.Println("tip: keep this install path on your user PATH for fast updates outside winget review timing.")
	}
	return nil
}

func fetchRelease(version string) (selfUpdateRelease, error) {
	version = strings.TrimSpace(version)
	endpoint := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", selfUpdateRepoOwner, selfUpdateRepoName)
	if version != "" {
		tag := version
		if !strings.HasPrefix(tag, "v") {
			tag = "v" + tag
		}
		endpoint = fmt.Sprintf(
			"https://api.github.com/repos/%s/%s/releases/tags/%s",
			selfUpdateRepoOwner,
			selfUpdateRepoName,
			url.PathEscape(tag),
		)
	}

	var release selfUpdateRelease
	if err := getJSON(endpoint, &release); err != nil {
		return selfUpdateRelease{}, err
	}
	if strings.TrimSpace(release.TagName) == "" {
		return selfUpdateRelease{}, errors.New("release response did not include tag_name")
	}
	return release, nil
}

func getJSON(endpoint string, out any) error {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "yt-vod-manager-self-update")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("github api request failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func releaseAssetForRuntime(tag string) (archiveName string, binaryName string, err error) {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "windows/amd64":
		return fmt.Sprintf("%s_%s_windows_amd64.zip", selfUpdateBinary, tag), selfUpdateBinary + ".exe", nil
	case "darwin/amd64":
		return fmt.Sprintf("%s_%s_darwin_amd64.tar.gz", selfUpdateBinary, tag), selfUpdateBinary, nil
	case "darwin/arm64":
		return fmt.Sprintf("%s_%s_darwin_arm64.tar.gz", selfUpdateBinary, tag), selfUpdateBinary, nil
	case "linux/amd64":
		return fmt.Sprintf("%s_%s_linux_amd64.tar.gz", selfUpdateBinary, tag), selfUpdateBinary, nil
	case "linux/arm64":
		return fmt.Sprintf("%s_%s_linux_arm64.tar.gz", selfUpdateBinary, tag), selfUpdateBinary, nil
	default:
		return "", "", fmt.Errorf("self-update is not supported on %s/%s", runtime.GOOS, runtime.GOARCH)
	}
}

func releaseAssetURL(release selfUpdateRelease, assetName string) (string, bool) {
	for _, a := range release.Assets {
		if a.Name == assetName && strings.TrimSpace(a.BrowserDownloadURL) != "" {
			return a.BrowserDownloadURL, true
		}
	}
	return "", false
}

func defaultInstallDir() (string, error) {
	switch runtime.GOOS {
	case "windows":
		base := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, "AppData", "Local")
		}
		return filepath.Join(base, "Programs", selfUpdateBinary), nil
	default:
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".local", "bin"), nil
	}
}

func performSelfUpdate(tag, assetName, archiveURL, checksumURL, targetExe string) (selfUpdateResult, error) {
	workdir, err := os.MkdirTemp("", "yt-vod-manager-self-update-*")
	if err != nil {
		return selfUpdateResult{}, err
	}
	defer os.RemoveAll(workdir)

	archivePath := filepath.Join(workdir, assetName)
	checksumPath := filepath.Join(workdir, "checksums.txt")

	if err := downloadFile(archiveURL, archivePath); err != nil {
		return selfUpdateResult{}, err
	}
	if err := downloadFile(checksumURL, checksumPath); err != nil {
		return selfUpdateResult{}, err
	}

	expectedHash, err := expectedChecksumForAsset(checksumPath, assetName)
	if err != nil {
		return selfUpdateResult{}, err
	}
	actualHash, err := sha256File(archivePath)
	if err != nil {
		return selfUpdateResult{}, err
	}
	if !strings.EqualFold(expectedHash, actualHash) {
		return selfUpdateResult{}, fmt.Errorf("checksum mismatch for %s (expected %s, got %s)", assetName, expectedHash, actualHash)
	}

	extractedExe, err := extractBinary(workdir, archivePath)
	if err != nil {
		return selfUpdateResult{}, err
	}

	if err := installBinary(extractedExe, targetExe); err != nil {
		return selfUpdateResult{}, err
	}

	return selfUpdateResult{
		Tag:          tag,
		Asset:        assetName,
		InstallDir:   filepath.Dir(targetExe),
		InstalledExe: targetExe,
	}, nil
}

func downloadFile(fileURL, targetPath string) error {
	req, err := http.NewRequest(http.MethodGet, fileURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "yt-vod-manager-self-update")

	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("download failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	out, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		return err
	}
	return nil
}

func expectedChecksumForAsset(checksumPath, assetName string) (string, error) {
	content, err := os.ReadFile(checksumPath)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		hash := strings.TrimSpace(fields[0])
		name := strings.TrimLeft(strings.TrimSpace(fields[1]), "*")
		if name == assetName {
			return hash, nil
		}
	}
	return "", fmt.Errorf("checksum for %s not found in checksums file", assetName)
}

func sha256File(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func extractBinary(workdir, archivePath string) (string, error) {
	targetName := selfUpdateBinary
	if runtime.GOOS == "windows" {
		targetName += ".exe"
	}
	dstPath := filepath.Join(workdir, targetName)

	switch {
	case strings.HasSuffix(archivePath, ".zip"):
		if err := extractBinaryFromZip(archivePath, targetName, dstPath); err != nil {
			return "", err
		}
	case strings.HasSuffix(archivePath, ".tar.gz"):
		if err := extractBinaryFromTarGz(archivePath, targetName, dstPath); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("unsupported archive format: %s", archivePath)
	}
	return dstPath, nil
}

func extractBinaryFromZip(archivePath, targetName, dstPath string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, f := range reader.File {
		if path.Base(f.Name) != targetName {
			continue
		}
		src, err := f.Open()
		if err != nil {
			return err
		}
		defer src.Close()

		out, err := os.Create(dstPath)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, src); err != nil {
			out.Close()
			return err
		}
		if err := out.Close(); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("binary %s not found in %s", targetName, archivePath)
}

func extractBinaryFromTarGz(archivePath, targetName, dstPath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if hdr == nil || hdr.FileInfo().IsDir() {
			continue
		}
		if path.Base(hdr.Name) != targetName {
			continue
		}

		out, err := os.Create(dstPath)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			return err
		}
		if err := out.Close(); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("binary %s not found in %s", targetName, archivePath)
}

func installBinary(srcPath, targetPath string) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	tmpPath := targetPath + ".tmp"
	if err := copyFile(srcPath, tmpPath); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(tmpPath, 0o755); err != nil {
			return err
		}
	}

	if runtime.GOOS == "windows" {
		_ = os.Remove(targetPath)
	}
	if err := os.Rename(tmpPath, targetPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func copyFile(srcPath, dstPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		return err
	}
	return dst.Close()
}

func samePath(a, b string) bool {
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return false
	}
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(absA), filepath.Clean(absB))
	}
	return filepath.Clean(absA) == filepath.Clean(absB)
}

