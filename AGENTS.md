# Agent instructions for android-toolbox

Practical guidance for any AI coding agent (Claude Code or otherwise)
working in this repository.

## What this is

A Go CLI/TUI (Cobra + Bubbletea) for controlling Android devices via `adb`
and `scrcpy`. Entry point: `cmd/android-toolbox/`. Everything else lives
under `internal/` (see the table below). This project was itself largely
written with AI assistance - see the note near the top of
[README.md](README.md). Keep that in mind: don't assume existing code is
correct just because it's there; verify behavior, especially around
OS-specific shell handling and the AI action-generation path.

## Build, test, lint

```bash
go build ./...      # or: make build   (embeds VERSION/commit via -ldflags)
go vet ./...         # or: make vet
gofmt -l .           # or: make fmt    (should print nothing)
go test ./...        # or: make test
```

Run all four before considering any change done. CI
(`.github/workflows/ci.yml`) enforces the same gate on every PR/branch, and
`.github/workflows/release.yml` re-runs it before cutting a release from
`main` - a failure there blocks the release entirely.

## Language convention (read this before touching any string)

**English is the default and only language everywhere except one specific,
gated table.** Concretely:

- All CLI output, `cobra.Command` `Short`/`Long` text, flag descriptions,
  and `error` strings across `cmd/android-toolbox/` and every `internal/*`
  package must be English. There is no localization for any of this.
- The **one exception**: `internal/app/i18n.go` defines `uiTextEN` /
  `uiTextDE`, selected at runtime by `config.Settings.Language()` (defaults
  to `"en"`; `"de"` only if the user explicitly set it in Settings). This
  is a deliberate, tested TUI-chrome bilingual system - `internal/app/action_translations.go`
  is its counterpart for built-in action names/descriptions. When adding a
  new TUI-visible string, add it to **both** `uiTextEN` and `uiTextDE` in
  `i18n.go`, matching the existing field-per-string pattern. Do not
  introduce a third language table or a second mechanism for translating
  anything.
- Everything else - `internal/healthcheck` results, error messages from
  `internal/actions`, `internal/toolsmanager`, `internal/ai`, etc. - is
  plain English with no translation layer, and should stay that way. If
  you're tempted to hardcode non-English text anywhere outside
  `i18n.go`/`action_translations.go`, don't.
- User-created/AI-generated actions (`actions.yaml` entries beyond the
  built-in set) are exempt: whatever language the user or the AI wrote
  them in is displayed verbatim, unmodified. Only the *built-in* actions in
  `internal/actions/actions.default.yaml` go through the translation table.

## Adding a new AI provider

`internal/ai/provider.go` defines the `Provider` interface
(`Name() string`, `Available() error`, `GenerateAction(ctx, req) (ActionDraft, error)`).
To add one beyond the existing `internal/ai/claude.go`:

1. New file in `internal/ai/`, e.g. `internal/ai/mytool.go`, implementing
   `Provider`.
2. In its `init()`, call `ai.Register("mytool", newMyToolProvider)` (see
   `internal/ai/registry.go`).
3. No other file needs to change - `ai.New(name, ...)` resolves it by the
   `ai.provider` value in `settings.yaml`.

If the provider shells out to an external CLI and builds its own prompt
(like `claude.go`'s `buildUserPrompt`), keep the host-OS-awareness pattern:
tell the model which OS `tool: shell` commands will actually run on (see
`hostOSDescription()` in `claude.go` and the "Host OS accuracy" section of
`internal/ai/system_prompt.default.md`) - a shell command that only works
on a different OS than the one the app is running on is useless output.

## OS-conditional code

This codebase uses Go build tags for OS-specific behavior, not runtime
branching, wherever the code genuinely differs per platform:

- `internal/actions/shell_windows.go` / `shell_unix.go` - how `tool: shell`
  actions are executed (`cmd.exe` vs `sh -c`), including Windows'
  hand-built `SysProcAttr.CmdLine` to avoid Go's default re-quoting
  mangling already-quoted paths.
- `internal/install/install_windows.go` / `install_unix.go` - PATH
  installation (registry vs symlink + PATH note).
- `internal/toolsmanager/sdkcmd_windows.go` / `sdkcmd_unix.go` - how
  cmdline-tools' `sdkmanager`/`avdmanager` scripts are invoked (Windows
  `.bat` files need `cmd /c`; Unix's are directly executable via their own
  shebang).

Follow this pattern (a `_windows.go` and a `_unix.go`/`_other.go` pair with
matching `//go:build` tags and identical exported signatures) for any new
platform-specific behavior, rather than `runtime.GOOS` branches scattered
through shared code.

## Emulator manager (AVDs)

`internal/avd` + `internal/toolsmanager/sdk*.go` implement creating/managing
Android Virtual Devices, on top of Google's `avdmanager`/`sdkmanager`/
`emulator` binaries (a Java toolchain, resolved separately from - and
preferred over - a bundled copy if `ANDROID_HOME`/`ANDROID_SDK_ROOT` already
points at a usable SDK; see `toolsmanager.resolveSdkTool`). Three sourcing
decisions here are deliberate, not oversights - don't "fix" them without
re-reading this first:

- **`cmdlineToolsRevision` in `sdksources.go` is a pinned, hand-bumped
  constant**, not a live version lookup. Google publishes no stable
  "-latest-" alias for this package (unlike platform-tools) and no
  GitHub-releases-style API (unlike scrcpy) - the revision is looked up by
  hand from developer.android.com/studio#command-tools and bumped
  occasionally, the same staleness tolerance this codebase already accepts
  for scrcpy's own hardcoded `fallbackScrcpyVersion`.
- **Everything installed *after* cmdline-tools (system images, platforms,
  the `emulator` package itself) is always fetched live by shelling out to
  the bootstrapped `sdkmanager --install`** (see
  `toolsmanager.InstallSdkPackage`), rather than reimplementing Google's SDK
  repository XML resolution in Go. Do not add a hardcoded URL/version for
  any of these - sdkmanager already resolves them correctly and it's the
  only piece of this that changes often enough to matter.
- **The emulator binary refuses to start unless its SDK root has a
  `platform-tools` subdirectory** - checked via `ANDROID_SDK_ROOT`/
  `ANDROID_HOME` or guessed from its own path otherwise, and fatal
  ("Cannot find AVD system path") if neither checks out. Since this app
  manages its own `adb` in a separate directory from the SDK root it
  bootstraps for the emulator (`toolsmanager.SdkRoot()`), that directory is
  mirrored into place by `toolsmanager.EnsureEmulatorPlatformTools` before
  every launch (`internal/avd/launch.go`'s `Launcher` also always exports
  `ANDROID_SDK_ROOT`/`ANDROID_HOME`) - this was a real, silent-failure bug
  during development (a "Running... exit status 1" with no other symptom),
  so don't drop either half of the fix.

`avdmanager list avd` reports two sections: normal AVDs, and ones under "The
following Android Virtual Devices could not be loaded" (typically a system
image later removed via `sdkmanager --uninstall` or manually) - `AVD.Broken`
tracks the latter (see `avd.parseAVDList`), and the TUI/CLI both must keep
surfacing it distinctly rather than showing a healthy-looking entry with
silently blank fields, which is what happened before that field existed.

## Actions schema

`internal/actions/schema.go` defines `Action`; `internal/actions/actions.default.yaml`
is the seed file embedded via `//go:embed` and copied to
`<config>/actions.yaml` on first run. `internal/actions/loader.go` handles
load/validate/save; invalid individual entries are collected rather than
failing the whole file (see `ActionSet.Invalid`). See the README's
["Extending actions"](README.md#extending-actions-actionsyaml) section for
the field-level contract (`tool`/`command`/`params`/`confirm`/`interactive`/`format`/`live_preview`).

## Project layout

```
cmd/android-toolbox/   Cobra commands; no subcommand -> starts the TUI (internal/app)
internal/
  config/        Paths, settings, first-run state
  logging/       File logger + panic recovery
  toolsmanager/  Download/resolution of adb/scrcpy/cmdline-tools, sdkmanager wrapper
  adb/           adb client (devices, shell, battery, ...)
  device/        Aggregated device info
  scrcpy/        scrcpy process launch (detached, log-redirected)
  avd/           AVD lifecycle (list/create/delete), config.ini specs, emulator launch, emulator-console simulation
  actions/       Schema, YAML loader, execution (placeholders, streaming, OS shell)
  output/        Classifies output lines (logcat/keyvalue/packages) for TUI highlighting
  ai/            AI provider interface, registry, Claude CLI implementation, prompt
  backup/        Snapshot/list/restore of config files
  install/       PATH installation (Windows registry / Unix symlink)
  healthcheck/   Startup diagnostics
  apkinfo/       Pure-Go APK manifest/signing-block parser (no aapt/SDK)
  selfupdate/    GitHub-release-based self-update
  app/           Bubbletea TUI - one screen_*.go per screen, i18n.go for bilingual chrome
```

## Testing conventions

Most packages use table-driven tests with `t.TempDir()`/`t.Setenv()` for
filesystem/env isolation rather than mocking - follow that pattern for new
tests (see `internal/backup/backup_test.go` or `internal/actions/loader_test.go`
for examples). Packages needing a real device or network access
(`internal/adb`, `internal/toolsmanager`, parts of `cmd/android-toolbox`)
are lower-coverage by necessity; don't force a mock adb/network layer into
existence just to chase coverage there - prefer testing the pure logic
(parsing, path resolution, formatting) directly instead.

`internal/adb/devices_test.go` and `internal/avd/avd_test.go` both favor
verbatim captures of real `adb devices -l`/`avdmanager list avd` output as
test fixtures over hand-written approximations - real tool output has
already surfaced parsing bugs synthetic fixtures didn't (e.g. `AVD.Broken`:
`avdmanager` reports AVDs with a since-deleted system image under a
completely separate "could not be loaded" section, with none of the normal
Device/Target/Tag-ABI fields). Prefer a real capture over a hand-rolled one
when adding a test against either tool's output.
