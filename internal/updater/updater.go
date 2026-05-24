package updater

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const DefaultRepo = "ModelsLab/modelslab-cli"

type ReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type Release struct {
	TagName     string         `json:"tag_name"`
	HTMLURL     string         `json:"html_url"`
	PublishedAt time.Time      `json:"published_at"`
	Assets      []ReleaseAsset `json:"assets"`
}

type Info struct {
	CurrentVersion  string    `json:"current_version"`
	LatestVersion   string    `json:"latest_version"`
	UpdateAvailable bool      `json:"update_available"`
	CanCompare      bool      `json:"can_compare"`
	ReleaseURL      string    `json:"release_url"`
	AssetName       string    `json:"asset_name,omitempty"`
	AssetURL        string    `json:"asset_url,omitempty"`
	PublishedAt     time.Time `json:"published_at"`
}

type CheckOptions struct {
	CurrentVersion string
	Repo           string
	GOOS           string
	GOARCH         string
	HTTPClient     *http.Client
	UserAgent      string
}

type InstallOptions struct {
	CheckOptions
	Force        bool
	SkipChecksum bool
	TargetPath   string
}

type InstallResult struct {
	Info
	InstalledPath    string `json:"installed_path,omitempty"`
	ChecksumVerified bool   `json:"checksum_verified"`
}

type Cache struct {
	CheckedAt       time.Time `json:"checked_at"`
	CurrentVersion  string    `json:"current_version"`
	LatestVersion   string    `json:"latest_version"`
	UpdateAvailable bool      `json:"update_available"`
	CanCompare      bool      `json:"can_compare"`
	ReleaseURL      string    `json:"release_url"`
	PublishedAt     time.Time `json:"published_at"`
}

func Check(ctx context.Context, opts CheckOptions) (*Info, error) {
	release, err := fetchLatestRelease(ctx, opts)
	if err != nil {
		return nil, err
	}

	goos := firstNonEmpty(opts.GOOS, runtime.GOOS)
	goarch := firstNonEmpty(opts.GOARCH, runtime.GOARCH)
	asset := SelectArchiveAsset(release.Assets, release.TagName, goos, goarch)

	canCompare := IsComparableVersion(opts.CurrentVersion)
	updateAvailable := false
	if canCompare {
		updateAvailable = CompareVersions(release.TagName, opts.CurrentVersion) > 0
	}

	info := &Info{
		CurrentVersion:  opts.CurrentVersion,
		LatestVersion:   NormalizeVersion(release.TagName),
		UpdateAvailable: updateAvailable,
		CanCompare:      canCompare,
		ReleaseURL:      release.HTMLURL,
		PublishedAt:     release.PublishedAt,
	}
	if asset != nil {
		info.AssetName = asset.Name
		info.AssetURL = asset.BrowserDownloadURL
	}

	return info, nil
}

func Install(ctx context.Context, opts InstallOptions) (*InstallResult, error) {
	info, err := Check(ctx, opts.CheckOptions)
	if err != nil {
		return nil, err
	}
	if !info.UpdateAvailable && !opts.Force {
		return &InstallResult{Info: *info}, nil
	}
	if info.AssetURL == "" {
		return nil, fmt.Errorf("no release archive found for %s/%s", firstNonEmpty(opts.GOOS, runtime.GOOS), firstNonEmpty(opts.GOARCH, runtime.GOARCH))
	}

	targetPath := opts.TargetPath
	if targetPath == "" {
		targetPath, err = os.Executable()
		if err != nil {
			return nil, fmt.Errorf("could not resolve current executable: %w", err)
		}
	}
	if resolved, err := filepath.EvalSymlinks(targetPath); err == nil {
		targetPath = resolved
	}

	client := httpClient(opts.HTTPClient)
	archivePath, err := downloadToTemp(ctx, client, opts.UserAgent, info.AssetURL, info.AssetName)
	if err != nil {
		return nil, err
	}
	defer os.Remove(archivePath)

	checksumVerified := false
	if !opts.SkipChecksum {
		release, err := fetchLatestRelease(ctx, opts.CheckOptions)
		if err != nil {
			return nil, err
		}
		expected, err := findExpectedChecksum(ctx, client, opts.UserAgent, release.Assets, info.AssetName)
		if err != nil {
			return nil, err
		}
		if err := verifySHA256(archivePath, expected); err != nil {
			return nil, err
		}
		checksumVerified = true
	}

	extractedPath, err := extractBinaryToTemp(archivePath, info.AssetName)
	if err != nil {
		return nil, err
	}
	defer os.Remove(extractedPath)

	if err := replaceExecutable(targetPath, extractedPath); err != nil {
		return nil, err
	}

	return &InstallResult{
		Info:             *info,
		InstalledPath:    targetPath,
		ChecksumVerified: checksumVerified,
	}, nil
}

func CachedCheck(ctx context.Context, opts CheckOptions, cachePath string, interval time.Duration) (*Info, error) {
	if cache, err := ReadCache(cachePath); err == nil {
		if cache.CurrentVersion == opts.CurrentVersion && time.Since(cache.CheckedAt) < interval {
			return &Info{
				CurrentVersion:  cache.CurrentVersion,
				LatestVersion:   cache.LatestVersion,
				UpdateAvailable: cache.UpdateAvailable,
				CanCompare:      cache.CanCompare,
				ReleaseURL:      cache.ReleaseURL,
				PublishedAt:     cache.PublishedAt,
			}, nil
		}
	}

	info, err := Check(ctx, opts)
	if err != nil {
		return nil, err
	}

	_ = WriteCache(cachePath, Cache{
		CheckedAt:       time.Now(),
		CurrentVersion:  info.CurrentVersion,
		LatestVersion:   info.LatestVersion,
		UpdateAvailable: info.UpdateAvailable,
		CanCompare:      info.CanCompare,
		ReleaseURL:      info.ReleaseURL,
		PublishedAt:     info.PublishedAt,
	})

	return info, nil
}

func ReadCache(path string) (*Cache, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cache Cache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}

	return &cache, nil
}

func WriteCache(path string, cache Cache) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

func SelectArchiveAsset(assets []ReleaseAsset, version, goos, goarch string) *ReleaseAsset {
	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}

	normalizedVersion := NormalizeVersion(version)
	candidates := []string{
		fmt.Sprintf("modelslab_%s_%s_%s%s", normalizedVersion, goos, goarch, ext),
		fmt.Sprintf("modelslab_v%s_%s_%s%s", normalizedVersion, goos, goarch, ext),
		fmt.Sprintf("modelslab_%s_%s%s", goos, goarch, ext),
	}

	for _, candidate := range candidates {
		for i := range assets {
			if assets[i].Name == candidate {
				return &assets[i]
			}
		}
	}

	for i := range assets {
		name := strings.ToLower(assets[i].Name)
		if strings.HasSuffix(name, ext) && strings.Contains(name, goos) && strings.Contains(name, goarch) {
			return &assets[i]
		}
	}

	return nil
}

func SelectChecksumAsset(assets []ReleaseAsset) *ReleaseAsset {
	for i := range assets {
		if strings.EqualFold(assets[i].Name, "checksums.txt") {
			return &assets[i]
		}
	}
	for i := range assets {
		name := strings.ToLower(assets[i].Name)
		if strings.Contains(name, "checksum") && strings.HasSuffix(name, ".txt") {
			return &assets[i]
		}
	}
	return nil
}

func CompareVersions(a, b string) int {
	aparts := versionParts(a)
	bparts := versionParts(b)
	maxLen := len(aparts)
	if len(bparts) > maxLen {
		maxLen = len(bparts)
	}

	for i := 0; i < maxLen; i++ {
		av, bv := 0, 0
		if i < len(aparts) {
			av = aparts[i]
		}
		if i < len(bparts) {
			bv = bparts[i]
		}
		if av > bv {
			return 1
		}
		if av < bv {
			return -1
		}
	}

	return 0
}

func NormalizeVersion(version string) string {
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(version, "refs/tags/")
	version = strings.TrimPrefix(version, "v")
	return version
}

func IsComparableVersion(version string) bool {
	version = NormalizeVersion(version)
	if version == "" || version == "dev" || version == "none" || strings.Contains(version, "next") {
		return false
	}
	parts := versionParts(version)
	return len(parts) > 0
}

func fetchLatestRelease(ctx context.Context, opts CheckOptions) (*Release, error) {
	repo := firstNonEmpty(opts.Repo, DefaultRepo)
	url := "https://api.github.com/repos/" + repo + "/releases/latest"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", firstNonEmpty(opts.UserAgent, "modelslab-cli/"+opts.CurrentVersion))

	resp, err := httpClient(opts.HTTPClient).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("no GitHub release found for %s", repo)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("GitHub release check failed: HTTP %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}
	if release.TagName == "" {
		return nil, errors.New("GitHub release response did not include tag_name")
	}

	return &release, nil
}

func findExpectedChecksum(ctx context.Context, client *http.Client, userAgent string, assets []ReleaseAsset, assetName string) (string, error) {
	checksumAsset := SelectChecksumAsset(assets)
	if checksumAsset == nil {
		return "", errors.New("release does not include checksums.txt; rerun with --skip-checksum to bypass verification")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checksumAsset.BrowserDownloadURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", firstNonEmpty(userAgent, "modelslab-cli"))

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("could not download checksums.txt: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if strings.TrimPrefix(fields[1], "*") == assetName {
			return fields[0], nil
		}
	}

	return "", fmt.Errorf("checksums.txt does not include %s", assetName)
}

func verifySHA256(path, expected string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}

	actual := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("checksum mismatch for %s", filepath.Base(path))
	}

	return nil
}

func downloadToTemp(ctx context.Context, client *http.Client, userAgent, url, name string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", firstNonEmpty(userAgent, "modelslab-cli"))

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("could not download %s: HTTP %d", name, resp.StatusCode)
	}

	file, err := os.CreateTemp("", "modelslab-update-*"+filepath.Ext(name))
	if err != nil {
		return "", err
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		os.Remove(file.Name())
		return "", err
	}

	return file.Name(), nil
}

func extractBinaryToTemp(archivePath, assetName string) (string, error) {
	if strings.HasSuffix(assetName, ".zip") {
		return extractZipBinary(archivePath)
	}
	if strings.HasSuffix(assetName, ".tar.gz") || strings.HasSuffix(assetName, ".tgz") {
		return extractTarGzBinary(archivePath)
	}
	return "", fmt.Errorf("unsupported archive format: %s", assetName)
}

func extractTarGzBinary(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}
		if header.FileInfo().IsDir() || !isBinaryName(header.Name) {
			continue
		}
		return writeExtractedBinary(tr, header.FileInfo().Mode())
	}

	return "", errors.New("archive did not contain a modelslab binary")
}

func extractZipBinary(path string) (string, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	for _, file := range reader.File {
		if file.FileInfo().IsDir() || !isBinaryName(file.Name) {
			continue
		}
		src, err := file.Open()
		if err != nil {
			return "", err
		}
		defer src.Close()
		return writeExtractedBinary(src, file.FileInfo().Mode())
	}

	return "", errors.New("archive did not contain a modelslab binary")
}

func writeExtractedBinary(src io.Reader, mode os.FileMode) (string, error) {
	if mode == 0 {
		mode = 0755
	}

	out, err := os.CreateTemp("", "modelslab-binary-*")
	if err != nil {
		return "", err
	}
	defer out.Close()

	if _, err := io.Copy(out, src); err != nil {
		os.Remove(out.Name())
		return "", err
	}
	if err := out.Chmod(mode | 0700); err != nil {
		os.Remove(out.Name())
		return "", err
	}

	return out.Name(), nil
}

func replaceExecutable(targetPath, newBinaryPath string) error {
	info, err := os.Stat(targetPath)
	if err != nil {
		return fmt.Errorf("could not stat current executable: %w", err)
	}

	mode := info.Mode().Perm()
	if mode == 0 {
		mode = 0755
	}

	replacement := targetPath + ".new"
	if err := copyFile(newBinaryPath, replacement, mode); err != nil {
		return err
	}

	if runtime.GOOS == "windows" {
		return fmt.Errorf("downloaded update to %s, but Windows cannot replace a running executable automatically", replacement)
	}

	backup := targetPath + ".old"
	_ = os.Remove(backup)
	if err := os.Rename(targetPath, backup); err != nil {
		_ = os.Remove(replacement)
		return fmt.Errorf("could not prepare executable replacement: %w", err)
	}
	if err := os.Rename(replacement, targetPath); err != nil {
		_ = os.Rename(backup, targetPath)
		_ = os.Remove(replacement)
		return fmt.Errorf("could not install updated executable: %w", err)
	}
	_ = os.Remove(backup)

	return nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return out.Sync()
}

func isBinaryName(name string) bool {
	base := filepath.Base(name)
	return base == "modelslab" || base == "modelslab.exe"
}

func versionParts(version string) []int {
	version = NormalizeVersion(version)
	version = strings.TrimSpace(version)
	version = strings.Split(version, "-")[0]
	version = strings.Split(version, "+")[0]
	if version == "" {
		return nil
	}

	rawParts := strings.Split(version, ".")
	parts := make([]int, 0, len(rawParts))
	for _, raw := range rawParts {
		if raw == "" {
			return nil
		}
		value := 0
		for _, r := range raw {
			if r < '0' || r > '9' {
				return nil
			}
			value = value*10 + int(r-'0')
		}
		parts = append(parts, value)
	}

	return parts
}

func httpClient(client *http.Client) *http.Client {
	if client != nil {
		return client
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
