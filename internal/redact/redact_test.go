package redact

import (
	"testing"

	"github.com/anchore/go-make/require"
)

func TestIsSensitiveName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "github token env", input: "TAG_TOKEN", want: true},
		{name: "project-prefixed token", input: "ANCHORE_GO_MAKE_TAG_TOKEN", want: true},
		{name: "deploy key", input: "DEPLOY_KEY", want: true},
		{name: "password", input: "DB_PASSWORD", want: true},
		{name: "authorization header", input: "Authorization", want: true},
		{name: "proxy authorization header", input: "Proxy-Authorization", want: true},
		{name: "cookie header", input: "Cookie", want: true},
		{name: "credential helper key", input: "credential.helper", want: true},
		{name: "non-sensitive accept header", input: "Accept", want: false},
		{name: "non-sensitive api version", input: "X-GitHub-Api-Version", want: false},
		{name: "non-sensitive path key", input: "GOPATH", want: false},
		{name: "extraheader config key", input: "http.https://github.com/.extraheader", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, IsSensitiveName(tt.input))
		})
	}
}

func TestValue(t *testing.T) {
	require.Equal(t, "***", Value("TAG_TOKEN", "ghp_secret"))
	require.Equal(t, "plain", Value("GOPATH", "plain"))
}

func TestArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "masks sensitive flag=value",
			args: []string{"--token=ghp_secret", "push"},
			want: []string{"--token=***", "push"},
		},
		{
			name: "masks value following sensitive flag",
			args: []string{"--password", "hunter2", "login"},
			want: []string{"--password", "***", "login"},
		},
		{
			name: "leaves empty value readable",
			args: []string{"-c", "credential.helper=", "push"},
			want: []string{"-c", "credential.helper=", "push"},
		},
		{
			name: "leaves non-secret extraheader key readable",
			args: []string{"-c", "http.https://github.com/.extraheader=", "ls-remote"},
			want: []string{"-c", "http.https://github.com/.extraheader=", "ls-remote"},
		},
		{
			name: "leaves ordinary args untouched",
			args: []string{"push", "origin", "v1.0.0"},
			want: []string{"push", "origin", "v1.0.0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, Args(tt.args))
		})
	}
}

func TestSecrets(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "github classic token",
			input: "fatal: remote error using ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZ012345 oops",
			want:  "fatal: remote error using *** oops",
		},
		{
			name:  "fine-grained PAT",
			input: "token=github_pat_ABCDEFGHIJKLMNOPQRSTUV_0123456789",
			want:  "token=***",
		},
		{
			name:  "authorization bearer header",
			input: "Authorization: Bearer ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZ012345",
			want:  "Authorization: Bearer ***",
		},
		{
			name:  "basic auth extraheader line",
			input: "AUTHORIZATION: basic dG9rZW46cGFzc3dvcmQ=",
			want:  "AUTHORIZATION: basic ***",
		},
		{
			name:  "url userinfo",
			input: "cloning https://x-access-token:ghp_secretsecretsecret@github.com/o/r.git",
			want:  "cloning https://***:***@github.com/o/r.git",
		},
		{
			name:  "no secret passes through",
			input: "everything is fine here",
			want:  "everything is fine here",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, Secrets(tt.input))
		})
	}
}
