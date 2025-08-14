.PHONY: *
.DEFAULT_GOAL: help

help:
	@go run -C .make . help

.PHONY: *
.DEFAULT:
%:
	@go run -C .make . $@
