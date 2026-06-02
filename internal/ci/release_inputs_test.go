package ci

import (
	"os"
	"testing"

	"github.com/anchore/go-make/require"
)

func TestReleasePushCredentials(t *testing.T) {
	tests := []struct {
		name string
		// env values to set before the call; an empty string means the var is
		// left unset (os.Getenv treats unset and empty identically here).
		deployKeyEnv  string
		tagTokenEnv   string
		wantPanic     bool
		wantDeployKey string
		wantTagToken  string
	}{
		{
			name:         "tag token only",
			tagTokenEnv:  "ghp_token",
			wantTagToken: "ghp_token",
		},
		{
			name:          "deploy key only",
			deployKeyEnv:  "-----BEGIN KEY-----",
			wantDeployKey: "-----BEGIN KEY-----",
		},
		{
			// when both are set TAG_TOKEN wins and DEPLOY_KEY is dropped.
			name:         "both set, tag token wins",
			deployKeyEnv: "-----BEGIN KEY-----",
			tagTokenEnv:  "ghp_token",
			wantTagToken: "ghp_token",
		},
		{
			name:      "neither set panics",
			wantPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// start from a clean slate and guarantee both vars are gone after
			// the subtest regardless of how it exits.
			require.NoError(t, os.Unsetenv("DEPLOY_KEY"))
			require.NoError(t, os.Unsetenv("TAG_TOKEN"))
			t.Cleanup(func() {
				_ = os.Unsetenv("DEPLOY_KEY")
				_ = os.Unsetenv("TAG_TOKEN")
			})

			if tt.deployKeyEnv != "" {
				require.NoError(t, os.Setenv("DEPLOY_KEY", tt.deployKeyEnv))
			}
			if tt.tagTokenEnv != "" {
				require.NoError(t, os.Setenv("TAG_TOKEN", tt.tagTokenEnv))
			}

			// the deferred recover is the single source of truth for
			// panic-vs-no-panic expectations; assertions below run only on the
			// no-panic path.
			defer func() {
				r := recover()
				if tt.wantPanic && r == nil {
					t.Error("expected panic but did not get one")
				}
				if !tt.wantPanic && r != nil {
					t.Errorf("unexpected panic: %v", r)
				}
			}()

			deployKey, tagToken := ReleasePushCredentials()

			require.Equal(t, tt.wantDeployKey, deployKey)
			require.Equal(t, tt.wantTagToken, tagToken)

			// defense-in-depth: both source env vars must be purged after the
			// call so they aren't inherited by later child processes.
			require.Equal(t, "", os.Getenv("DEPLOY_KEY"))
			require.Equal(t, "", os.Getenv("TAG_TOKEN"))
		})
	}
}
