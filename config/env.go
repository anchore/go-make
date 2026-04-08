package config

import "os"

// Env returns the value of an environment variable, or defaultValue if not set.
// Unlike os.Getenv, this distinguishes between "not set" and "set to empty string".
func Env(key, defaultValue string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		return defaultValue
	}
	return value
}
