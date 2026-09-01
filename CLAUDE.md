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
- The emulator manager (`internal/avd`, `internal/toolsmanager/sdk*.go`,
  `internal/app/screen_emulator*.go`) shells out to a real Android SDK
  toolchain (`avdmanager`/`sdkmanager`/`emulator`, plus Java) - reading the
  code is not enough to trust a change here. Where possible, actually run
  `android-toolbox emulator setup/list/create/start` (or the TUI's
  Emulators tool) against a real machine rather than reasoning about what
  `avdmanager`'s output "should" look like: several real bugs in this
  feature (a missing `ANDROID_SDK_ROOT` the emulator silently needed, a
  double-launch crash, `avdmanager` splitting broken AVDs into their own
  unparsed section) only ever showed up against real tool output, never in
  a synthetic test case. See the "Emulator manager (AVDs)" section of
  AGENTS.md before touching its sourcing logic.
- This project was itself built with heavy AI assistance (see the
  disclaimer near the top of [README.md](README.md)). When asked to
  "clean up" or "review" code here, don't assume existing patterns are
  correct by default - verify against actual behavior (run the build, run
  the tests, and for TUI/CLI changes, actually run the binary) rather than
  trusting that AI-authored code already works as intended.
- Before finishing any change, run `go build ./...`, `go vet ./...`,
  `gofmt -l .`, and `go test ./...` - see AGENTS.md for the equivalent
  `make` targets.
