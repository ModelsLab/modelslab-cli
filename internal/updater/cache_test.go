package updater

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// roundTripFunc stands in for api.github.com.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func countingClient(calls *int, respond func() (*http.Response, error)) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		*calls++
		return respond()
	})}
}

func release(tag string) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"tag_name":"` + tag + `"}`)),
		Header:     http.Header{},
	}, nil
}

// Offline, every command used to re-run the check and wait out its timeout.
func TestCachedCheck_CachesAFailedCheck(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "update-check.json")
	calls := 0
	opts := CheckOptions{
		CurrentVersion: "0.2.0",
		HTTPClient:     countingClient(&calls, func() (*http.Response, error) { return nil, errors.New("offline") }),
	}

	_, err := CachedCheck(context.Background(), opts, cachePath, time.Hour)
	require.Error(t, err)

	_, err = CachedCheck(context.Background(), opts, cachePath, time.Hour)
	require.NoError(t, err)

	assert.Equal(t, 1, calls, "the second command must not hit the network again")
}

// A failed check must not forget an update that the last good check found.
func TestCachedCheck_FailureKeepsTheLastKnownUpdate(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "update-check.json")
	calls := 0
	online := true
	opts := CheckOptions{
		CurrentVersion: "0.2.0",
		HTTPClient: countingClient(&calls, func() (*http.Response, error) {
			if online {
				return release("v0.2.1")
			}
			return nil, errors.New("offline")
		}),
	}

	info, err := CachedCheck(context.Background(), opts, cachePath, 0)
	require.NoError(t, err)
	require.True(t, info.UpdateAvailable)

	online = false
	_, err = CachedCheck(context.Background(), opts, cachePath, 0)
	require.Error(t, err)

	info, err = CachedCheck(context.Background(), opts, cachePath, time.Hour)
	require.NoError(t, err)
	assert.True(t, info.UpdateAvailable)
	assert.Equal(t, "0.2.1", info.LatestVersion)
}

// After an update the cache describes the old binary, so it must be ignored.
func TestCachedCheck_IgnoresACacheForAnotherVersion(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "update-check.json")
	require.NoError(t, WriteCache(cachePath, Cache{CheckedAt: time.Now(), CurrentVersion: "0.1.0", LatestVersion: "0.2.0", UpdateAvailable: true}))
	calls := 0
	opts := CheckOptions{
		CurrentVersion: "0.2.0",
		HTTPClient:     countingClient(&calls, func() (*http.Response, error) { return release("v0.2.0") }),
	}

	info, err := CachedCheck(context.Background(), opts, cachePath, time.Hour)

	require.NoError(t, err)
	assert.Equal(t, 1, calls)
	assert.False(t, info.UpdateAvailable)
}

func TestReplaceExecutable_SwapsTheFileAndCleansUp(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "modelslab")
	fresh := filepath.Join(dir, "download")
	require.NoError(t, os.WriteFile(target, []byte("old"), 0o755))
	require.NoError(t, os.WriteFile(fresh, []byte("new"), 0o644))

	require.NoError(t, replaceExecutable(target, fresh))

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "new", string(got))

	info, err := os.Stat(target)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o755), info.Mode().Perm(), "the new binary keeps the old one's mode")

	assert.NoFileExists(t, target+".new")
	assert.NoFileExists(t, target+".old")
}

func TestPermissionHint(t *testing.T) {
	err := permissionHint("/usr/local/bin/modelslab", &os.PathError{Op: "open", Path: "/usr/local/bin/modelslab.new", Err: os.ErrPermission})

	assert.ErrorIs(t, err, os.ErrPermission)
	assert.Contains(t, err.Error(), "/usr/local/bin")

	other := errors.New("disk full")
	assert.Same(t, other, permissionHint("/usr/local/bin/modelslab", other))
}
