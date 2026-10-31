// Package update replaces the sermo binary with the latest GitHub release.
package update

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/creativeprojects/go-selfupdate"
)

// Repo is the GitHub repository that publishes the releases.
const Repo = "albertgmz/sermo-cli"

// checksumsFile is the GoReleaser checksum asset of each release.
const checksumsFile = "checksums.txt"

// ErrDevBuild tells that the current version is not a release version.
var ErrDevBuild = errors.New("update: this is a development build, self-update is not possible")

// Result gives the outcome of Check or Apply.
type Result struct {
	Current, Latest string
	Updated         bool
	ReleaseURL      string
}

// Newer reports whether Latest is a release that is newer than Current.
func (r Result) Newer() bool {
	cur, err := parseRelease(r.Current)
	if err != nil {
		return false
	}
	latest, err := parseRelease(r.Latest)
	if err != nil {
		return false
	}
	return latest.GreaterThan(cur)
}

// Check finds the latest release. It does not change the binary.
// Latest is empty when the repository has no release.
// For a development build, Check returns the result and ErrDevBuild.
func Check(ctx context.Context, current string) (Result, error) {
	u, err := newUpdater()
	if err != nil {
		return Result{Current: current}, err
	}
	return u.check(ctx, current)
}

// Apply downloads the latest release, verifies its checksum and replaces the
// running executable when the release is newer than current.
// For a development build, Apply returns ErrDevBuild and changes nothing.
func Apply(ctx context.Context, current string) (Result, error) {
	u, err := newUpdater()
	if err != nil {
		return Result{Current: current}, err
	}
	return u.apply(ctx, current)
}

type updater struct {
	source     selfupdate.Source
	executable func() (string, error)
}

func newUpdater() (*updater, error) {
	src, err := selfupdate.NewGitHubSource(selfupdate.GitHubConfig{})
	if err != nil {
		return nil, fmt.Errorf("update: %w", err)
	}
	return &updater{source: src, executable: executablePath}, nil
}

func (u *updater) check(ctx context.Context, current string) (Result, error) {
	res, _, _, err := u.latest(ctx, current)
	if err != nil {
		return res, err
	}
	if _, err := parseRelease(current); err != nil {
		return res, ErrDevBuild
	}
	return res, nil
}

func (u *updater) apply(ctx context.Context, current string) (Result, error) {
	res := Result{Current: current}
	if _, err := parseRelease(current); err != nil {
		return res, ErrDevBuild
	}
	res, up, rel, err := u.latest(ctx, current)
	if err != nil || rel == nil || !res.Newer() {
		return res, err
	}
	exe, err := u.executable()
	if err != nil {
		return res, fmt.Errorf("update: find executable: %w", err)
	}
	if err := up.UpdateTo(ctx, rel, exe); err != nil {
		return res, fmt.Errorf("update: install %s: %w", res.Latest, err)
	}
	res.Updated = true
	return res, nil
}

// latest finds the latest release for this OS and architecture.
// The release is nil when the repository has no matching release.
func (u *updater) latest(ctx context.Context, current string) (Result, *selfupdate.Updater, *selfupdate.Release, error) {
	res := Result{Current: current}
	up, err := selfupdate.NewUpdater(selfupdate.Config{
		Source:    u.source,
		Validator: &selfupdate.ChecksumValidator{UniqueFilename: checksumsFile},
	})
	if err != nil {
		return res, nil, nil, fmt.Errorf("update: %w", err)
	}
	rel, found, err := up.DetectLatest(ctx, selfupdate.ParseSlug(Repo))
	if err != nil {
		return res, nil, nil, fmt.Errorf("update: find latest release: %w", err)
	}
	if !found {
		return res, up, nil, nil
	}
	res.Latest = rel.Version()
	res.ReleaseURL = rel.URL
	return res, up, rel, nil
}

// parseRelease accepts a strict semantic version with an optional "v" prefix.
func parseRelease(v string) (*semver.Version, error) {
	return semver.StrictNewVersion(strings.TrimPrefix(v, "v"))
}

// executablePath returns the path of the running binary without symbolic links.
func executablePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}
