package goreleaser

import (
	"testing"

	"github.com/anchore/go-make/require"
)

func Test_releaseDependencyTasks(t *testing.T) {
	tests := []struct {
		name      string
		toolNames []string
		wantNames []string
		wantDeps  []string
	}{
		{
			name:      "single tool",
			toolNames: []string{"quill"},
			wantNames: []string{"dependencies:quill", "release:dependencies"},
			wantDeps:  []string{"dependencies:quill"},
		},
		{
			name:      "multiple tools",
			toolNames: []string{"quill", "syft", "cosign"},
			wantNames: []string{"dependencies:quill", "dependencies:syft", "dependencies:cosign", "release:dependencies"},
			wantDeps:  []string{"dependencies:quill", "dependencies:syft", "dependencies:cosign"},
		},
		{
			name:      "no tools",
			toolNames: nil,
			wantNames: []string{"release:dependencies"},
			wantDeps:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks := releaseDependencyTasks(tt.toolNames...)

			// shape: one task per name + aggregate
			require.Equal(t, len(tt.toolNames)+1, len(tasks))

			gotNames := make([]string, len(tasks))
			for i, tsk := range tasks {
				gotNames[i] = tsk.Name
			}
			require.EqualElements(t, tt.wantNames, gotNames)

			// per-tool install tasks must have a Run set (otherwise depending on them is a no-op)
			for i := range tt.toolNames {
				if tasks[i].Run == nil {
					t.Fatalf("task %q expected non-nil Run", tasks[i].Name)
				}
			}

			// aggregate task is appended last
			aggregate := tasks[len(tasks)-1]
			require.Equal(t, "release:dependencies", aggregate.Name)
			require.NotEmpty(t, aggregate.Description)
			if aggregate.Run != nil {
				t.Fatalf("aggregate task should be a label (no Run)")
			}
			require.EqualElements(t, tt.wantDeps, aggregate.Dependencies)
		})
	}
}
