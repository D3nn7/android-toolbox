# CLAUDE.md

This file gives Claude Code guidance for working in this repository.

Read [AGENTS.md](AGENTS.md) first - it has the actual build/test/lint
commands, the project layout, and the conventions that matter most here
(the English-only-outside-one-table language rule, the AI-provider
registry pattern, the OS-build-tag pattern). Everything in it applies to
Claude Code equally; it isn't duplicated here to avoid the two files
drifting apart.

## Claude-specific notes

- `internal/ai/claude.go` shells out to the very CLI you're running as
  (`claude -p --output-format json --system-prompt ...`) to power the
  app's own AI action-generation feature. If you're changing that file or
  `internal/ai/system_prompt.default.md`, be aware you're editing how a
  *different* invocation of Claude gets prompted at runtime - test by
  actually running `android-toolbox ai "<request>"` (or the TUI's `a` key),
  not just by reading the prompt.
- This project was itself built with heavy AI assistance (see the
  disclaimer near the top of [README.md](README.md)). When asked to
  "clean up" or "review" code here, don't assume existing patterns are
  correct by default - verify against actual behavior (run the build, run
  the tests, and for TUI/CLI changes, actually run the binary) rather than
  trusting that AI-authored code already works as intended.
- Before finishing any change, run `go build ./...`, `go vet ./...`,
  `gofmt -l .`, and `go test ./...` - see AGENTS.md for the equivalent
  `make` targets.
