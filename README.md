# android-toolbox

[atbx.schape.it](https://atbx.schape.it)

An extensible CLI/TUI toolbox for controlling connected Android devices on
Windows, Linux, and macOS. Device interaction goes through pluggable device
tools, actions are configuration-driven (no code required to add one), the
UI supports multiple languages, and an AI mode can generate new actions from
a plain-text description via a pluggable AI provider - each of these is
built to grow beyond what's currently implemented:

- **Device tools**: `adb`, `scrcpy`
- **AI provider**: Claude Code CLI
- **UI languages**: English, German
- **Emulator manager**: create, configure, and run Android Virtual Devices
  (AVDs) from inside the app - see ["Emulator manager"](#emulator-manager-avds)

> **Built with AI assistance.** This project was developed with
> significant help from AI tools. The usual caveats apply: review anything
> before you rely on it, especially destructive actions (`confirm: true`
> entries, `dangerous-reset`, AI-generated commands) and anything touching
> a device or data you care about.

## Features

- **Interactive TUI** (Bubbletea/Bubbles): healthcheck → device selection →
  a two-pane dashboard - the action list stays visible on the left to
  browse/search, while the right pane's border color reflects its state
  (preview/params/confirm/running/done) - with color-highlighted live
  output.
- **Portable tools**: device tools are managed by the app itself
  (downloaded from their official sources, isolated in their own tool
  cache) - no dependency on a system-wide install. Currently: `adb` and
  `scrcpy`.
- **Configuration-driven actions**: every action (logs, file transfer,
  shell commands, scrcpy variants, ...) lives in an editable `actions.yaml`.
  Adding a new action means a new YAML entry, no code required.
- **AI mode**: a free-text request generates a matching action (with a
  preview/confirmation step before saving), via a pluggable AI provider -
  see ["Extending AI mode"](#extending-ai-mode). Currently implemented:
  the Claude Code CLI.
- **Backup & recover**: every change to `actions.yaml`/`settings.yaml` is
  preceded by an automatic timestamped backup, restorable at any time.
- **Optional PATH install**: on first run, you're asked whether the command
  should be made available system-wide (including the short alias `atbx`),
  or left portable (nothing copied, runs only from where you launched it).
  On Windows this updates the user PATH registry value directly; on
  Linux/macOS it symlinks into `~/.local/bin` and appends an export line to
  your shell's rc file (`~/.zshrc`, `~/.bash_profile`/`~/.bashrc`, or
  fish's `config.fish`, picked from `$SHELL`) if that directory isn't on
  PATH yet - either way, a newly opened terminal is required for the change
  to take effect.
- **Multilingual UI**: built on a translation table so new languages can be
  added without touching the rest of the app (see `internal/app/i18n.go`).
  Currently implemented: English (default) and German, switchable from the
  Settings screen (`s` in the dashboard). The AI provider/command/timeout
  can also be adjusted there, along with the resolved tool paths
  (adb/scrcpy/config directory).
- **Emulator manager**: a third tool (alongside Devices/APK Info, `ctrl+t`
  to switch) for creating, listing, starting/stopping, and deleting AVDs,
  with a richer detail view than a physical device gets (specs from
  `config.ini`, running status) plus live simulation - GPS position,
  network speed/latency profiles, battery level/charging state - driven
  through the emulator's own console via `adb emu`. Missing system images
  are downloaded on demand with a live progress bar. See
  ["Emulator manager"](#emulator-manager-avds).

## Installation / first run

```bash
go build -o android-toolbox.exe ./cmd/android-toolbox
./android-toolbox.exe
```

On first run:
1. A **healthcheck** verifies configuration, `adb`, `scrcpy`, and the AI
   provider.
2. If `adb`/`scrcpy` are missing, `android-toolbox tools fetch` fetches them
   once from their official sources (Google platform-tools and the
   [scrcpy](https://github.com/Genymobile/scrcpy) GitHub release). The
   emulator manager's own tools (`sdkmanager`/`avdmanager`/`emulator`, plus
   Java) are checked the same way but fetched separately and lazily, since
   that whole feature is optional - see
   ["Emulator manager"](#emulator-manager-avds).
3. The app asks whether it should register itself system-wide on PATH
   (only on the very first run).
4. After that: **choose a tool** (Devices, APK Info, or Emulators) → device
   selection/dashboard, an APK file, or the AVD list. `ctrl+t` switches
   tools at any time without restarting the app.

### macOS: "cannot be opened because the developer cannot be verified"

Release binaries are unsigned and not notarized, so macOS Gatekeeper flags
a downloaded, unpacked `android-toolbox` as coming from an unidentified
developer (or, on newer macOS versions, refuses to open it at all from
Finder). This is not a bug in the binary.

Either of these unblocks a downloaded binary (do this once per download):

```bash
xattr -d com.apple.quarantine ./android-toolbox
```

or right-click (Control-click) the file in Finder → **Open** → **Open**
in the confirmation dialog. Downloading via `curl`/`git clone` instead of a
browser avoids the quarantine flag being set in the first place, since only
browsers and similar apps apply it.

### Windows: "Windows protected your PC" (SmartScreen)

Release binaries are likewise unsigned, so SmartScreen shows its standard
unknown-publisher warning on first run - click **More info** → **Run
anyway**. Resolving that requires a purchased code-signing certificate (a
standard one still needs downloads to build up reputation before
SmartScreen trusts it automatically; only an EV certificate skips that).

All configuration files live under the OS's usual user-config directory,
e.g. on Windows under `%APPDATA%\android-toolbox\`:

```
actions.yaml       Actions (editable, seeded on first run)
settings.yaml       Base settings (AI provider, refresh intervals, ...)
ai/system_prompt.md Base prompt for AI mode (editable)
state.json           First-run/install status
.backup/             Automatic backups (timestamped files)
tools/                Downloaded, portable adb/scrcpy binaries
tools/sdk/            Bootstrapped Android cmdline-tools + emulator/system images
logs/                 Log file
```

## Key commands

Without an argument, `android-toolbox` starts the interactive TUI. For
scripts and quick checks, there's also:

| Command | Purpose |
|---|---|
| `android-toolbox healthcheck` | Runs all checks once |
| `android-toolbox devices [-v]` | Lists connected devices |
| `android-toolbox tools fetch [--os] [--arch]` | Downloads adb/scrcpy (fresh) |
| `android-toolbox tools status` | Shows which binaries are currently in use |
| `android-toolbox tools check [--os] [--arch]` | Checks (without downloading) whether newer adb/scrcpy versions are available |
| `android-toolbox tools update [--force]` | Updates adb/scrcpy if a newer version is available |
| `android-toolbox run <action-id> [--serial S] [--param k=v]` | Runs an action non-interactively |
| `android-toolbox ai "<request>" [--yes]` | Generates an action via AI (and saves it) |
| `android-toolbox backup` | Manual backup of `actions.yaml`/`settings.yaml` |
| `android-toolbox recover [index] [--list]` | Lists/restores a backup |
| `android-toolbox install` | System-wide PATH install (can be re-run later) |
| `android-toolbox self-update [--check] [--yes]` | Checks for a new android-toolbox version and installs it |
| `android-toolbox dangerous-reset [--yes]` | Deletes all local android-toolbox data and reinstalls everything |
| `android-toolbox apk-info <path> [--json]` | Analyzes an .apk file (package, version, permissions, signing) |
| `android-toolbox emulator setup` | Downloads the Android cmdline-tools and reports Java/sdkmanager/avdmanager/emulator status |
| `android-toolbox emulator list` | Lists locally defined AVDs |
| `android-toolbox emulator create --name --image [--device] [--sdcard]` | Creates an AVD, downloading its system image first if needed |
| `android-toolbox emulator start <name> [--headless]` | Starts an emulator by AVD name |
| `android-toolbox emulator stop <name>` | Stops a running emulator by AVD name |
| `android-toolbox emulator delete <name>` | Deletes an AVD |

## TUI shortcuts

After the healthcheck, the app first shows a **tool selection** screen:
**Devices** (the classic adb/scrcpy dashboard flow), **APK Info** (file
browser → analysis of a local `.apk` file, see ["APK analysis"](#apk-analysis-apk-info)
below), or **Emulators** (create/manage/simulate AVDs, see
["Emulator manager"](#emulator-manager-avds) below). `ctrl+t` jumps back to
this selection from **anywhere** in the app - even mid-action or
mid-file-analysis - cleaning up any still-running streams/live previews
along the way, just like quitting does.

In the dashboard (after device selection), the action list is always
visible on the left; the right pane shows a preview of the highlighted
action, the parameter prompt, a confirmation, or the running output
depending on state - indicated by the border color. Just highlighting/
browsing **never** runs an action - that only happens via `enter`. The one
exception: actions with `live_preview: true` (e.g. battery status, device
info) show their result automatically when highlighted, since they're
fast, side-effect-free read commands.

Above the action list, a row of category pills ("All" plus one per
`category` value from `actions.yaml`) shows which category is currently
filtered; the active one is highlighted in green.

| Key | Action |
|---|---|
| `enter` | Run the highlighted action (params/confirm/start) |
| `/` | Search actions (name, category, description, ID) |
| `tab` / `shift+tab` | Switch to the next/previous category pill |
| `a` | Create a new action via AI |
| `e` | Edit a self-created action |
| `b` | View/restore backups |
| `s` | Settings (also available from device selection) |
| `x` | Dismiss the update notice (if currently shown) |
| `ctrl+g` | Switch device |
| `ctrl+t` | Switch tool (Devices ↔ APK Info ↔ Emulators) |
| `q` | Quit |

A subtle notice under the title (healthcheck screen and dashboard header)
appears automatically whenever a newer android-toolbox release and/or an
outdated adb/scrcpy build is known (see ["Self-update"](#self-update)
below) - dismissible with `x`, but it reappears on every restart as long as
the update is still pending.

## Extending actions (`actions.yaml`)

Every action is a YAML entry with the following schema:

```yaml
- id: battery-info            # unique, stable, no spaces
  name: "Battery Status"      # display name in the TUI
  description: "Shows detailed battery status"
  category: Device            # grouping in the action list
  tool: adb                   # adb | scrcpy | shell
  command: "shell dumpsys battery"
  params: []                  # see below
  confirm: false               # true = ask for confirmation before running
  interactive: false            # true = a real terminal handover (e.g. an open shell)
  format: keyvalue              # optional: highlights the output instead of showing it raw
  live_preview: false           # true = run automatically when highlighted (see below)
```

**`format`** (optional) highlights the live-streamed output in the TUI
instead of showing it raw/undifferentiated:

| Value | Effect |
|---|---|
| *(empty)* | Unchanged (default) |
| `logcat` | Colors each line by log level (E=red, W=yellow, I=blue, D/V=dimmed) |
| `keyvalue` | Bolds `key:`/`key=` in `key: value` lines (e.g. `dumpsys` output) |
| `packages` | Hides the `package:` prefix from `pm list packages` |

Only applies in the interactive TUI (the `run` command for scripts still
outputs unmodified raw text).

**`live_preview`** (optional, default `false`) makes an action run
automatically as soon as it's highlighted in the list - its result appears
immediately in the right-hand pane, without `enter`. Meant for fast,
side-effect-free read commands (battery status, device info). For safety,
the flag is ignored as soon as the action has `params`, `confirm: true`,
or `interactive: true` set, or is `tool: scrcpy` - such actions still only
ever run via `enter`.

**`tool: adb`** — `command` is the arguments after `adb -s <serial> ...`.
For a pipe/redirect that should run *on the device*, quote the entire part
after `shell`:

```yaml
command: 'shell "dumpsys notification | grep -A 8 MESSAGES_4"'
```

**`tool: scrcpy`** — `command` is additional scrcpy flags (empty = plain
default settings from `settings.yaml`), e.g. `--no-audio` or
`--record=out.mp4`.

**`tool: shell`** — `command` runs via the PC's system shell (`cmd.exe` on
Windows, `sh` elsewhere). `{adb}`/`{scrcpy}` are replaced with the (quoted)
paths of the tool binaries currently in use:

```yaml
command: '{adb} -s {serial} shell screencap -p /sdcard/shot.png && {adb} -s {serial} pull /sdcard/shot.png .'
```

**Placeholders** in `command`: `{serial}` (current device) as well as
`{name}` for each parameter declared in `params`. Always quote parameters
that may contain spaces (paths, package names):

```yaml
params:
  - name: apk_path
    label: "Path to the APK file"
    default: ""
command: 'install -r "{apk_path}"'
```

After saving the file, it's reloaded on the next start (or as soon as the
action list is opened again). Invalid individual entries are reported by
the healthcheck without invalidating the rest of the file.

## Extending AI mode

The base prompt lives (editable) at `<config>/ai/system_prompt.md` - it
describes the action schema to the AI and is prepended to every request. To
add another AI provider besides Claude:

1. New file in `internal/ai/` implementing the `Provider` interface
   (`Name() string`, `Available() error`, `GenerateAction(ctx, req) (ActionDraft, error)`).
2. Register it via `ai.Register("name", factoryFunc)` in an `init()`
   function.
3. Select it in `settings.yaml` under `ai.provider: name`.

No caller needs to change - the provider registry exists exactly for that.

## APK analysis (`apk-info`)

```bash
android-toolbox apk-info path/to/app.apk
android-toolbox apk-info path/to/app.apk --json
```

Reads an `.apk` file - package name, version, min/target SDK, requested
permissions, hardware features, activities (including the main/launcher
activity), and, if present, the signing certificate (APK Signing Block
v2/v3: subject, issuer, serial number, validity, SHA-256 fingerprint).

Deliberately uses **no** external tools, no aapt, no Android SDK -
`internal/apkinfo` is a pure Go implementation of Android's binary XML
format (for `AndroidManifest.xml`) and the APK Signing Block format, so
analysis works identically on Windows, Linux, and macOS (unlike, say,
[APKInfo.exe](https://github.com/Enyby/APK-Info), which only runs on
Windows). `resources.arsc` is not parsed - an `@string/...` or
`@drawable/...` reference (e.g. an app name set via a string resource
instead of a literal) therefore appears unresolved as `@0x7f...`. Only the
v2/v3 signature block is evaluated; APKs signed with v1 (JAR) only are
recognized as such, but their certificate is not decoded (that would
require parsing PKCS#7, which the more modern v2/v3 block makes
unnecessary since it embeds certificates directly as X.509 DER data).

## Emulator manager (AVDs)

Creating and running Android Virtual Devices needs a real Android SDK -
specifically `avdmanager`/`sdkmanager` (from the SDK's "command-line tools"
package) and the `emulator` binary, both of which are Java tools requiring a
JRE/JDK (11+) on `PATH` or `JAVA_HOME`. This is genuinely a different kind of
dependency than `adb`/`scrcpy`, so it's resolved and fetched separately, and
the whole feature degrades gracefully (a clear "run `emulator setup`"
message, not a startup failure) if it isn't set up yet:

1. **Prefer an existing SDK.** If `ANDROID_HOME`/`ANDROID_SDK_ROOT` is set
   and already contains a usable `cmdline-tools`/`emulator`, that's used
   directly - no separate download, and AVDs are created in the standard
   `~/.android/avd` location either way, so they're visible to/from Android
   Studio too.
2. **Otherwise, bootstrap a bundled copy.** `android-toolbox emulator setup`
   (or the healthcheck's own remediation hint) downloads Google's
   "command-line tools" package into `<config>/tools/sdk/cmdline-tools/latest`.
   Unlike `adb`/`scrcpy`, Google doesn't publish a stable "always current"
   URL for this package - the download URL/revision is pinned in
   `internal/toolsmanager/sdksources.go` and bumped by hand occasionally,
   the same staleness tolerance already accepted for scrcpy's own
   GitHub-API fallback version.
3. **Everything else is always fetched live.** The `emulator` package,
   Android platforms, and system images have no such stable direct-download
   URL at all - rather than reimplementing Google's SDK repository
   resolution, this app shells out to the bootstrapped `sdkmanager --install`
   for those, which resolves and downloads them fresh from Google's servers
   every time. A missing system image is downloaded automatically (with a
   live percentage progress bar) the first time the create wizard needs it.

From the **Emulators** tool (`ctrl+t`), the AVD list shows more than a
physical device ever would: specs parsed straight from the AVD's
`config.ini` (RAM, heap, CPU core count, internal storage, LCD density),
plus - while it's running - live simulation of GPS position, network
speed/latency profiles, and battery level/charging state, all sent through
the emulator's own control console via `adb emu ...` (no separate telnet
client needed). `n` opens the create wizard (name → device profile → system
image → RAM/CPU/internal storage → optional SD card size, downloading the
image first if needed); the device profile and system image steps support
typing to search (huh's built-in filter), since the full system-image list
can run into the dozens - the picker only ever offers system images whose
ABI actually runs on the host (an `arm64-v8a` image is filtered out on an
x86_64 host, for example, since the emulator refuses that combination
outright rather than emulating it slowly), and each entry is labeled with
its real Android version, not just its SDK API level (API level and
marketing version don't line up - "android-17" is Android 4.2, not "Android
17"). `enter` starts/stops the highlighted AVD, waiting (with a bounded
timeout) for it to actually register with `adb` rather than just firing the
process and assuming success - an early crash or a timeout is reported with
the path to that launch's own log file so a stuck/failed boot is
diagnosable instead of silent, and this in-flight state is tracked
independently of which screen you're currently on, so switching tools
mid-boot can't result in accidentally starting a second, colliding instance
of the same AVD; `d` deletes an AVD (with confirmation); `e` edits its
specs after creation.

A running AVD's name is also shown as a badge in the Devices tool's header
(next to the model/serial) whenever the currently selected device is an
emulator, since a raw `emulator-5554` serial or generic build name doesn't
say which AVD it actually is.

An AVD whose system image was later removed (e.g. via `sdkmanager`, or one
originally created by Android Studio and cleaned up since) shows up as
**Broken** with `avdmanager`'s own reason shown in place of specs, rather
than a normal-looking entry with silently blank fields - it can still be
deleted from here, but not started until its system image is reinstalled.

## Development

```bash
go build ./...
go vet ./...
go test ./...
```

Project structure (`internal/`):

```
config/        Paths, settings, first-run status
logging/       File logger + panic recovery
toolsmanager/  Download/resolution of adb/scrcpy/cmdline-tools, sdkmanager wrapper
adb/           adb client (devices, shell, battery, ...)
device/        Aggregated device info
scrcpy/        scrcpy process launch (detached, with log redirect)
avd/           AVD lifecycle (create/list/delete), specs (config.ini), launch, emulator-console simulation
actions/       Schema, YAML loader, execution (placeholders, streaming)
output/        Classifies output lines (logcat/keyvalue/packages) for highlighting
ai/            AI provider interface + Claude CLI implementation
backup/        Snapshot/list/restore of config files
install/       PATH installation (Windows registry / Unix symlink)
healthcheck/   Startup diagnostics
app/           Bubbletea TUI (one screen_*.go per screen)
```

`cmd/android-toolbox/` contains the Cobra commands; without a subcommand,
`internal/app` (the TUI) is started.

## Releases

The current version is tracked centrally in the [`VERSION`](VERSION) file
(plain Semantic Versioning, `MAJOR.MINOR.PATCH`, no leading `v`) -
`internal/buildinfo` reads it at build time via `-ldflags` (`make build`
does this automatically locally; see `Makefile`).

Every push to `main` triggers [`.github/workflows/release.yml`](.github/workflows/release.yml):

1. **Test gate**: `gofmt`, `go vet`, `go build`, `go test ./...` must all
   succeed - otherwise the run aborts without producing a release.
2. **Version determination**: commit messages since the last tag are
   parsed per [Conventional Commits](https://www.conventionalcommits.org/)
   - `feat!:`/`fix!:`/a `BREAKING CHANGE` footer mean a major bump, `feat:`
   a minor bump, anything else (including `fix:`) a patch bump. The
   `VERSION` file is bumped accordingly, committed back to `main` with
   `[skip ci]`, and tagged as `vX.Y.Z`.
3. **Cross-platform build**: `windows/amd64`, `linux/amd64`,
   `darwin/amd64`, and `darwin/arm64` are built (packaged as `.zip`/`.tar.gz`).
4. **GitHub release**: created under the new tag, all four archives are
   uploaded as attachments, and release notes are generated automatically
   from the included commits/PRs.

A normal push with no relevant commits since the last tag (e.g. a pure
docs merge with no new commits) produces no new release. For pull requests
and branches other than `main`, only [`.github/workflows/ci.yml`](.github/workflows/ci.yml)
runs instead (build/vet/test on Linux, Windows, and macOS), without
versioning or a release.

### Self-update

`android-toolbox self-update` asks the GitHub releases API for the latest
version, compares it via SemVer against the running one
(`android-toolbox version`), and, if needed, downloads the matching
release archive for the current OS/architecture. Since a running
executable can't be overwritten directly (especially on Windows), the old
file is renamed to `<name>.old` and the new one takes its place; `.old` is
automatically removed on the next start.
`--check` only checks without installing; `--yes` skips the confirmation
prompt.

The TUI also checks in the background on startup (at most once every 24
hours, with the result cached in `state.json`) for a new version and, if
needed, shows a notice on the healthcheck screen and in the dashboard
header - without delaying startup or requiring an internet connection.
The notice can be dismissed with `x` (for the current session only - it
reappears on the next start as long as the update is still pending).

The same background check (also once every 24 hours) runs for
`adb`/`scrcpy` and lands in the same notice banner; it can be disabled in
Settings via "Auto-check tool updates" - only already-cached results are
then shown, never queried again automatically. Actually applying an update
always stays manual either way (`tools update`, see the command table
above, or the Settings screen action below).

In Settings itself, the **Updates** block shows the known status for
android-toolbox, adb, and scrcpy (each "up to date", "update available", or
"not checked yet"); the **Check for updates now** row triggers a fresh
check of all three immediately - independent of the auto-check toggle
above - and refreshes this block as well as the banner on the
dashboard/healthcheck screen.
