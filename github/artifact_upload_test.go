//go:build nixos

//this feature does not work on external runners

package github

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	. "github.com/anchore/go-make"
	"github.com/anchore/go-make/config"
	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/log"
	"github.com/anchore/go-make/require"
	"github.com/anchore/go-make/run"
)

func Test_UploadWorkflowArtifact(t *testing.T) {
	if !config.CI || skipPlatform() {
		t.Log("skipping in non-CI environment")
		return
	}
	defer require.Test(t)

	tmp := t.TempDir()
	err := os.WriteFile(filepath.Join(tmp, "my-file.txt"), []byte("test-upload"), 0o644)
	require.NoError(t, err)

	api := NewClient()

	_, err = api.UploadArtifactDir(tmp, UploadArtifactOption{
		ArtifactName: "test-artifact" + MatrixSuffix,
		Overwrite:    true,
	})
	require.NoError(t, err)
}

func _Test_DownloadBranchArtifactDir(t *testing.T) {
	if !config.CI || skipPlatform() {
		t.Log("skipping artifact upload test in non-CI environment")
		return
	}
	defer require.Test(t)

	tmp := t.TempDir()
	targetPath, err := filepath.Abs(tmp)
	require.NoError(t, err)

	api := NewClient()

	err = api.DownloadBranchArtifactDir("main", "Validations", "test-artifact"+MatrixSuffix, targetPath)
	require.NoError(t, err)

	require.Equal(t, "test-upload", file.Read(filepath.Join(tmp, "my-file.txt")))
}

func Test_UploadDownload(t *testing.T) {
	if !config.CI || skipPlatform() {
		t.Log("skipping in non-CI environment")
		return
	}

	tests := []struct {
		files    string
		artifact string
		expect   []string
	}{
		{
			files:    "pr_run_artifacts.json",
			artifact: "pr-run-artifacts-1" + MatrixSuffix,
			expect: []string{
				"pr_run_artifacts.json",
			},
		},
		{
			files:    "**/*_artifacts.json",
			artifact: "pr-run-artifacts-2" + MatrixSuffix,
			expect: []string{
				"pr_run_artifacts.json",
			},
		},
		{
			files: "*",
			expect: []string{
				"pr_run_artifacts.json",
				"pr_run.json",
				"empty.json",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.files, func(t *testing.T) {
			defer require.Test(t)

			p := Payload() // tests run in workflow in github

			api := NewClient()
			artifactId, err := api.UploadArtifactDir("testdata", UploadArtifactOption{
				ArtifactName: tt.artifact,
				Glob:         tt.files,
				Overwrite:    false,
			})
			require.NoError(t, err)
			require.True(t, artifactId != 0)

			if tt.artifact == "" {
				tt.artifact = "testdata" + MatrixSuffix
			}

			if config.Windows {
				const waitTime = 10 * time.Second
				const totalTime = 2 * time.Minute
				log.Info("waiting on Windows for up to %v minutes for artifact to be available...", totalTime/time.Minute)
				for i := time.Duration(0); i < totalTime; i += waitTime {
					artifacts, _ := api.ListArtifactsForWorkflowRun(p.RunID, tt.artifact)
					if len(artifacts) > 0 {
						break
					}
					time.Sleep(waitTime)
				}
			}

			tmpdir := t.TempDir()

			err = api.DownloadArtifactDir(p.RunID, tt.artifact, tmpdir)
			require.NoError(t, err)

			file.InDir(tmpdir, func() {
				file.LogWorkdir()
			})

			testdataFile := file.Read("testdata/pr_run_artifacts.json")
			downloadedFile := file.Read(filepath.Join(tmpdir, "pr_run_artifacts.json"))

			require.Equal(t, testdataFile, downloadedFile)

			for _, f := range tt.expect {
				require.True(t, file.Exists(filepath.Join(tmpdir, f)))
			}

			if !slices.Contains(tt.expect, "empty.json") {
				require.True(t, !file.Exists(filepath.Join(tmpdir, "empty.json")))
			}
		})
	}
}

func Test_ensureActionsArtifactNpmPackageInstalled(t *testing.T) {
	if !config.CI || skipPlatform() {
		t.Log("skipping in non-CI environment")
		return
	}
	defer require.Test(t)

	testDir := t.TempDir()
	require.SetAndRestore(t, &config.RootDir, testDir)

	ensureActionsArtifactInstalled(testDir)

	file.InDir(testDir, func() {
		log.Info("actions/artifact out: %s", Run("npm list @actions/artifact", run.NoFail()))
	})
}

func Test_renderUploadFiles(t *testing.T) {
	tests := []struct {
		name     string
		baseDir  string
		options  UploadArtifactOption
		expected []string
	}{
		{
			name:    "direct",
			baseDir: "testdata",
			options: UploadArtifactOption{
				Files: []string{
					"pr_run_artifacts.json",
				},
			},
			expected: []string{"pr_run_artifacts.json"},
		},
		{
			name:    "glob",
			baseDir: "testdata",
			options: UploadArtifactOption{
				Glob: "**/empty.json",
			},
			expected: []string{"empty.json"},
		},
		{
			name: "both",
			options: UploadArtifactOption{
				Glob: "**/*_run.json",
				Files: []string{
					"testdata/pr_run_artifacts.json",
				},
			},
			expected: []string{"testdata/pr_run.json", "testdata/pr_run_artifacts.json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer require.Test(t)

			baseDir, err := filepath.Abs(tt.baseDir)
			require.NoError(t, err)
			for i := range tt.expected {
				tt.expected[i] = filepath.Join(baseDir, filepath.FromSlash(tt.expected[i]))
			}

			files := listMatchingFiles(tt.baseDir, &tt.options)
			require.EqualElements(t, tt.expected, files)
		})
	}
}

func skipPlatform() bool {
	if !config.Windows {
		return false
	}
	// only run these tests on one windows runner, `windows-latest-stable` for now
	s := matrixSuffix()
	return !(strings.Contains(s, "latest") && strings.Contains(s, "stable"))
}
