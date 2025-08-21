package github

import (
	"path/filepath"
	"testing"

	"github.com/anchore/go-make/config"
	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/log"
	"github.com/anchore/go-make/require"
	"github.com/anchore/go-make/run"
	"github.com/anchore/go-make/script"
)

func Test_UploadDownload(t *testing.T) {
	if !config.CI {
		t.Log("skipping artifact upload test in non-CI environment")
		return
	}

	tests := []struct {
		files    string
		artifact string
	}{
		{
			files:    "pr_run_artifacts.json",
			artifact: "my-artifact-name",
		},
		{
			files: "**/*_artifacts.json",
		},
		{
			files: "*",
		},
	}

	for _, tt := range tests {
		t.Run(tt.files, func(t *testing.T) {
			p := Payload() // tests run in workflow in github

			api := NewClient(Owner("testorg"), Repo("testrepo"))
			artifactId, err := api.UploadArtifactDir("testdata", UploadArtifactOption{
				ArtifactName: tt.artifact,
				Glob:         tt.files,
			})
			require.NoError(t, err)
			require.True(t, artifactId != 0)

			tmpdir := t.TempDir()

			api.DownloadArtifactDir(HeadSha(p.SHA), p.Workflow, tt.artifact, tmpdir)

			testdataFile := file.Read("testdata/pr_run_artifacts.json")
			downloadedFile := file.Read(filepath.Join(tmpdir, "pr_run_artifacts.json"))

			require.Equal(t, testdataFile, downloadedFile)

			require.True(t, file.Exists(filepath.Join(tmpdir, "testdata/empty.json")))
			require.True(t, !file.Exists(filepath.Join(tmpdir, "empty.json")))
		})
	}
}

func Test_ensureActionsArtifactNpmPackageInstalled(t *testing.T) {
	if !config.CI {
		t.Log("skipping artifact upload test in non-CI environment")
		return
	}

	ensureActionsArtifactInstalled()

	log.Log("actions/artifact out: %s", script.Run("npm list -g @actions/artifact", run.NoFail()))
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
			baseDir, err := filepath.Abs(tt.baseDir)
			require.NoError(t, err)
			for i := range tt.expected {
				tt.expected[i] = filepath.Join(baseDir, tt.expected[i])
			}

			files := renderUploadFiles(tt.baseDir, &tt.options)
			require.EqualElements(t, tt.expected, files)
		})
	}
}
