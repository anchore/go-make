package git

import (
	"strings"
	"testing"

	"github.com/anchore/go-make/require"
)

func TestValidateTag(t *testing.T) {
	tests := []struct {
		name    string
		tag     string
		wantErr require.ValidationError
	}{
		// valid cases
		{
			name: "valid simple tag",
			tag:  "v1.0.0",
		},
		{
			name: "valid tag with dots",
			tag:  "v1.2.3",
		},
		{
			name: "valid tag with slashes",
			tag:  "release/v1.0.0",
		},
		{
			name: "valid tag with underscores",
			tag:  "v1_0_0",
		},
		{
			name: "valid tag with dashes",
			tag:  "v1-0-0-rc1",
		},
		{
			name: "valid single char tag v",
			tag:  "v",
		},
		{
			name: "valid single char tag 1",
			tag:  "1",
		},
		{
			name: "valid tag starting with number",
			tag:  "1.0.0",
		},
		{
			name: "valid tag with alpha suffix",
			tag:  "v1.0.0-alpha",
		},
		{
			name: "valid tag with beta.1 suffix",
			tag:  "v1.0.0-beta.1",
		},
		{
			name: "valid tag with rc suffix",
			tag:  "v1.0.0_rc1",
		},
		{
			name: "valid hierarchical tag",
			tag:  "release/2024/v1/final",
		},
		{
			name: "valid tag with dots dashes underscores slashes",
			tag:  "v1.0.0-beta_1/release",
		},
		{
			name: "valid tag at max length (256 chars)",
			tag:  "v" + strings.Repeat("a", 255),
		},
		// invalid cases
		{
			name:    "empty tag",
			tag:     "",
			wantErr: require.Error,
		},
		{
			name:    "tag with spaces",
			tag:     "v1 0 0",
			wantErr: require.Error,
		},
		{
			name:    "tag with double dots",
			tag:     "v1..0",
			wantErr: require.Error,
		},
		{
			name:    "tag ending with .lock",
			tag:     "v1.0.0.lock",
			wantErr: require.Error,
		},
		{
			name:    "tag starting with dash",
			tag:     "-v1.0.0",
			wantErr: require.Error,
		},
		{
			name:    "tag starting with dot",
			tag:     ".v1.0.0",
			wantErr: require.Error,
		},
		{
			name:    "tag starting with slash",
			tag:     "/v1.0.0",
			wantErr: require.Error,
		},
		{
			name:    "tag starting with underscore",
			tag:     "_v1.0.0",
			wantErr: require.Error,
		},
		{
			name:    "tag with shell metacharacter semicolon",
			tag:     "v1.0.0;rm -rf",
			wantErr: require.Error,
		},
		{
			name:    "tag with shell metacharacter backtick",
			tag:     "v1.0.0`id`",
			wantErr: require.Error,
		},
		{
			name:    "tag with shell metacharacter dollar",
			tag:     "v1.0.0$(id)",
			wantErr: require.Error,
		},
		{
			name:    "tag with pipe",
			tag:     "v1.0.0|cat",
			wantErr: require.Error,
		},
		{
			name:    "tag with ampersand",
			tag:     "v1.0.0&id",
			wantErr: require.Error,
		},
		{
			name:    "tag with newline",
			tag:     "v1.0.0\nmalicious",
			wantErr: require.Error,
		},
		{
			name:    "tag with tab",
			tag:     "v1.0.0\tmalicious",
			wantErr: require.Error,
		},
		{
			name:    "tag too long (257 chars)",
			tag:     "v" + strings.Repeat("a", 256),
			wantErr: require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTag(tt.tag)
			tt.wantErr.Validate(t, err)
		})
	}
}

func TestValidateDeployKey(t *testing.T) {
	validPrivateKey := `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACBHK2aJ4cSB3j+0bhfNIJJL9yCuGFqUm4jJlO7C8VLTQwAAAJgEhJvVBISb
1QAAAAtzc2gtZWQyNTUxOQAAACBHK2aJ4cSB3j+0bhfNIJJL9yCuGFqUm4jJlO7C8VLTQ
wAAAECQYJy3qVPVLCJKT6zXRX7E5t1cMX7E5t1cMX7E5t1cMUcrZonhxIHeP7RuF80gkkv
3IK4YWpSbiMmU7sLxUtNDAAAADnRlc3RAZXhhbXBsZS5jb20BAgMEBQ==
-----END OPENSSH PRIVATE KEY-----`

	tests := []struct {
		name    string
		key     string
		wantErr require.ValidationError
	}{
		// valid cases
		{
			name: "valid openssh private key",
			key:  validPrivateKey,
		},
		{
			name: "valid RSA private key",
			key: `-----BEGIN RSA PRIVATE KEY-----
MIIBOgIBAAJBALRiMLAHudeSA2ai47KJm6SLp0vt4QYy...
-----END RSA PRIVATE KEY-----`,
		},
		{
			name: "valid DSA private key",
			key: `-----BEGIN DSA PRIVATE KEY-----
MIIBuwIBAAKBgQC...
-----END DSA PRIVATE KEY-----`,
		},
		{
			name: "valid EC private key",
			key: `-----BEGIN EC PRIVATE KEY-----
MHQCAQEEICc...
-----END EC PRIVATE KEY-----`,
		},
		{
			name: "valid PKCS8 private key",
			key: `-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgk...
-----END PRIVATE KEY-----`,
		},
		{
			name: "valid key with leading whitespace",
			key:  "  \n" + validPrivateKey,
		},
		{
			name: "valid key with trailing whitespace",
			key:  validPrivateKey + "\n  ",
		},
		{
			name: "valid key at exactly 16KB",
			key:  "-----BEGIN OPENSSH PRIVATE KEY-----" + strings.Repeat("a", 16*1024-70) + "-----END OPENSSH PRIVATE KEY-----",
		},
		// invalid cases
		{
			name:    "empty key",
			key:     "",
			wantErr: require.Error,
		},
		{
			name:    "public key rejected (ssh format)",
			key:     "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI... user@host",
			wantErr: require.Error,
		},
		{
			name: "PEM public key rejected",
			key: `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA...
-----END PUBLIC KEY-----`,
			wantErr: require.Error,
		},
		{
			name: "SSH2 public key rejected",
			key: `---- BEGIN SSH2 PUBLIC KEY ----
AAAAB3NzaC1yc2EAAA...
---- END SSH2 PUBLIC KEY ----`,
			wantErr: require.Error,
		},
		{
			name:    "key with null bytes",
			key:     "-----BEGIN OPENSSH PRIVATE KEY-----\x00-----END OPENSSH PRIVATE KEY-----",
			wantErr: require.Error,
		},
		{
			name:    "key exceeds 16KB",
			key:     "-----BEGIN OPENSSH PRIVATE KEY-----" + strings.Repeat("a", 17*1024) + "-----END OPENSSH PRIVATE KEY-----",
			wantErr: require.Error,
		},
		{
			name:    "not a PEM key",
			key:     "this is not a key at all",
			wantErr: require.Error,
		},
		{
			name:    "missing BEGIN marker",
			key:     "some content-----END OPENSSH PRIVATE KEY-----",
			wantErr: require.Error,
		},
		{
			name:    "missing END marker",
			key:     "-----BEGIN OPENSSH PRIVATE KEY-----some content",
			wantErr: require.Error,
		},
		{
			name:    "mismatched BEGIN/END markers",
			key:     "-----BEGIN RSA PRIVATE KEY-----\nsome content\n-----END OPENSSH PRIVATE KEY-----",
			wantErr: require.Error,
		},
		{
			name:    "certificate rejected",
			key:     "-----BEGIN CERTIFICATE-----\nMIIC...\n-----END CERTIFICATE-----",
			wantErr: require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDeployKey(tt.key)
			tt.wantErr.Validate(t, err)
		})
	}
}

func TestValidateSafeString(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		field   string
		wantErr require.ValidationError
	}{
		// valid cases
		{
			name:  "valid simple name",
			value: "github-actions",
			field: "git user name",
		},
		{
			name:  "valid name with bot suffix",
			value: "github-actions[bot]",
			field: "git user name",
		},
		{
			name:  "valid email",
			value: "user@example.com",
			field: "git user email",
		},
		{
			name:  "valid name with spaces",
			value: "John Doe",
			field: "git user name",
		},
		{
			name:  "valid name with underscore",
			value: "bot_name",
			field: "git user name",
		},
		{
			name:  "valid name with dots",
			value: "user.name",
			field: "git user name",
		},
		{
			name:  "valid name at max length (256 chars)",
			value: strings.Repeat("a", 256),
			field: "test field",
		},
		{
			name:  "valid single char",
			value: "a",
			field: "test field",
		},
		// invalid cases
		{
			name:    "empty value",
			value:   "",
			field:   "test field",
			wantErr: require.Error,
		},
		{
			name:    "contains semicolon",
			value:   "user;rm -rf",
			field:   "git user name",
			wantErr: require.Error,
		},
		{
			name:    "contains dollar sign",
			value:   "user$HOME",
			field:   "git user name",
			wantErr: require.Error,
		},
		{
			name:    "contains backtick",
			value:   "user`id`",
			field:   "git user name",
			wantErr: require.Error,
		},
		{
			name:    "contains parentheses",
			value:   "user(test)",
			field:   "git user name",
			wantErr: require.Error,
		},
		{
			name:    "contains backslash",
			value:   "user\\test",
			field:   "git user name",
			wantErr: require.Error,
		},
		{
			name:    "contains pipe",
			value:   "user|cat",
			field:   "git user name",
			wantErr: require.Error,
		},
		{
			name:    "contains ampersand",
			value:   "user&id",
			field:   "git user name",
			wantErr: require.Error,
		},
		{
			name:    "contains newline",
			value:   "user\nmalicious",
			field:   "git user name",
			wantErr: require.Error,
		},
		{
			name:    "contains carriage return",
			value:   "user\rmalicious",
			field:   "git user name",
			wantErr: require.Error,
		},
		{
			name:    "contains tab",
			value:   "user\tname",
			field:   "git user name",
			wantErr: require.Error,
		},
		{
			name:    "contains single quote",
			value:   "user'name",
			field:   "git user name",
			wantErr: require.Error,
		},
		{
			name:    "contains double quote",
			value:   `user"name`,
			field:   "git user name",
			wantErr: require.Error,
		},
		{
			name:    "contains less than",
			value:   "user<script>",
			field:   "git user name",
			wantErr: require.Error,
		},
		{
			name:    "contains greater than",
			value:   "user>file",
			field:   "git user name",
			wantErr: require.Error,
		},
		{
			name:    "value too long (257 chars)",
			value:   strings.Repeat("a", 257),
			field:   "test field",
			wantErr: require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSafeString(tt.value, tt.field)
			tt.wantErr.Validate(t, err)
		})
	}
}

func TestValidateRepository(t *testing.T) {
	tests := []struct {
		name    string
		repo    string
		wantErr require.ValidationError
	}{
		// valid cases
		{
			name: "valid simple repo",
			repo: "owner/repo",
		},
		{
			name: "valid repo with dots",
			repo: "my.org/my.repo",
		},
		{
			name: "valid repo with dashes",
			repo: "my-org/my-repo",
		},
		{
			name: "valid repo with underscores",
			repo: "my_org/my_repo",
		},
		{
			name: "valid repo with numbers",
			repo: "user123/project456",
		},
		{
			name: "valid repo mixed case",
			repo: "Org.Name/Repo.Name",
		},
		{
			name: "valid repo single char parts",
			repo: "a/b",
		},
		// invalid cases
		{
			name:    "empty repo",
			repo:    "",
			wantErr: require.Error,
		},
		{
			name:    "missing slash",
			repo:    "ownerrepo",
			wantErr: require.Error,
		},
		{
			name:    "multiple slashes",
			repo:    "owner/sub/repo",
			wantErr: require.Error,
		},
		{
			name:    "empty owner",
			repo:    "/repo",
			wantErr: require.Error,
		},
		{
			name:    "empty repo name",
			repo:    "owner/",
			wantErr: require.Error,
		},
		{
			name:    "contains spaces in owner",
			repo:    "my owner/repo",
			wantErr: require.Error,
		},
		{
			name:    "contains spaces in repo",
			repo:    "owner/my repo",
			wantErr: require.Error,
		},
		{
			name:    "contains shell metacharacter semicolon",
			repo:    "owner/repo;rm",
			wantErr: require.Error,
		},
		{
			name:    "contains backtick",
			repo:    "owner/repo`id`",
			wantErr: require.Error,
		},
		{
			name:    "contains dollar sign",
			repo:    "owner/$repo",
			wantErr: require.Error,
		},
		{
			name:    "contains parentheses",
			repo:    "owner/repo()",
			wantErr: require.Error,
		},
		{
			name:    "contains ampersand",
			repo:    "owner/repo&id",
			wantErr: require.Error,
		},
		{
			name:    "contains pipe",
			repo:    "owner/repo|cat",
			wantErr: require.Error,
		},
		{
			name:    "contains newline",
			repo:    "owner/repo\nmalicious",
			wantErr: require.Error,
		},
		{
			name:    "owner with semicolon",
			repo:    "owner;evil/repo",
			wantErr: require.Error,
		},
		{
			name:    "just a slash",
			repo:    "/",
			wantErr: require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRepository(tt.repo)
			tt.wantErr.Validate(t, err)
		})
	}
}

func TestCreateTagConfigValidate(t *testing.T) {
	tests := []struct {
		name         string
		tag          string
		tagMessage   string
		gitUserName  string
		gitUserEmail string
		wantPanic    bool
	}{
		// valid config
		{
			name:         "valid complete config",
			tag:          "v1.0.0",
			tagMessage:   "Release v1.0.0",
			gitUserName:  "github-actions[bot]",
			gitUserEmail: "github-actions[bot]@users.noreply.github.com",
		},
		{
			name:         "valid config with minimal values",
			tag:          "v1",
			tagMessage:   "Release",
			gitUserName:  "bot",
			gitUserEmail: "bot@example.com",
		},
		// invalid tag
		{
			name:         "fails on empty tag",
			tag:          "",
			tagMessage:   "Release",
			gitUserName:  "bot",
			gitUserEmail: "bot@example.com",
			wantPanic:    true,
		},
		{
			name:         "fails on invalid tag",
			tag:          "v1..0",
			tagMessage:   "Release",
			gitUserName:  "bot",
			gitUserEmail: "bot@example.com",
			wantPanic:    true,
		},
		// invalid tag message
		{
			name:         "fails on empty tag message",
			tag:          "v1.0.0",
			tagMessage:   "",
			gitUserName:  "bot",
			gitUserEmail: "bot@example.com",
			wantPanic:    true,
		},
		{
			name:         "fails on tag message with shell injection",
			tag:          "v1.0.0",
			tagMessage:   "Release;rm -rf",
			gitUserName:  "bot",
			gitUserEmail: "bot@example.com",
			wantPanic:    true,
		},
		// invalid git user name
		{
			name:         "fails on empty git user name",
			tag:          "v1.0.0",
			tagMessage:   "Release",
			gitUserName:  "",
			gitUserEmail: "bot@example.com",
			wantPanic:    true,
		},
		{
			name:         "fails on git user name with shell injection",
			tag:          "v1.0.0",
			tagMessage:   "Release",
			gitUserName:  "bot$(id)",
			gitUserEmail: "bot@example.com",
			wantPanic:    true,
		},
		// invalid git user email
		{
			name:         "fails on empty git user email",
			tag:          "v1.0.0",
			tagMessage:   "Release",
			gitUserName:  "bot",
			gitUserEmail: "",
			wantPanic:    true,
		},
		{
			name:         "fails on git user email with shell injection",
			tag:          "v1.0.0",
			tagMessage:   "Release",
			gitUserName:  "bot",
			gitUserEmail: "bot`id`@example.com",
			wantPanic:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if tt.wantPanic && r == nil {
					t.Error("expected panic but did not get one")
				}
				if !tt.wantPanic && r != nil {
					t.Errorf("unexpected panic: %v", r)
				}
			}()

			cfg := CreateTagConfig{
				Tag:          tt.tag,
				TagMessage:   tt.tagMessage,
				GitUserName:  tt.gitUserName,
				GitUserEmail: tt.gitUserEmail,
			}
			cfg.validate()

			if tt.wantPanic {
				t.Error("expected panic but got config")
				return
			}

			require.Equal(t, tt.tag, cfg.Tag)
			require.Equal(t, tt.tagMessage, cfg.TagMessage)
			require.Equal(t, tt.gitUserName, cfg.GitUserName)
			require.Equal(t, tt.gitUserEmail, cfg.GitUserEmail)
		})
	}
}

func TestPushTagConfigValidate(t *testing.T) {
	validPrivateKey := `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACBHK2aJ4cSB3j+0bhfNIJJL9yCuGFqUm4jJlO7C8VLTQwAAAJgEhJvVBISb
1QAAAAtzc2gtZWQyNTUxOQAAACBHK2aJ4cSB3j+0bhfNIJJL9yCuGFqUm4jJlO7C8VLTQ
wAAAECQYJy3qVPVLCJKT6zXRX7E5t1cMX7E5t1cMX7E5t1cMUcrZonhxIHeP7RuF80gkkv
3IK4YWpSbiMmU7sLxUtNDAAAADnRlc3RAZXhhbXBsZS5jb20BAgMEBQ==
-----END OPENSSH PRIVATE KEY-----`

	tests := []struct {
		name       string
		tag        string
		deployKey  string
		repository string
		wantPanic  bool
	}{
		// valid config
		{
			name:       "valid complete config",
			tag:        "v1.0.0",
			deployKey:  validPrivateKey,
			repository: "owner/repo",
		},
		{
			name:       "valid config with minimal values",
			tag:        "v1",
			deployKey:  validPrivateKey,
			repository: "a/b",
		},
		// invalid tag
		{
			name:       "fails on empty tag",
			tag:        "",
			deployKey:  validPrivateKey,
			repository: "owner/repo",
			wantPanic:  true,
		},
		{
			name:       "fails on invalid tag",
			tag:        "v1..0",
			deployKey:  validPrivateKey,
			repository: "owner/repo",
			wantPanic:  true,
		},
		// invalid deploy key
		{
			name:       "fails on empty deploy key",
			tag:        "v1.0.0",
			deployKey:  "",
			repository: "owner/repo",
			wantPanic:  true,
		},
		{
			name:       "fails on invalid deploy key",
			tag:        "v1.0.0",
			deployKey:  "not a key",
			repository: "owner/repo",
			wantPanic:  true,
		},
		// invalid repository
		{
			name:       "fails on empty repository",
			tag:        "v1.0.0",
			deployKey:  validPrivateKey,
			repository: "",
			wantPanic:  true,
		},
		{
			name:       "fails on invalid repository format",
			tag:        "v1.0.0",
			deployKey:  validPrivateKey,
			repository: "invalid",
			wantPanic:  true,
		},
		{
			name:       "fails on repository with shell injection",
			tag:        "v1.0.0",
			deployKey:  validPrivateKey,
			repository: "owner;evil/repo",
			wantPanic:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if tt.wantPanic && r == nil {
					t.Error("expected panic but did not get one")
				}
				if !tt.wantPanic && r != nil {
					t.Errorf("unexpected panic: %v", r)
				}
			}()

			cfg := PushTagConfig{
				Tag:        tt.tag,
				DeployKey:  tt.deployKey,
				Repository: tt.repository,
			}
			cfg.validate()

			if tt.wantPanic {
				t.Error("expected panic but got config")
				return
			}

			require.Equal(t, tt.tag, cfg.Tag)
			require.Equal(t, tt.deployKey, cfg.DeployKey)
			require.Equal(t, tt.repository, cfg.Repository)
		})
	}
}

func TestCreateTagConfigStruct(t *testing.T) {
	cfg := CreateTagConfig{
		Tag:          "v1.0.0",
		TagMessage:   "message",
		GitUserName:  "user",
		GitUserEmail: "email@example.com",
	}

	require.Equal(t, "v1.0.0", cfg.Tag)
	require.Equal(t, "message", cfg.TagMessage)
	require.Equal(t, "user", cfg.GitUserName)
	require.Equal(t, "email@example.com", cfg.GitUserEmail)
}

func TestPushTagConfigStruct(t *testing.T) {
	cfg := PushTagConfig{
		Tag:        "v1.0.0",
		DeployKey:  "key",
		Repository: "owner/repo",
	}

	require.Equal(t, "v1.0.0", cfg.Tag)
	require.Equal(t, "key", cfg.DeployKey)
	require.Equal(t, "owner/repo", cfg.Repository)
}
