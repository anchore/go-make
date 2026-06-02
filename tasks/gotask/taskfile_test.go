package gotask

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anchore/go-make/require"
)

func TestDiscoverTasks(t *testing.T) {
	tests := []struct {
		name string
		// files maps a path (relative to a temp dir) to its YAML contents; the
		// "Taskfile.yaml" entry is the root passed to discoverTasks.
		files map[string]string
		want  []taskInfo
	}{
		{
			name: "extracts names and descriptions, sorted",
			files: map[string]string{
				"Taskfile.yaml": `
version: '3'
tasks:
  test:
    desc: run tests
  build:
    desc: build the binary
`,
			},
			want: []taskInfo{
				{Name: "build", Desc: "build the binary"},
				{Name: "test", Desc: "run tests"},
			},
		},
		{
			name: "internal tasks and bare command lists are handled",
			files: map[string]string{
				"Taskfile.yaml": `
version: '3'
tasks:
  build:
    desc: build the binary
  hidden:
    internal: true
    desc: should not be listed
  oneliner:
    - echo hi
`,
			},
			want: []taskInfo{
				{Name: "build", Desc: "build the binary"},
				{Name: "oneliner", Desc: ""},
			},
		},
		{
			name: "includes are namespaced",
			files: map[string]string{
				"Taskfile.yaml": `
version: '3'
includes:
  db: ./db/Taskfile.yaml
tasks:
  build:
    desc: build the binary
`,
				"db/Taskfile.yaml": `
version: '3'
tasks:
  migrate:
    desc: run migrations
`,
			},
			want: []taskInfo{
				{Name: "build", Desc: "build the binary"},
				{Name: "db:migrate", Desc: "run migrations"},
			},
		},
		{
			name: "include pointing at a directory resolves the conventional Taskfile",
			files: map[string]string{
				"Taskfile.yaml": `
version: '3'
includes:
  db: ./db
tasks:
  build:
    desc: build the binary
`,
				// note: directory reference, not a file; uses .yml (a tried name)
				"db/Taskfile.yml": `
version: '3'
tasks:
  migrate:
    desc: run migrations
`,
			},
			want: []taskInfo{
				{Name: "build", Desc: "build the binary"},
				{Name: "db:migrate", Desc: "run migrations"},
			},
		},
		{
			name: "nested includes are namespaced at every level",
			files: map[string]string{
				"Taskfile.yaml": `
version: '3'
includes:
  a: ./a
tasks:
  build:
    desc: build the binary
`,
				"a/Taskfile.yml": `
version: '3'
includes:
  b: ./b
tasks:
  lint:
    desc: lint a
`,
				"a/b/Taskfile.yml": `
version: '3'
tasks:
  test:
    desc: test b
`,
			},
			want: []taskInfo{
				{Name: "a:b:test", Desc: "test b"},
				{Name: "a:lint", Desc: "lint a"},
				{Name: "build", Desc: "build the binary"},
			},
		},
		{
			name: "Taskfile with no tasks yields nothing",
			files: map[string]string{
				"Taskfile.yaml": `
version: '3'
`,
			},
			want: nil,
		},
		{
			name: "the same Taskfile reached two ways keeps both namespaces (diamond)",
			files: map[string]string{
				"Taskfile.yaml": `
version: '3'
includes:
  x: ./shared
  y: ./shared
`,
				"shared/Taskfile.yml": `
version: '3'
tasks:
  common:
    desc: shared task
`,
			},
			want: []taskInfo{
				{Name: "x:common", Desc: "shared task"},
				{Name: "y:common", Desc: "shared task"},
			},
		},
		{
			name: "a cycle in the include graph terminates",
			files: map[string]string{
				"Taskfile.yaml": `
version: '3'
includes:
  b: ./b
tasks:
  root:
    desc: root task
`,
				"b/Taskfile.yml": `
version: '3'
includes:
  back: ../
tasks:
  loop:
    desc: b task
`,
			},
			// the include back to the root is dropped once the cycle is detected,
			// so we still see root's task plus b's task (namespaced once).
			want: []taskInfo{
				{Name: "b:loop", Desc: "b task"},
				{Name: "root", Desc: "root task"},
			},
		},
		{
			name: "flattened includes drop the namespace",
			files: map[string]string{
				"Taskfile.yaml": `
version: '3'
includes:
  extra:
    taskfile: ./extra/Taskfile.yaml
    flatten: true
tasks:
  build:
    desc: build the binary
`,
				"extra/Taskfile.yaml": `
version: '3'
tasks:
  lint:
    desc: lint the code
`,
			},
			want: []taskInfo{
				{Name: "build", Desc: "build the binary"},
				{Name: "lint", Desc: "lint the code"},
			},
		},
		{
			name: "internal and remote includes are skipped",
			files: map[string]string{
				"Taskfile.yaml": `
version: '3'
includes:
  shared:
    taskfile: ./shared/Taskfile.yaml
    internal: true
  remote: https://example.com/Taskfile.yaml
tasks:
  build:
    desc: build the binary
`,
				"shared/Taskfile.yaml": `
version: '3'
tasks:
  helper:
    desc: a helper
`,
			},
			want: []taskInfo{
				{Name: "build", Desc: "build the binary"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for relPath, contents := range tt.files {
				full := filepath.Join(dir, relPath)
				require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
				require.NoError(t, os.WriteFile(full, []byte(contents), 0o644))
			}
			got := discoverTasks(filepath.Join(dir, "Taskfile.yaml"))
			require.Equal(t, tt.want, got)
		})
	}
}

func TestMatchesAny(t *testing.T) {
	tests := []struct {
		name     string
		taskName string
		globs    []string
		want     bool
	}{
		{
			name:     "no globs matches everything",
			taskName: "build",
			globs:    nil,
			want:     true,
		},
		{
			name:     "exact name matches",
			taskName: "build",
			globs:    []string{"build"},
			want:     true,
		},
		{
			name:     "exact name does not match other task",
			taskName: "test",
			globs:    []string{"build"},
			want:     false,
		},
		{
			name:     "prefix glob matches namespaced task",
			taskName: "db:migrate",
			globs:    []string{"db:*"},
			want:     true,
		},
		{
			name:     "prefix glob does not match unrelated task",
			taskName: "build",
			globs:    []string{"db:*"},
			want:     false,
		},
		{
			name:     "matches when any glob matches",
			taskName: "test",
			globs:    []string{"build", "test", "db:*"},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, matchesAny(tt.taskName, tt.globs))
		})
	}
}
