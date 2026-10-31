package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/creativeprojects/go-selfupdate"
)

const newBinary = "new sermo binary"

// fakeSource is a selfupdate.Source that serves releases from memory.
type fakeSource struct {
	releases []selfupdate.SourceRelease
	assets   map[int64][]byte
}

func (f *fakeSource) ListReleases(context.Context, selfupdate.Repository) ([]selfupdate.SourceRelease, error) {
	return f.releases, nil
}

func (f *fakeSource) DownloadReleaseAsset(_ context.Context, _ *selfupdate.Release, id int64) (io.ReadCloser, error) {
	data, ok := f.assets[id]
	if !ok {
		return nil, fmt.Errorf("asset %d not found", id)
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

type fakeRelease struct {
	tag    string
	url    string
	assets []selfupdate.SourceAsset
}

func (r fakeRelease) GetID() int64                        { return 1 }
func (r fakeRelease) GetTagName() string                  { return r.tag }
func (r fakeRelease) GetDraft() bool                      { return false }
func (r fakeRelease) GetPrerelease() bool                 { return false }
func (r fakeRelease) GetPublishedAt() time.Time           { return time.Time{} }
func (r fakeRelease) GetReleaseNotes() string             { return "" }
func (r fakeRelease) GetName() string                     { return r.tag }
func (r fakeRelease) GetURL() string                      { return r.url }
func (r fakeRelease) GetAssets() []selfupdate.SourceAsset { return r.assets }

type fakeAsset struct {
	id   int64
	name string
	size int
}

func (a fakeAsset) GetID() int64                  { return a.id }
func (a fakeAsset) GetName() string               { return a.name }
func (a fakeAsset) GetSize() int                  { return a.size }
func (a fakeAsset) GetBrowserDownloadURL() string { return "https://example.invalid/" + a.name }

// tarGz returns a tar.gz archive with one file.
func tarGz(t *testing.T, name, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{Name: name, Mode: 0o755, Size: int64(len(content))}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// release returns a fake source with one release in the GoReleaser layout.
// If badSum is true, checksums.txt holds a wrong hash for the archive.
func release(t *testing.T, tag string, badSum bool) *fakeSource {
	t.Helper()
	version := tag[1:]
	archiveName := fmt.Sprintf("sermo_%s_%s_%s.tar.gz", version, runtime.GOOS, runtime.GOARCH)
	archive := tarGz(t, "sermo", newBinary)
	sum := sha256.Sum256(archive)
	if badSum {
		sum = sha256.Sum256([]byte("other content"))
	}
	sums := []byte(fmt.Sprintf("%x  %s\n", sum, archiveName))
	return &fakeSource{
		releases: []selfupdate.SourceRelease{fakeRelease{
			tag: tag,
			url: "https://github.com/" + Repo + "/releases/tag/" + tag,
			assets: []selfupdate.SourceAsset{
				fakeAsset{id: 10, name: archiveName, size: len(archive)},
				fakeAsset{id: 11, name: "checksums.txt", size: len(sums)},
			},
		}},
		assets: map[int64][]byte{10: archive, 11: sums},
	}
}

// target writes a fake old binary and returns an updater that replaces it.
func target(t *testing.T, src selfupdate.Source) (*updater, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sermo")
	if err := os.WriteFile(path, []byte("old sermo binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	return &updater{source: src, executable: func() (string, error) { return path, nil }}, path
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestCheckNewerRelease(t *testing.T) {
	u, path := target(t, release(t, "v1.3.0", false))

	res, err := u.check(context.Background(), "1.2.0")
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	want := Result{
		Current:    "1.2.0",
		Latest:     "1.3.0",
		ReleaseURL: "https://github.com/" + Repo + "/releases/tag/v1.3.0",
	}
	if res != want {
		t.Fatalf("check = %+v, want %+v", res, want)
	}
	if !res.Newer() {
		t.Fatal("Newer() = false, want true")
	}
	if got := readFile(t, path); got != "old sermo binary" {
		t.Fatalf("check changed the binary: %q", got)
	}
}

func TestApplyNewerRelease(t *testing.T) {
	u, path := target(t, release(t, "v1.3.0", false))

	res, err := u.apply(context.Background(), "v1.2.0")
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Updated || res.Latest != "1.3.0" {
		t.Fatalf("apply = %+v, want Updated to 1.3.0", res)
	}
	if got := readFile(t, path); got != newBinary {
		t.Fatalf("binary = %q, want %q", got, newBinary)
	}
}

func TestAlreadyLatest(t *testing.T) {
	u, path := target(t, release(t, "v1.3.0", false))

	res, err := u.check(context.Background(), "1.3.0")
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if res.Newer() {
		t.Fatalf("Newer() = true for %+v", res)
	}

	res, err = u.apply(context.Background(), "1.3.0")
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Updated || res.Latest != "1.3.0" {
		t.Fatalf("apply = %+v, want no update", res)
	}
	if got := readFile(t, path); got != "old sermo binary" {
		t.Fatalf("apply changed the binary: %q", got)
	}
}

func TestNoRelease(t *testing.T) {
	u, _ := target(t, &fakeSource{})

	res, err := u.check(context.Background(), "1.0.0")
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if res.Latest != "" || res.Newer() {
		t.Fatalf("check = %+v, want empty Latest", res)
	}

	res, err = u.apply(context.Background(), "1.0.0")
	if err != nil || res.Updated {
		t.Fatalf("apply = %+v, %v; want no update and no error", res, err)
	}
}

func TestDevBuild(t *testing.T) {
	for _, current := range []string{"dev", "1.2", "abc"} {
		t.Run(current, func(t *testing.T) { checkDevBuild(t, current) })
	}
}

func checkDevBuild(t *testing.T, current string) {
	u, path := target(t, release(t, "v1.3.0", false))

	res, err := u.check(context.Background(), current)
	if !errors.Is(err, ErrDevBuild) {
		t.Fatalf("check error = %v, want ErrDevBuild", err)
	}
	if res.Latest != "1.3.0" || res.Newer() {
		t.Fatalf("check = %+v, want Latest 1.3.0 and Newer false", res)
	}

	res, err = u.apply(context.Background(), current)
	if !errors.Is(err, ErrDevBuild) {
		t.Fatalf("apply error = %v, want ErrDevBuild", err)
	}
	if res.Updated {
		t.Fatal("apply reported an update for a development build")
	}
	if got := readFile(t, path); got != "old sermo binary" {
		t.Fatalf("apply changed the binary: %q", got)
	}
}

func TestApplyChecksumMismatch(t *testing.T) {
	u, path := target(t, release(t, "v1.3.0", true))

	res, err := u.apply(context.Background(), "1.2.0")
	if !errors.Is(err, selfupdate.ErrChecksumValidationFailed) {
		t.Fatalf("apply error = %v, want ErrChecksumValidationFailed", err)
	}
	if res.Updated {
		t.Fatal("apply reported an update after a checksum failure")
	}
	if got := readFile(t, path); got != "old sermo binary" {
		t.Fatalf("apply changed the binary: %q", got)
	}
}
