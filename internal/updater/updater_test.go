package updater

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want int
	}{
		{name: "newer patch", a: "v1.2.4", b: "1.2.3", want: 1},
		{name: "older minor", a: "1.1.9", b: "1.2.0", want: -1},
		{name: "equal with missing patch", a: "1.2", b: "1.2.0", want: 0},
		{name: "newer major", a: "2.0.0", b: "1.9.9", want: 1},
		{name: "ignores prerelease", a: "1.2.3", b: "1.2.3-next", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, CompareVersions(tt.a, tt.b))
		})
	}
}

func TestIsComparableVersion(t *testing.T) {
	assert.True(t, IsComparableVersion("v1.2.3"))
	assert.False(t, IsComparableVersion("dev"))
	assert.False(t, IsComparableVersion("1.2.3-next"))
	assert.False(t, IsComparableVersion("none"))
}

func TestSelectArchiveAssetPrefersVersionedMatch(t *testing.T) {
	assets := []ReleaseAsset{
		{Name: "modelslab_1.2.3_linux_amd64.tar.gz", BrowserDownloadURL: "https://example.com/linux"},
		{Name: "modelslab_1.2.3_darwin_arm64.tar.gz", BrowserDownloadURL: "https://example.com/darwin"},
		{Name: "checksums.txt", BrowserDownloadURL: "https://example.com/checksums"},
	}

	asset := SelectArchiveAsset(assets, "v1.2.3", "darwin", "arm64")

	require.NotNil(t, asset)
	assert.Equal(t, "modelslab_1.2.3_darwin_arm64.tar.gz", asset.Name)
}

func TestSelectArchiveAssetFallsBackToPlatformMatch(t *testing.T) {
	assets := []ReleaseAsset{
		{Name: "modelslab_darwin_arm64.tar.gz", BrowserDownloadURL: "https://example.com/darwin"},
		{Name: "modelslab_linux_amd64.deb", BrowserDownloadURL: "https://example.com/deb"},
	}

	asset := SelectArchiveAsset(assets, "1.2.3", "darwin", "arm64")

	require.NotNil(t, asset)
	assert.Equal(t, "modelslab_darwin_arm64.tar.gz", asset.Name)
}

func TestSelectArchiveAssetUsesWindowsZip(t *testing.T) {
	assets := []ReleaseAsset{
		{Name: "modelslab_1.2.3_windows_amd64.zip", BrowserDownloadURL: "https://example.com/windows"},
		{Name: "modelslab_1.2.3_windows_amd64.tar.gz", BrowserDownloadURL: "https://example.com/wrong"},
	}

	asset := SelectArchiveAsset(assets, "1.2.3", "windows", "amd64")

	require.NotNil(t, asset)
	assert.Equal(t, "modelslab_1.2.3_windows_amd64.zip", asset.Name)
}

func TestSelectChecksumAsset(t *testing.T) {
	assets := []ReleaseAsset{
		{Name: "modelslab_1.2.3_linux_amd64.tar.gz"},
		{Name: "checksums.txt", BrowserDownloadURL: "https://example.com/checksums"},
	}

	asset := SelectChecksumAsset(assets)

	require.NotNil(t, asset)
	assert.Equal(t, "checksums.txt", asset.Name)
}
