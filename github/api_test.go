package github

import (
	"maps"
	"net/http"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/require"
)

func Test_ListWorkflowRunsSha(t *testing.T) {
	defer require.Test(t)

	url := require.Server(t, fileMap(map[string]string{
		"/repos/testorg/testrepo/actions/runs?head_sha=7befde0c7a4cdf5e9fd4cef19212c799fdca29a5": "pr_run.json",
	}), removeCommonQueryParams)

	testrepo := Api{
		Token:   "my-token",
		BaseURL: url,
		Owner:   "testorg",
		Repo:    "testrepo",
	}

	runs := testrepo.ListWorkflowRuns(HeadSha("7befde0c7a4cdf5e9fd4cef19212c799fdca29a5"))
	require.Equal(t, 3, len(runs))
}

func Test_ListWorkflowRunsBranch(t *testing.T) {
	defer require.Test(t)

	url := require.Server(t, fileMap(map[string]string{
		"/repos/testorg/testrepo/actions/runs?branch=main": "pr_run.json",
	}), removeCommonQueryParams)

	testrepo := Api{
		Token:   "my-token",
		BaseURL: url,
		Owner:   "testorg",
		Repo:    "testrepo",
	}

	runs := testrepo.ListWorkflowRuns(Branch("main"))
	require.Equal(t, 3, len(runs))
}

func Test_ListArtifactsForCommit(t *testing.T) {
	defer require.Test(t)

	url := require.Server(t, fileMap(map[string]string{
		"/repos/testorg/testrepo/actions/runs?head_sha=7befde0c7a4cdf5e9fd4cef19212c799fdca29a5":           "pr_run.json",
		"/repos/testorg/testrepo/actions/runs/16972954485/artifacts":                                       "pr_run_artifacts.json",
		"/repos/testorg/testrepo/actions/runs/16972954485/artifacts?name=not-linux-build_linux_arm64_v8.0": "empty.json",
		"/repos/testorg/testrepo/actions/runs/16972954485/artifacts?name=linux-build_linux_arm64_v8.0":     "pr_run_artifacts.json",
		"/repos/testorg/testrepo/actions/runs/16972954485/artifacts?":                                      "pr_run_artifacts.json",
	}), removeCommonQueryParams)

	testrepo := Api{
		Token:   "my-token",
		BaseURL: url,
		Owner:   "testorg",
		Repo:    "testrepo",
	}

	artifacts := testrepo.ListArtifactsForWorkflow(HeadSha("7befde0c7a4cdf5e9fd4cef19212c799fdca29a5"), "Not Validations", "linux-build_linux_arm64_v8.0")
	require.Equal(t, 0, len(artifacts))

	artifacts = testrepo.ListArtifactsForWorkflow(HeadSha("7befde0c7a4cdf5e9fd4cef19212c799fdca29a5"), "Validations", "not-linux-build_linux_arm64_v8.0")
	require.Equal(t, 0, len(artifacts))

	artifacts = testrepo.ListArtifactsForWorkflow(HeadSha("7befde0c7a4cdf5e9fd4cef19212c799fdca29a5"), "Validations", "linux-build_linux_arm64_v8.0")
	require.Equal(t, 1, len(artifacts))

	artifacts = testrepo.ListArtifactsForWorkflow(HeadSha("7befde0c7a4cdf5e9fd4cef19212c799fdca29a5"), "Validations", "*")
	require.Equal(t, 8, len(artifacts))
}

func Test_ArtifactDownload(t *testing.T) {
	defer require.Test(t)

	url := require.Server(t, multiMap(fileMap(map[string]string{
		"/repos/testorg/testrepo/actions/runs?head_sha=7befde0c7a4cdf5e9fd4cef19212c799fdca29a5":       "pr_run.json",
		"/repos/testorg/testrepo/actions/runs/16972954485/artifacts":                                   "pr_run_artifacts.json",
		"/repos/testorg/testrepo/actions/runs/16972954485/artifacts?name=linux-build_linux_arm64_v8.0": "pr_run_artifacts.json",
		"/repos/testorg/testrepo/actions/runs/16972954485/artifacts?":                                  "pr_run_artifacts.json",
	}), map[string]any{
		// https://api.com/repos/OWNER/REPO/actions/artifacts/ARTIFACT_ID/ARCHIVE_FORMAT
		// 3767901755 = "linux-build_linux_arm64_v8.0"
		"/repos/testorg/testrepo/actions/artifacts/3767901755/zip": func(writer http.ResponseWriter, request *http.Request) {
			_, err := writer.Write(require.Zip(map[string][]byte{
				"a_file.json": []byte(`{"something":true}`),
			}))
			require.NoError(t, err)
		},
	}), removeCommonQueryParams)

	testrepo := Api{
		Token:   "my-token",
		BaseURL: url,
		Owner:   "testorg",
		Repo:    "testrepo",
	}

	tmpdir := t.TempDir()
	testrepo.DownloadArtifactDir(HeadSha("7befde0c7a4cdf5e9fd4cef19212c799fdca29a5"), "Validations", "linux-build_linux_arm64_v8.0", tmpdir)

	filePath := filepath.Join(tmpdir, "a_file.json")
	require.True(t, file.Exists(filePath))
	require.Equal(t, `{"something":true}`, file.Read(filePath))
}

func Test_nameMatches(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		testName string
		want     bool
	}{
		{
			name:     "exact match",
			pattern:  "linux-build_linux_arm64_v8.0",
			testName: "linux-build_linux_arm64_v8.0",
			want:     true,
		},
		{
			name:     "wildcard match",
			pattern:  "*_linux_*",
			testName: "linux-build_linux_arm64_v8.0",
			want:     true,
		},
		{
			name:     "wildcard all match",
			pattern:  "*",
			testName: "linux-build_linux_arm64_v8.0",
			want:     true,
		},
		{
			name:     "no match",
			pattern:  "windows-build",
			testName: "linux-build_linux_arm64_v8.0",
			want:     false,
		},
		{
			name:     "empty pattern",
			pattern:  "",
			testName: "linux-build_linux_arm64_v8.0",
			want:     false,
		},
		{
			name:     "empty test name",
			pattern:  "linux-build_linux_arm64_v8.0",
			testName: "",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nameMatches(tt.pattern, tt.testName)
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_isExactMatch(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		want    bool
	}{
		{
			name:    "simple pattern without wildcards",
			pattern: "test-pattern",
			want:    true,
		},
		{
			name:    "pattern with asterisk",
			pattern: "test*pattern",
			want:    false,
		},
		{
			name:    "empty pattern",
			pattern: "",
			want:    true,
		},
		{
			name:    "only wildcard pattern",
			pattern: "*",
			want:    false,
		},
		{
			name:    "pattern with question mark",
			pattern: "test?pattern",
			want:    false,
		},
		{
			name:    "pattern with brackets",
			pattern: "test[a-z]pattern",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isExactMatch(tt.pattern)
			require.Equal(t, tt.want, got)
		})
	}
}

func removeCommonQueryParams(s string) string {
	return regexp.MustCompile(`[?&](sort|direction|per_page)=[^&]+`).ReplaceAllString(s, "")
}

func multiMap(values ...map[string]any) map[string]any {
	for i := 0; i < len(values); i++ {
		maps.Copy(values[0], values[i])
	}
	return values[0]
}

func fileMap(files map[string]string) map[string]any {
	out := map[string]any{}
	for path, fileName := range files {
		fileName, _ = filepath.Abs("testdata/" + fileName)
		out[path] = func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			lang.Return(w.Write([]byte(file.Read(fileName))))
		}
	}
	return out
}
