package gotest

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"

	. "github.com/anchore/go-make"
	"github.com/anchore/go-make/config"
	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/github"
	"github.com/anchore/go-make/gomod"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/log"
	"github.com/anchore/go-make/run"
)

// coverageTotalRE matches the "total:" line emitted by `go tool cover -func`.
var coverageTotalRE = regexp.MustCompile(`total:[^\n%]+?(\d+\.\d+)%`)

// Tasks creates a test task that runs Go tests with coverage reporting.
// The task hooks into the "test" label, so it runs whenever "make test" is called.
// By default, it runs tests for all packages with coverage enabled and race detection
// in CI environments.
//
// Example:
//
//	gotest.Tasks()                           // default: run all tests
//	gotest.Tasks(gotest.Name("integration")) // named test suite
//	gotest.Tasks(gotest.ExcludeGlob("**/test/**")) // exclude paths
func Tasks(options ...Option) Task {
	cfg := defaultConfig()
	for _, opt := range options {
		opt(&cfg)
	}

	// fail fast at task construction so misconfiguration surfaces before any tests run.
	if cfg.CoverageThreshold > 0 && !cfg.Coverage {
		lang.Throw(fmt.Errorf("coverage threshold requires coverage to be enabled"))
	}

	return Task{
		Name:        cfg.Name,
		Description: fmt.Sprintf("run %s tests", cfg.Name),
		RunsOn:      Deps("test"),
		Run:         func() { runTests(&cfg) },
	}
}

func runTests(cfg *Config) {
	start := time.Now()

	coverageFile, cleanup := resolveCoverageFile(cfg)
	defer cleanup()

	args := buildTestArgs(cfg, coverageFile)
	Run("go", run.Args(args...), run.Stdout(os.Stderr), run.Env("GODEBUG", "dontfreezetheworld=1"))
	Log("Done running %s tests in %v", cfg.Name, time.Since(start))

	if coverageFile == "" {
		return
	}
	processCoverage(cfg, coverageFile)
	uploadCoverage(coverageFile)
}

// buildTestArgs assembles the `go test` argument list. coverageFile is the resolved
// cover profile path (may be empty when coverage is disabled or temp-dir creation failed).
func buildTestArgs(cfg *Config, coverageFile string) []string {
	args := Deps("test")
	if cfg.Verbose {
		args = append(args, "-v")
	}
	if cfg.RunFilter != "" {
		args = append(args, "-run", cfg.RunFilter)
	}
	args = append(args, selectPackages(cfg.IncludeGlob, cfg.ExcludeGlob)...)

	tags := cfg.Tags
	if cfg.Coverage {
		args = append(args, "-coverprofile", coverageFile)
		args = append(args, "-covermode=atomic", "-coverpkg=./...")
		tags = append(tags, "coverage")
	}
	if len(tags) > 0 {
		args = append(args, "-tags="+strings.Join(tags, ","))
	}
	if cfg.Race {
		args = append(args, "-race")
	}
	return args
}

// resolveCoverageFile returns the coverage file path and a cleanup func. When the caller
// did not supply a path, a temp dir is created and its removal is deferred via the returned
// func. Returns empty string when coverage is disabled.
func resolveCoverageFile(cfg *Config) (string, func()) {
	noop := func() {}
	if !cfg.Coverage {
		return "", noop
	}
	if cfg.CoverageFile != "" {
		return cfg.CoverageFile, noop
	}
	coverageDir, err := os.MkdirTemp(config.TmpDir, "cover-dir-")
	if err != nil {
		return "", noop
	}
	cleanup := func() { log.Error(os.RemoveAll(coverageDir)) }
	absDir, err := filepath.Abs(coverageDir)
	if err != nil {
		return "", cleanup
	}
	return filepath.Join(absDir, "cover.out"), cleanup
}

// processCoverage runs `go tool cover -func`, logs the report, and enforces any threshold.
func processCoverage(cfg *Config, coverageFile string) {
	if cfg.CoverageFile == "" {
		// only scrub the cover profile when we own the file (temp dir). When the caller
		// supplied an explicit path, leave their file untouched. Drops entries for files
		// that no longer exist (e.g. renamed/deleted between cached test runs and now)
		// so `go tool cover -func` doesn't fail trying to open them.
		scrubMissingFilesFromCoverProfile(coverageFile)
	}
	report := Run("go tool cover", run.Args("-func", coverageFile), run.Quiet())
	rawPct, coverage, found := parseCoveragePercent(report)
	logCoverageReport(report, rawPct, found, cfg.Verbose)

	if err := enforceCoverageThreshold(rawPct, coverage, found, cfg.CoverageThreshold); err != nil {
		lang.Throw(err)
	}
}

func logCoverageReport(report, rawPct string, found, verbose bool) {
	switch {
	case found && verbose:
		Log(" -------------- Coverage Report -------------- ")
		Log(report)
	case found:
		Log("Coverage: %s%%", rawPct)
	default:
		// parse failures always log the report and an error, regardless of verbose,
		// so the user can see what went wrong.
		Log(" -------------- Coverage Report -------------- ")
		Log(report)
		log.Error(fmt.Errorf("unable to find coverage percentage in report"))
	}
}

// uploadCoverage best-effort uploads the cover profile as a GitHub Actions artifact on linux.
// Errors are intentionally swallowed (logged at debug) so they don't fail the task.
func uploadCoverage(coverageFile string) {
	if !config.CI || config.OS != "linux" {
		return
	}
	err := lang.Catch(func() {
		dir := filepath.Dir(coverageFile)
		lang.Return(github.NewClient().UploadArtifactDir(dir, github.UploadArtifactOption{
			ArtifactName: "code-coverage",
			Overwrite:    false, // we only need one, failures are logged but ignored
			Files:        []string{coverageFile},
		}))
	})
	if err != nil {
		log.Debug("error uploading coverage file: %v", err)
	}
}

// Config holds configuration for the test task.
type Config struct {
	// Name identifies this test suite (e.g., "unit", "integration"). Used in logs.
	Name string
	// IncludeGlob specifies which packages to test (default: "./...").
	IncludeGlob string
	// ExcludeGlob filters out packages matching this pattern.
	ExcludeGlob string
	// Verbose enables verbose test output (-v flag).
	Verbose bool
	// Coverage enables coverage reporting (default: true).
	Coverage bool
	// CoverageFile specifies where to write coverage data. If empty, uses temp file.
	CoverageFile string
	// CoverageThreshold is the minimum total coverage percentage required to pass.
	// If > 0, the task fails when measured coverage falls below this value, or
	// when the coverage percentage cannot be parsed from the report. Requires
	// Coverage to be enabled.
	CoverageThreshold float64
	// Race enables race detector (-race flag). Defaults to true in CI on non-Windows.
	Race bool
	// Tags specifies build tags to use during testing.
	Tags []string
	// RunFilter limits which tests run (-run flag pattern).
	RunFilter string
}

func defaultConfig() Config {
	return Config{
		Name:        "unit",
		IncludeGlob: "./...",
		Coverage:    true,
		Race:        config.CI && !config.Windows,
	}
}

// Option is a functional option for configuring test tasks.
type Option func(*Config)

// Name sets the test suite name (used in log output and task naming).
func Name(name string) Option {
	return func(c *Config) {
		c.Name = name
	}
}

// IncludeGlob sets which packages to include in testing (default: "./...").
func IncludeGlob(packages string) Option {
	return func(c *Config) {
		c.IncludeGlob = packages
	}
}

// ExcludeGlob sets a pattern to exclude packages from testing.
func ExcludeGlob(packages string) Option {
	return func(c *Config) {
		c.ExcludeGlob = packages
	}
}

// Verbose enables verbose test output.
func Verbose() Option {
	return func(c *Config) {
		c.Verbose = true
	}
}

// NoCoverage disables coverage reporting.
func NoCoverage() Option {
	return func(c *Config) {
		c.Coverage = false
	}
}

// CoverageThreshold sets the minimum total coverage percentage required to pass.
// Values <= 0 disable the check. If measured coverage falls below this value,
// the task fails. Example: gotest.CoverageThreshold(80) requires at least 80%.
func CoverageThreshold(percent float64) Option {
	return func(c *Config) {
		c.CoverageThreshold = percent
	}
}

// parseCoveragePercent extracts the "total: ... NN.N%" coverage percentage from
// the output of `go tool cover -func`. Returns the raw matched string (for
// display, to avoid rounding the value the tool emitted), the parsed float, and
// true on success. The regex's `(\d+\.\d+)` capture guarantees a valid float, so
// strconv.ParseFloat cannot fail on a successful match.
func parseCoveragePercent(report string) (string, float64, bool) {
	match := coverageTotalRE.FindStringSubmatch(report)
	if len(match) < 2 {
		return "", 0, false
	}
	pct, _ := strconv.ParseFloat(match[1], 64)
	return match[1], pct, true
}

// enforceCoverageThreshold returns an error if the threshold is set (> 0) and
// either the coverage value could not be parsed or it falls below the threshold.
// A non-positive threshold disables the check. rawPct is the tool-emitted string
// (used for display so we don't round the value the user is reading).
func enforceCoverageThreshold(rawPct string, coverage float64, found bool, threshold float64) error {
	if threshold <= 0 {
		return nil
	}
	if !found {
		return fmt.Errorf("coverage threshold %g%% set but coverage percentage could not be determined", threshold)
	}
	if coverage < threshold {
		return fmt.Errorf("coverage %s%% is below threshold %g%%", rawPct, threshold)
	}
	return nil
}

// Tags adds build tags to use during testing.
func Tags(tags ...string) Option {
	return func(c *Config) {
		c.Tags = append(c.Tags, tags...)
	}
}

// RunFilter sets a -run pattern to limit which tests execute.
func RunFilter(filter string) Option {
	return func(c *Config) {
		c.RunFilter = filter
	}
}

// scrubMissingFilesFromCoverProfile rewrites the given cover profile to remove rows that point
// at source files which no longer exist on disk. This happens with `-coverpkg=./...` plus the
// Go test cache: cached test results from packages that didn't change still replay coverage
// data for files (in other packages) that have since been renamed or deleted. `go tool cover
// -func` would otherwise fail with "open ...: no such file or directory" on those entries.
func scrubMissingFilesFromCoverProfile(coverageFile string) {
	mod := gomod.Read()
	if mod == nil || mod.Module == nil {
		return
	}
	modPath := mod.Module.Mod.Path
	modFile := file.FindParent(file.Cwd(), "go.mod")
	if modFile == "" {
		return
	}
	modRoot := filepath.Dir(modFile)

	contents, err := os.ReadFile(coverageFile)
	if err != nil {
		log.Debug("unable to read cover profile for scrubbing: %v", err)
		return
	}

	fileExists := func(rel string) bool {
		_, statErr := os.Stat(filepath.Join(modRoot, rel)) //nolint:gosec // G703: path derived from in-module cover profile rows
		return statErr == nil
	}

	kept, dropped := scrubCoverLines(strings.Split(string(contents), "\n"), modPath, fileExists)
	if len(dropped) == 0 {
		return
	}

	for _, p := range dropped {
		log.Debug("dropping stale cover profile entries for missing file: %s", p)
	}

	if err := os.WriteFile(coverageFile, []byte(strings.Join(kept, "\n")), 0o600); err != nil {
		log.Debug("unable to rewrite scrubbed cover profile: %v", err)
	}
}

// scrubCoverLines filters cover profile lines, dropping any in-module rows whose source file
// is reported missing by fileExists. Non-module rows, the mode header, blanks, and anything
// unparseable are kept as-is. Existence checks are memoized per source path.
func scrubCoverLines(lines []string, modPath string, fileExists func(rel string) bool) (kept, dropped []string) {
	exists := map[string]bool{}
	kept = make([]string, 0, len(lines))
	for _, line := range lines {
		if line == "" || strings.HasPrefix(line, "mode:") {
			kept = append(kept, line)
			continue
		}
		colon := strings.Index(line, ":")
		if colon < 0 {
			kept = append(kept, line)
			continue
		}
		srcPath := line[:colon]
		if !strings.HasPrefix(srcPath, modPath+"/") {
			kept = append(kept, line)
			continue
		}
		ok, seen := exists[srcPath]
		if !seen {
			ok = fileExists(strings.TrimPrefix(srcPath, modPath+"/"))
			exists[srcPath] = ok
			if !ok {
				dropped = append(dropped, srcPath)
			}
		}
		if ok {
			kept = append(kept, line)
		}
	}
	return kept, dropped
}

func selectPackages(include, exclude string) []string {
	if exclude == "" {
		return []string{include}
	}

	// TODO: cannot use {{"{{.Dir}}"}} as a -f arg, and escaping is not working
	absDirs := Run(`go list`, run.Args(include))

	// split by newline, and use relpath with cwd to get the non-absolute path
	var dirs []string
	cwd := file.Cwd()
	for _, dir := range strings.Split(absDirs, "\n") {
		p, err := filepath.Rel(cwd, dir)
		if err != nil {
			dirs = append(dirs, dir)
			continue
		}
		dirs = append(dirs, p)
	}

	var final []string
	for _, dir := range dirs {
		matched, err := doublestar.Match(exclude, dir)
		if err != nil {
			final = append(final, dir)
			continue
		}
		if !matched {
			final = append(final, dir)
		}
	}
	return final
}
