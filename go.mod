module github.com/anchore/go-make

go 1.23.1 // for go 1.23, use .1, as .0 has a setup-go cache restore bug

require (
	github.com/bmatcuk/doublestar/v4 v4.9.1
	github.com/goccy/go-yaml v1.18.0
	golang.org/x/mod v0.27.0
)
