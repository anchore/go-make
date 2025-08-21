package github

import "strconv"

type Param func() string

func Owner(v string) Param {
	return queryParam("owner", v)
}

func Repo(v string) Param {
	return queryParam("repo", v)
}

func HeadSha(v string) Param {
	return queryParam("head_sha", v)
}

func Branch(v string) Param {
	return queryParam("branch", v)
}

func Name(v string) Param {
	return queryParam("name", v)
}

func Actor(v string) Param {
	return queryParam("actor", v)
}

// Status in: completed, action_required, cancelled, failure, neutral, skipped,
// stale, success, timed_out, in_progress, queued, requested, waiting, pending
func Status(v string) Param {
	return queryParam("status", v)
}

func PerPage(v uint) Param {
	return queryParam("per_page", strconv.Itoa(int(v)))
}

func sort(v string) Param {
	return queryParam("sort", v)
}

func direction(v string) Param {
	return queryParam("direction", v)
}
