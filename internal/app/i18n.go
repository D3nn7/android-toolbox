package app

// uiText holds every piece of static UI copy the TUI renders, so the whole
// app can be re-skinned to a different language by swapping which instance
// (uiTextEN/uiTextDE) Model resolves to - see resolveUIText and
// config.Settings.Language. Fields ending in "Fmt" are fmt.Sprintf format
// strings; everything else is used verbatim.
//
// Deliberately NOT covered here: action names/descriptions/categories for
// user-created actions (they come from actions.yaml, user-authored content
// rather than UI copy - see action_translations.go for the separate,
// ID-keyed translation table covering only the built-in ones), and the
// healthcheck package's check names/details/remediations (a separate
// package with its own tests - left in German for now, see the healthcheck
// screen's own chrome below for what IS covered).
type uiText struct {
	// LanguageCode is "en"/"de" - the same value Settings.Language()
	// resolved to build this table. Cheaper than comparing e.g.
	// RunHint == uiTextDE.RunHint just to ask "is this German?", and used
	// by action_translations.go to decide whether to translate built-in
	// action names/descriptions at all.
	LanguageCode string

	AppTitle string

	// Header badges (layout.go).
	BatteryLabelFmt         string
	DeviceInfoIncompleteFmt string

	// Key help labels (footer hints), shared across screens via keyMap.
	KeyUp           string
	KeyDown         string
	KeySelect       string
	KeyRun          string
	KeyBack         string
	KeyQuit         string
	KeyRefresh      string
	KeySwitchDevice string
	KeyFilter       string
	KeyConfirm      string
	KeyCancel       string
	KeyNextField    string
	KeyHelp         string
	KeyAIAction     string
	KeyBackups      string
	KeyNextCategory string
	KeyPrevCategory string
	KeySettings     string
	KeyEditAction   string

	// Healthcheck screen (screen_health.go).
	HealthTitle          string
	HealthChecking       string
	HealthInitFailedFmt  string
	HealthFooterDone     string // "[enter] weiter   [r] erneut pruefen   [q] beenden"
	HealthFooterFailed   string // "[r] erneut pruefen   [q] beenden"
	InstallPromptTitle   string
	InstallPromptDescFmt string
	InstallPromptYes     string
	InstallPromptNo      string
	InstallOutcomeFmt    string // "Installiert nach %s"
	InstallFailedFmt     string

	// Update notices (selfupdate_check.go/toolsupdate_check.go) - shown on
	// the healthcheck screen and the dashboard header alike whenever a
	// newer android-toolbox release and/or an outdated adb/scrcpy build is
	// known. UpdateNoticeDismissHint is appended to whichever notice
	// line(s) are actually showing - see layout.go's renderUpdateNotice.
	UpdateAvailableFmt      string
	ToolUpdateAvailableFmt  string
	UpdateNoticeDismissHint string

	// Device selection (screen_deviceselect.go).
	DeviceSelectTitle             string
	DeviceStateConnected          string
	DeviceStateUnauthAuth         string
	DeviceStateOffline            string
	DeviceNoneConnected           string
	DeviceSelectFooter            string
	DeviceNotReadyErrorFmt        string
	DeviceSelectFilterPlaceholder string

	// Dashboard (screen_dashboard.go, layout.go).
	CategoryAll             string
	CategoryFallback        string // "Allgemein" - used when an action has no category
	NoActions               string
	FieldCategory           string
	FieldTool               string
	FieldCommand            string
	FieldParams             string
	FieldHint               string
	HintConfirmNeeded       string
	HintInteractive         string
	RunHint                 string // "[enter] bedienen"
	LivePreviewLabel        string
	LivePreviewLoading      string
	LivePreviewOpenHint     string
	NoOutput                string
	ScrcpyStartedFmt        string
	ActionFinishedOK        string
	ActionFinishedErrFmt    string
	ActionFilterPlaceholder string
	ActionEditableBadge     string // shown next to a user-created action's title
	ActionNotEditableStatus string

	// Action editor (screen_action_editor.go) - "e" on an editable action in
	// the dashboard. Reuses several Settings-screen fields verbatim
	// (SettingsEditingFooter, SettingsConfirmTitleFmt, SettingsConfirmYes/No,
	// SettingsChangeSavedFmt): the edit-one-field, confirm-the-change part of
	// the interaction is identical, just applied to an action's fields
	// instead of an app setting. ActionEditBrowsingFooter differs from
	// Settings' own browsing footer because it also advertises the "a"
	// (regenerate with AI) shortcut.
	ActionEditTitle            string
	ActionEditFieldName        string
	ActionEditFieldDescription string
	ActionEditFieldConfirm     string
	ActionEditFieldInteractive string
	ActionEditSaveErrorFmt     string
	ActionEditFieldRequiredMsg string
	ActionEditBrowsingFooter   string

	// Confirm dialog (screen_confirm.go).
	ConfirmTitleFmt string
	ConfirmYes      string
	ConfirmNo       string

	// Parameter form (screen_paramform.go).

	// Runner (screen_runner.go).
	RunnerRunning      string
	RunnerLine         string
	RunnerLines        string
	RunnerErrorFmt     string
	RunnerCompletedFmt string

	// AI screen (screen_ai.go).
	AITitle             string
	AIPlaceholder       string
	AIFooterInput       string
	AIGenerating        string
	AIFooterErr         string
	AISaveTitle         string
	AISaveYes           string
	AISaveNo            string
	AIFooterReformulate string
	AISavedFmt          string
	AIFooterSaved       string

	// AI screen, opened from the action editor to revise an existing action
	// (see aiScreen.editingAction) rather than create a new one - same flow,
	// different title/placeholder/save-prompt/outcome text.
	AIEditTitle       string
	AIEditPlaceholder string
	AISaveEditTitle   string
	AIEditSavedFmt    string
	AIFooterSavedEdit string

	// Recover screen (screen_recover.go).
	RecoverTitle            string
	RecoverNone             string
	RecoveredFmt            string
	RestoredEntryFmt        string // "%s (as of %s)" - name + timestamp, nested inside RecoveredFmt
	RecoverFooter           string
	RestoreTitleFmt         string
	RestoreDescription      string
	RestoreYes              string
	RestoreNo               string
	BackupFilterPlaceholder string

	// Settings screen (screen_settings.go).
	SettingsTitle                           string
	SettingsLanguageLabel                   string
	SettingsShowStartupAnimationLabel       string
	SettingsShowStartupAnimationDescription string
	SettingsShowHealthcheckLabel            string
	SettingsShowHealthcheckDescription      string
	SettingsEnabledLabel                    string
	SettingsDisabledLabel                   string
	SettingsAutoCheckToolUpdatesLabel       string
	SettingsAutoCheckToolUpdatesDescription string
	SettingsCheckForUpdatesLabel            string
	SettingsCheckForUpdatesIdleHint         string
	SettingsCheckingForUpdates              string
	SettingsCheckCompleteStatus             string
	SettingsUpdatesTitle                    string
	SettingsUpdateNotCheckedYetLabel        string
	SettingsUpdateAvailableLabel            string
	SettingsUpToDateLabel                   string
	SettingsAIProviderLabel                 string
	SettingsAIProviderDescription           string
	SettingsAICommandLabel                  string
	SettingsAICommandDescription            string
	SettingsAITimeoutLabel                  string
	SettingsInvalidNumber                   string
	SettingsProviderInstalledFmt            string // "%s (installed)"
	SettingsProviderNotInstalledFmt         string // "%s (not installed)"
	SettingsToolInfoTitle                   string
	SettingsToolADBFmt                      string
	SettingsToolScrcpyFmt                   string
	SettingsToolNotResolved                 string
	SettingsToolConfigDirFmt                string
	SettingsToolActionsFmt                  string
	SettingsSaveErrorFmt                    string

	// Settings screen interaction: select a row, enter to edit just that
	// row, enter again to confirm the change (screen_settings.go).
	SettingsBrowsingFooter  string
	SettingsEditingFooter   string
	SettingsConfirmTitleFmt string
	SettingsConfirmYes      string
	SettingsConfirmNo       string
	SettingsChangeSavedFmt  string

	// App info (version + repo link), shown on the healthcheck and settings
	// screens (screen_health.go, screen_settings.go).
	AppInfoVersionFmt string
	AppInfoRepoFmt    string

	// Tool-select screen (screen_toolselect.go) - the first thing shown
	// after the healthcheck passes, and reachable at any later point via
	// ctrl+t (see Model.Update's global key handling and KeySwitchTool
	// below).
	ToolSelectTitle  string
	ToolDevicesLabel string
	ToolDevicesDesc  string
	ToolAPKInfoLabel string
	ToolAPKInfoDesc  string
	ToolSelectFooter string
	KeySwitchTool    string

	// APK Info tool (screen_apkinfo.go) - analyzes a local .apk file (see
	// internal/apkinfo), styled/localized counterpart of
	// cmd/android-toolbox's own `apk-info` CLI command.
	APKInfoTitle                string
	APKInfoPickingFooter        string
	APKInfoResultFooter         string
	APKInfoWrongFileType        string
	APKInfoAnalyzeErrorFmt      string
	APKInfoFileLabel            string
	APKInfoSizeLabel            string
	APKInfoHashLabel            string
	APKInfoEntriesLabel         string
	APKInfoPackageLabel         string
	APKInfoVersionLabel         string
	APKInfoMinSDKLabel          string
	APKInfoTargetSDKLabel       string
	APKInfoAppLabelLabel        string
	APKInfoMainActivityLabel    string
	APKInfoPermissionsHeaderFmt string
	APKInfoFeaturesHeaderFmt    string
	APKInfoActivitiesHeaderFmt  string
	APKInfoSigningTitle         string
	APKInfoSigningSchemeLabel   string
	APKInfoCertificateFmt       string
	APKInfoCertSubjectLabel     string
	APKInfoCertIssuerLabel      string
	APKInfoCertSerialLabel      string
	APKInfoCertValidLabel       string
	APKInfoCertSHA256Label      string
	APKInfoSigningV1OnlyLabel   string
	APKInfoSigningNoneLabel     string

	// Emulator manager (screen_emulatorlist.go, screen_emulatorcreate.go) -
	// the tool-select entry, the AVD list/detail/simulation screen, and the
	// create wizard.
	ToolEmulatorsLabel string
	ToolEmulatorsDesc  string

	// Shown as a header badge (layout.go) whenever the currently selected
	// device is a running emulator, so its AVD name is visible without
	// switching to the Emulators tool.
	AVDBadgeFmt string

	EmulatorsTitle     string
	EmulatorsFooter    string
	EmulatorsNone      string
	EmulatorNoToolsMsg string

	// Detail panel fields, shared between the emulator list's specs/status
	// view and the create wizard's form labels.
	FieldTarget          string
	FieldDevice          string
	FieldPath            string
	FieldStatusRunning   string
	FieldStatusStopped   string
	FieldStatusBroken    string
	FieldName            string
	FieldSystemImage     string
	FieldSDCard          string
	FieldRAM             string
	FieldHeap            string
	FieldCPUCores        string
	FieldStorage         string
	FieldDensity         string
	FieldLatitude        string
	FieldLongitude       string
	FieldNetworkSpeed    string
	FieldNetworkDelay    string
	FieldBatteryPercent  string
	FieldBatteryCharging string
	ChargingYes          string
	ChargingNo           string

	EmulatorSpecsTitle string

	WizardFieldRequiredMsg string
	WizardFieldNumberMsg   string
	WizardSearchHint       string

	EmulatorNotRunningMsg        string
	EmulatorStartFailedNoToolMsg string
	EmulatorBrokenMsg            string
	EmulatorAlreadyStartingFmt   string
	EmulatorBootWaitingFmt       string
	EmulatorBootedFmt            string
	EmulatorBootTimeoutFmt       string
	EmulatorCrashedFmt           string
	EmulatorExitedEarlyFmt       string
	EmulatorStoppedFmt           string
	EmulatorDeletedFmt           string
	EmulatorDeleteTitleFmt       string
	EmulatorDeleteYes            string
	EmulatorDeleteNo             string
	EmulatorGPSAppliedMsg        string
	EmulatorNetworkAppliedMsg    string
	EmulatorBatteryAppliedMsg    string
	EmulatorSpecsAppliedMsg      string

	EmulatorCreateTitle         string
	EmulatorCreateDefaultDevice string
	EmulatorCreateStepFmt       string
	EmulatorCreateLoadingMsg    string
	EmulatorCreateCreatingMsg   string
	EmulatorCreateDoneFmt       string
	EmulatorCreateFooter        string
}

var uiTextEN = uiText{
	LanguageCode: "en",

	AppTitle: "android-toolbox",

	BatteryLabelFmt:         "Battery %d%%",
	DeviceInfoIncompleteFmt: "Device info incomplete: %s",

	KeyUp:           "up",
	KeyDown:         "down",
	KeySelect:       "select",
	KeyRun:          "run",
	KeyBack:         "back",
	KeyQuit:         "quit",
	KeyRefresh:      "refresh",
	KeySwitchDevice: "switch device",
	KeyFilter:       "search",
	KeyConfirm:      "yes",
	KeyCancel:       "no",
	KeyNextField:    "next field",
	KeyHelp:         "help",
	KeyAIAction:     "AI action",
	KeyBackups:      "backups",
	KeyNextCategory: "next category",
	KeyPrevCategory: "previous category",
	KeySettings:     "settings",
	KeyEditAction:   "edit",

	HealthTitle:          "android-toolbox - Healthcheck",
	HealthChecking:       "Checking environment...",
	HealthInitFailedFmt:  "Initialization failed: %s",
	HealthFooterDone:     "[enter] continue   [r] recheck   [q] quit",
	HealthFooterFailed:   "[r] recheck   [q] quit",
	InstallPromptTitle:   "Add android-toolbox to your PATH system-wide?",
	InstallPromptDescFmt: "Also installs the short alias '%s'.",
	InstallPromptYes:     "Yes, install",
	InstallPromptNo:      "No, skip",
	InstallOutcomeFmt:    "Installed to %s",
	InstallFailedFmt:     "PATH installation failed: %s",

	UpdateAvailableFmt:      "Update available: v%s (current: v%s) - run 'android-toolbox self-update'",
	ToolUpdateAvailableFmt:  "Tool update available: %s - run 'android-toolbox tools update'",
	UpdateNoticeDismissHint: "[x] dismiss",

	DeviceSelectTitle:             "android-toolbox - Select device",
	DeviceStateConnected:          "connected",
	DeviceStateUnauthAuth:         "unauthorized - confirm on the device",
	DeviceStateOffline:            "offline",
	DeviceNoneConnected:           "No devices connected. Enable USB debugging and connect a device.",
	DeviceSelectFooter:            "[enter] select   [r] refresh   [/] search   [s] settings   [ctrl+t] switch tool   [q] quit",
	DeviceNotReadyErrorFmt:        "device %s is not in state 'device' (%s)",
	DeviceSelectFilterPlaceholder: "Search devices...",

	CategoryAll:             "All",
	CategoryFallback:        "General",
	NoActions:               "No actions available.",
	FieldCategory:           "Category:",
	FieldTool:               "Tool:",
	FieldCommand:            "Command:",
	FieldParams:             "Params:",
	FieldHint:               "Note:",
	HintConfirmNeeded:       "confirmation required",
	HintInteractive:         "interactive session",
	RunHint:                 "[enter] run",
	LivePreviewLabel:        "Live preview - updates automatically as you browse",
	LivePreviewLoading:      "loading...",
	LivePreviewOpenHint:     "[enter] open as an action with full output",
	NoOutput:                "(no output)",
	ScrcpyStartedFmt:        "scrcpy started (PID %d)",
	ActionFinishedOK:        "Action finished.",
	ActionFinishedErrFmt:    "Action finished with an error: %s",
	ActionFilterPlaceholder: "Search actions (name, category, description)...",
	ActionEditableBadge:     "[Custom]",
	ActionNotEditableStatus: "Built-in actions can't be edited.",

	ActionEditTitle:            "android-toolbox - Edit Action",
	ActionEditFieldName:        "Name:",
	ActionEditFieldDescription: "Description:",
	ActionEditFieldConfirm:     "Confirm before running:",
	ActionEditFieldInteractive: "Interactive session:",
	ActionEditSaveErrorFmt:     "Could not save action: %s",
	ActionEditFieldRequiredMsg: "This field cannot be empty.",
	ActionEditBrowsingFooter:   "[up/down] select   [enter] edit   [a] edit with AI   [esc] back",

	ConfirmTitleFmt: "Really run %q?",
	ConfirmYes:      "Yes, run it",
	ConfirmNo:       "Cancel",

	RunnerRunning:      "running...",
	RunnerLine:         "line",
	RunnerLines:        "lines",
	RunnerErrorFmt:     "Error: %s (%d %s of output)",
	RunnerCompletedFmt: "Done. (%d %s of output)",

	AITitle:             "android-toolbox - Create AI action",
	AIPlaceholder:       `What should the new action do? e.g. "ADB shell command that filters dumpsys notification for MESSAGES_4"`,
	AIFooterInput:       "[ctrl+d] generate   [esc] cancel",
	AIGenerating:        "Generating action, please wait...",
	AIFooterErr:         "[r] try again   [n/esc] cancel",
	AISaveTitle:         "Save this new action?",
	AISaveYes:           "Yes, save",
	AISaveNo:            "Discard",
	AIFooterReformulate: "[r] reformulate",
	AISavedFmt:          "Action %q saved.",
	AIFooterSaved:       "[enter] back to dashboard",

	AIEditTitle:       "android-toolbox - Edit action with AI",
	AIEditPlaceholder: `What should change about this action? e.g. "Also show the device's IP address"`,
	AISaveEditTitle:   "Save these changes?",
	AIEditSavedFmt:    "Action %q updated.",
	AIFooterSavedEdit: "[enter] back to action editor",

	RecoverTitle:            "android-toolbox - Restore",
	RecoverNone:             "No backups available.",
	RecoveredFmt:            "Restored: %s",
	RestoredEntryFmt:        "%s (as of %s)",
	RecoverFooter:           "[enter] restore   [/] search   [esc] back",
	RestoreTitleFmt:         "Reset %s to the version from %s?",
	RestoreDescription:      "The current version will be backed up first.",
	RestoreYes:              "Yes, restore",
	RestoreNo:               "Cancel",
	BackupFilterPlaceholder: "Search backups...",

	SettingsTitle:                           "android-toolbox - Settings",
	SettingsLanguageLabel:                   "Language",
	SettingsShowStartupAnimationLabel:       "Startup animation",
	SettingsShowStartupAnimationDescription: "Shows an animated ASCII splash screen while the app starts.",
	SettingsShowHealthcheckLabel:            "Health check screen",
	SettingsShowHealthcheckDescription:      "When off, this screen only appears if something's wrong - a passed check goes straight to device selection.",
	SettingsEnabledLabel:                    "Enabled",
	SettingsDisabledLabel:                   "Disabled",
	SettingsAutoCheckToolUpdatesLabel:       "Auto-check tool updates",
	SettingsAutoCheckToolUpdatesDescription: "Periodically checks (never installs) whether newer adb/scrcpy builds are available.",
	SettingsCheckForUpdatesLabel:            "Check for updates now",
	SettingsCheckForUpdatesIdleHint:         "press enter",
	SettingsCheckingForUpdates:              "Checking...",
	SettingsCheckCompleteStatus:             "Check complete.",
	SettingsUpdatesTitle:                    "Updates",
	SettingsUpdateNotCheckedYetLabel:        "not checked yet",
	SettingsUpdateAvailableLabel:            "update available",
	SettingsUpToDateLabel:                   "up to date",
	SettingsAIProviderLabel:                 "AI provider",
	SettingsAIProviderDescription:           "Which AI backend generates new actions.",
	SettingsAICommandLabel:                  "AI CLI command",
	SettingsAICommandDescription:            "The executable to run for the provider above - a name on PATH (e.g. \"claude\") or a full path.",
	SettingsAITimeoutLabel:                  "AI timeout (seconds)",
	SettingsInvalidNumber:                   "must be a whole number",
	SettingsProviderInstalledFmt:            "%s (installed)",
	SettingsProviderNotInstalledFmt:         "%s (not installed)",
	SettingsToolInfoTitle:                   "Tool info",
	SettingsToolADBFmt:                      "adb:          %s (%s)",
	SettingsToolScrcpyFmt:                   "scrcpy:       %s",
	SettingsToolNotResolved:                 "not resolved",
	SettingsToolConfigDirFmt:                "config dir:   %s",
	SettingsToolActionsFmt:                  "actions.yaml: %s (%d actions)",
	SettingsSaveErrorFmt:                    "Could not save settings: %s",

	SettingsBrowsingFooter:  "[up/down] select   [enter] edit   [esc] back",
	SettingsEditingFooter:   "[enter] confirm change   [esc] cancel",
	SettingsConfirmTitleFmt: "Change %s to %q?",
	SettingsConfirmYes:      "Yes, change it",
	SettingsConfirmNo:       "Keep current value",
	SettingsChangeSavedFmt:  "%s updated.",

	AppInfoVersionFmt: "android-toolbox %s (commit %s)",
	AppInfoRepoFmt:    "Repository: %s",

	ToolSelectTitle:  "android-toolbox - Choose a tool",
	ToolDevicesLabel: "Devices",
	ToolDevicesDesc:  "Control a connected Android device via adb/scrcpy",
	ToolAPKInfoLabel: "APK Info",
	ToolAPKInfoDesc:  "Analyze an .apk file (package, version, permissions, signing)",
	ToolSelectFooter: "[up/down] select   [enter] open   [q] quit",
	KeySwitchTool:    "switch tool",

	APKInfoTitle:                "android-toolbox - APK Info",
	APKInfoPickingFooter:        "[enter] open/select   [esc] back   [ctrl+t] switch tool   [q] quit",
	APKInfoResultFooter:         "[up/down] scroll   [esc] choose another file   [ctrl+t] switch tool   [q] quit",
	APKInfoWrongFileType:        "Only .apk files can be selected.",
	APKInfoAnalyzeErrorFmt:      "Could not analyze this file: %s",
	APKInfoFileLabel:            "File:",
	APKInfoSizeLabel:            "Size:",
	APKInfoHashLabel:            "SHA-256:",
	APKInfoEntriesLabel:         "Entries:",
	APKInfoPackageLabel:         "Package:",
	APKInfoVersionLabel:         "Version:",
	APKInfoMinSDKLabel:          "Min SDK:",
	APKInfoTargetSDKLabel:       "Target SDK:",
	APKInfoAppLabelLabel:        "App label:",
	APKInfoMainActivityLabel:    "Main activity:",
	APKInfoPermissionsHeaderFmt: "Permissions (%d):",
	APKInfoFeaturesHeaderFmt:    "Features (%d):",
	APKInfoActivitiesHeaderFmt:  "Activities (%d):",
	APKInfoSigningTitle:         "Signing",
	APKInfoSigningSchemeLabel:   "Scheme:",
	APKInfoCertificateFmt:       "Certificate #%d",
	APKInfoCertSubjectLabel:     "Subject:",
	APKInfoCertIssuerLabel:      "Issuer:",
	APKInfoCertSerialLabel:      "Serial:",
	APKInfoCertValidLabel:       "Valid:",
	APKInfoCertSHA256Label:      "SHA-256:",
	APKInfoSigningV1OnlyLabel:   "v1 (JAR signature) - certificate details not decoded",
	APKInfoSigningNoneLabel:     "No signature block found (unsigned or unknown format)",

	ToolEmulatorsLabel: "Emulators",
	ToolEmulatorsDesc:  "Create, manage, and simulate Android Virtual Devices (AVDs)",

	AVDBadgeFmt: "AVD: %s",

	EmulatorsTitle:     "android-toolbox - Emulators",
	EmulatorsFooter:    "[n] new   [enter] start/stop   [d] delete   [e] edit specs   [g] GPS   [w] network   [b] battery   [esc] back   [ctrl+t] switch tool   [q] quit",
	EmulatorsNone:      "No AVDs defined yet - press 'n' to create one.",
	EmulatorNoToolsMsg: "sdkmanager/avdmanager not available - run 'android-toolbox emulator setup'",

	FieldTarget:          "Target:",
	FieldDevice:          "Device:",
	FieldPath:            "Path:",
	FieldStatusRunning:   "Running",
	FieldStatusStopped:   "Stopped",
	FieldStatusBroken:    "Broken",
	FieldName:            "Name:",
	FieldSystemImage:     "System image:",
	FieldSDCard:          "SD card (MB):",
	FieldRAM:             "RAM (MB):",
	FieldHeap:            "Heap (MB):",
	FieldCPUCores:        "CPU cores:",
	FieldStorage:         "Internal storage (MB):",
	FieldDensity:         "LCD density (dpi):",
	FieldLatitude:        "Latitude:",
	FieldLongitude:       "Longitude:",
	FieldNetworkSpeed:    "Network speed profile:",
	FieldNetworkDelay:    "Network delay profile:",
	FieldBatteryPercent:  "Battery percent:",
	FieldBatteryCharging: "Charging:",
	ChargingYes:          "Yes",
	ChargingNo:           "No",

	EmulatorSpecsTitle: "Specs",

	WizardFieldRequiredMsg: "This field is required.",
	WizardFieldNumberMsg:   "Enter a number.",
	WizardSearchHint:       "Type to search - esc clears the search, ↑/↓ to browse.",

	EmulatorNotRunningMsg:        "This AVD is not currently running.",
	EmulatorStartFailedNoToolMsg: "The emulator binary is not available - run 'android-toolbox emulator setup'.",
	EmulatorBrokenMsg:            "%s can't be started: %s",
	EmulatorAlreadyStartingFmt:   "%s is already starting - please wait for it to boot.",
	EmulatorBootWaitingFmt:       "%s is starting - waiting for it to boot (this can take a while, especially without hardware acceleration)...",
	EmulatorBootedFmt:            "%s is now running.",
	EmulatorBootTimeoutFmt:       "%s hasn't come up after 90s - it may still be booting (check Log: %s), or hardware acceleration (HAXM/WHPX/KVM) may not be set up.",
	EmulatorCrashedFmt:           "%s failed to start: %s (see Log: %s)",
	EmulatorExitedEarlyFmt:       "%s exited before finishing boot (see Log: %s)",
	EmulatorStoppedFmt:           "%s stopped.",
	EmulatorDeletedFmt:           "%s deleted.",
	EmulatorDeleteTitleFmt:       "Delete AVD %q?",
	EmulatorDeleteYes:            "Yes, delete it",
	EmulatorDeleteNo:             "Cancel",
	EmulatorGPSAppliedMsg:        "GPS position applied.",
	EmulatorNetworkAppliedMsg:    "Network profile applied.",
	EmulatorBatteryAppliedMsg:    "Battery state applied.",
	EmulatorSpecsAppliedMsg:      "Specs saved - restart the AVD for changes to take effect.",

	EmulatorCreateTitle:         "android-toolbox - New Emulator",
	EmulatorCreateDefaultDevice: "(avdmanager default)",
	EmulatorCreateStepFmt:       "Step %d/%d",
	EmulatorCreateLoadingMsg:    "Fetching device profiles and system images...",
	EmulatorCreateCreatingMsg:   "Creating AVD...",
	EmulatorCreateDoneFmt:       "AVD %q created.",
	EmulatorCreateFooter:        "[enter]/[esc] back to the emulator list",
}

var uiTextDE = uiText{
	LanguageCode: "de",

	AppTitle: "android-toolbox",

	BatteryLabelFmt:         "Akku %d%%",
	DeviceInfoIncompleteFmt: "Geräteinfo unvollständig: %s",

	KeyUp:           "hoch",
	KeyDown:         "runter",
	KeySelect:       "auswählen",
	KeyRun:          "bedienen",
	KeyBack:         "zurück",
	KeyQuit:         "beenden",
	KeyRefresh:      "aktualisieren",
	KeySwitchDevice: "Gerät wechseln",
	KeyFilter:       "suchen",
	KeyConfirm:      "ja",
	KeyCancel:       "nein",
	KeyNextField:    "nächstes Feld",
	KeyHelp:         "Hilfe",
	KeyAIAction:     "KI-Aktion",
	KeyBackups:      "Backups",
	KeyNextCategory: "nächste Kategorie",
	KeyPrevCategory: "vorherige Kategorie",
	KeySettings:     "Einstellungen",
	KeyEditAction:   "bearbeiten",

	HealthTitle:          "android-toolbox - Healthcheck",
	HealthChecking:       "Umgebung wird geprüft...",
	HealthInitFailedFmt:  "Initialisierung fehlgeschlagen: %s",
	HealthFooterDone:     "[enter] weiter   [r] erneut prüfen   [q] beenden",
	HealthFooterFailed:   "[r] erneut prüfen   [q] beenden",
	InstallPromptTitle:   "android-toolbox systemweit ins PATH aufnehmen?",
	InstallPromptDescFmt: "Installiert zusätzlich den kurzen Alias \"%s\".",
	InstallPromptYes:     "Ja, installieren",
	InstallPromptNo:      "Nein, überspringen",
	InstallOutcomeFmt:    "Installiert nach %s",
	InstallFailedFmt:     "PATH-Installation fehlgeschlagen: %s",

	UpdateAvailableFmt:      "Update verfügbar: v%s (aktuell: v%s) - führe 'android-toolbox self-update' aus",
	ToolUpdateAvailableFmt:  "Tool-Update verfügbar: %s - führe 'android-toolbox tools update' aus",
	UpdateNoticeDismissHint: "[x] ausblenden",

	DeviceSelectTitle:             "android-toolbox - Geräteauswahl",
	DeviceStateConnected:          "verbunden",
	DeviceStateUnauthAuth:         "nicht autorisiert - Bestätigung auf dem Gerät erforderlich",
	DeviceStateOffline:            "offline",
	DeviceNoneConnected:           "Keine Geräte verbunden. USB-Debugging aktivieren und Gerät anschließen.",
	DeviceSelectFooter:            "[enter] auswählen   [r] aktualisieren   [/] suchen   [s] einstellungen   [ctrl+t] Werkzeug wechseln   [q] beenden",
	DeviceNotReadyErrorFmt:        "Gerät %s befindet sich nicht im Zustand \"device\" (%s).",
	DeviceSelectFilterPlaceholder: "Gerät suchen...",

	CategoryAll:             "Alle",
	CategoryFallback:        "Allgemein",
	NoActions:               "Keine Aktionen vorhanden.",
	FieldCategory:           "Kategorie:",
	FieldTool:               "Tool:",
	FieldCommand:            "Befehl:",
	FieldParams:             "Parameter:",
	FieldHint:               "Hinweis:",
	HintConfirmNeeded:       "Bestätigung nötig",
	HintInteractive:         "interaktive Sitzung",
	RunHint:                 "[enter] bedienen",
	LivePreviewLabel:        "Live-Vorschau - aktualisiert sich automatisch beim Markieren",
	LivePreviewLoading:      "lädt...",
	LivePreviewOpenHint:     "[enter] als Aktion mit vollständiger Ausgabe öffnen",
	NoOutput:                "(keine Ausgabe)",
	ScrcpyStartedFmt:        "scrcpy gestartet (PID %d)",
	ActionFinishedOK:        "Aktion abgeschlossen.",
	ActionFinishedErrFmt:    "Aktion mit Fehler beendet: %s",
	ActionFilterPlaceholder: "Aktion suchen (Name, Kategorie, Beschreibung)...",
	ActionEditableBadge:     "[Eigene]",
	ActionNotEditableStatus: "Eingebaute Aktionen können nicht bearbeitet werden.",

	ActionEditTitle:            "android-toolbox - Aktion bearbeiten",
	ActionEditFieldName:        "Name:",
	ActionEditFieldDescription: "Beschreibung:",
	ActionEditFieldConfirm:     "Bestätigung vor Ausführung:",
	ActionEditFieldInteractive: "Interaktive Sitzung:",
	ActionEditSaveErrorFmt:     "Aktion konnte nicht gespeichert werden: %s",
	ActionEditFieldRequiredMsg: "Dieses Feld darf nicht leer sein.",
	ActionEditBrowsingFooter:   "[hoch/runter] auswählen   [enter] bearbeiten   [a] mit KI bearbeiten   [esc] zurück",

	ConfirmTitleFmt: "%q wirklich ausführen?",
	ConfirmYes:      "Ja, ausführen",
	ConfirmNo:       "Abbrechen",

	RunnerRunning:      "läuft...",
	RunnerLine:         "Zeile",
	RunnerLines:        "Zeilen",
	RunnerErrorFmt:     "Fehler: %s (%d %s Ausgabe)",
	RunnerCompletedFmt: "Abgeschlossen. (%d %s Ausgabe)",

	AITitle:             "android-toolbox - KI-Aktion erstellen",
	AIPlaceholder:       `Was soll die neue Aktion tun? Zum Beispiel: "ADB-Shell-Befehl, der dumpsys notification nach MESSAGES_4 filtert"`,
	AIFooterInput:       "[ctrl+d] generieren   [esc] abbrechen",
	AIGenerating:        "Aktion wird generiert, bitte warten...",
	AIFooterErr:         "[r] erneut versuchen   [n/esc] abbrechen",
	AISaveTitle:         "Neue Aktion speichern?",
	AISaveYes:           "Ja, speichern",
	AISaveNo:            "Verwerfen",
	AIFooterReformulate: "[r] neu formulieren",
	AISavedFmt:          "Aktion %q gespeichert.",
	AIFooterSaved:       "[enter] zurück zum Dashboard",

	AIEditTitle:       "android-toolbox - Aktion mit KI bearbeiten",
	AIEditPlaceholder: `Was soll an dieser Aktion geändert werden? Zum Beispiel: "Auch die IP-Adresse des Geräts anzeigen"`,
	AISaveEditTitle:   "Diese Änderungen speichern?",
	AIEditSavedFmt:    "Aktion %q aktualisiert.",
	AIFooterSavedEdit: "[enter] zurück zum Aktions-Editor",

	RecoverTitle:            "android-toolbox - Wiederherstellen",
	RecoverNone:             "Keine Backups vorhanden.",
	RecoveredFmt:            "Wiederhergestellt: %s",
	RestoredEntryFmt:        "%s (Stand vom %s)",
	RecoverFooter:           "[enter] wiederherstellen   [/] suchen   [esc] zurück",
	RestoreTitleFmt:         "%s auf den Stand vom %s zurücksetzen?",
	RestoreDescription:      "Der aktuelle Stand wird vorher ebenfalls gesichert.",
	RestoreYes:              "Ja, wiederherstellen",
	RestoreNo:               "Abbrechen",
	BackupFilterPlaceholder: "Backup suchen...",

	SettingsTitle:                           "android-toolbox - Einstellungen",
	SettingsLanguageLabel:                   "Sprache",
	SettingsShowStartupAnimationLabel:       "Startanimation",
	SettingsShowStartupAnimationDescription: "Zeigt beim Start einen animierten ASCII-Begrüßungsbildschirm.",
	SettingsShowHealthcheckLabel:            "Healthcheck-Anzeige",
	SettingsShowHealthcheckDescription:      "Bei Deaktivierung erscheint dieser Bildschirm nur, wenn etwas nicht stimmt - bei erfolgreicher Prüfung geht es direkt zur Geräteauswahl.",
	SettingsEnabledLabel:                    "Aktiviert",
	SettingsDisabledLabel:                   "Deaktiviert",
	SettingsAutoCheckToolUpdatesLabel:       "Automatische Tool-Update-Prüfung",
	SettingsAutoCheckToolUpdatesDescription: "Prüft periodisch (ohne zu installieren), ob neuere adb/scrcpy-Versionen verfügbar sind.",
	SettingsCheckForUpdatesLabel:            "Jetzt auf Updates prüfen",
	SettingsCheckForUpdatesIdleHint:         "Enter drücken",
	SettingsCheckingForUpdates:              "Prüfe...",
	SettingsCheckCompleteStatus:             "Prüfung abgeschlossen.",
	SettingsUpdatesTitle:                    "Updates",
	SettingsUpdateNotCheckedYetLabel:        "noch nicht geprüft",
	SettingsUpdateAvailableLabel:            "Update verfügbar",
	SettingsUpToDateLabel:                   "aktuell",
	SettingsAIProviderLabel:                 "KI-Provider",
	SettingsAIProviderDescription:           "Legt fest, welches KI-Backend neue Aktionen erstellt.",
	SettingsAICommandLabel:                  "KI-Befehl",
	SettingsAICommandDescription:            "Der Befehl, der für den oben ausgewählten Provider ausgeführt wird - ein Name im PATH (z. B. \"claude\") oder ein vollständiger Pfad.",
	SettingsAITimeoutLabel:                  "KI-Timeout (Sekunden)",
	SettingsInvalidNumber:                   "Bitte eine ganze Zahl eingeben.",
	SettingsProviderInstalledFmt:            "%s (installiert)",
	SettingsProviderNotInstalledFmt:         "%s (nicht installiert)",
	SettingsToolInfoTitle:                   "Tool-Infos",
	SettingsToolADBFmt:                      "adb:          %s (%s)",
	SettingsToolScrcpyFmt:                   "scrcpy:       %s",
	SettingsToolNotResolved:                 "nicht gefunden",
	SettingsToolConfigDirFmt:                "Konfig-Verzeichnis: %s",
	SettingsToolActionsFmt:                  "actions.yaml: %s (%d Aktionen)",
	SettingsSaveErrorFmt:                    "Einstellungen konnten nicht gespeichert werden: %s",

	SettingsBrowsingFooter:  "[hoch/runter] auswählen   [enter] bearbeiten   [esc] zurück",
	SettingsEditingFooter:   "[enter] Änderung bestätigen   [esc] abbrechen",
	SettingsConfirmTitleFmt: "%s wirklich auf %q ändern?",
	SettingsConfirmYes:      "Ja, ändern",
	SettingsConfirmNo:       "Aktuellen Wert behalten",
	SettingsChangeSavedFmt:  "%s aktualisiert.",

	AppInfoVersionFmt: "android-toolbox %s (Commit %s)",
	AppInfoRepoFmt:    "Repository: %s",

	ToolSelectTitle:  "android-toolbox - Werkzeug wählen",
	ToolDevicesLabel: "Geräte",
	ToolDevicesDesc:  "Ein verbundenes Android-Gerät über adb/scrcpy steuern",
	ToolAPKInfoLabel: "APK Info",
	ToolAPKInfoDesc:  "Eine .apk-Datei analysieren (Paket, Version, Berechtigungen, Signatur)",
	ToolSelectFooter: "[hoch/runter] auswählen   [enter] öffnen   [q] beenden",
	KeySwitchTool:    "Werkzeug wechseln",

	APKInfoTitle:                "android-toolbox - APK Info",
	APKInfoPickingFooter:        "[enter] öffnen/auswählen   [esc] zurück   [ctrl+t] Werkzeug wechseln   [q] beenden",
	APKInfoResultFooter:         "[hoch/runter] scrollen   [esc] andere Datei wählen   [ctrl+t] Werkzeug wechseln   [q] beenden",
	APKInfoWrongFileType:        "Es können nur .apk-Dateien ausgewählt werden.",
	APKInfoAnalyzeErrorFmt:      "Datei konnte nicht analysiert werden: %s",
	APKInfoFileLabel:            "Datei:",
	APKInfoSizeLabel:            "Größe:",
	APKInfoHashLabel:            "SHA-256:",
	APKInfoEntriesLabel:         "Einträge:",
	APKInfoPackageLabel:         "Paket:",
	APKInfoVersionLabel:         "Version:",
	APKInfoMinSDKLabel:          "Min SDK:",
	APKInfoTargetSDKLabel:       "Target SDK:",
	APKInfoAppLabelLabel:        "Anzeigename:",
	APKInfoMainActivityLabel:    "Start-Activity:",
	APKInfoPermissionsHeaderFmt: "Berechtigungen (%d):",
	APKInfoFeaturesHeaderFmt:    "Hardware-/Feature-Anforderungen (%d):",
	APKInfoActivitiesHeaderFmt:  "Activities (%d):",
	APKInfoSigningTitle:         "Signatur",
	APKInfoSigningSchemeLabel:   "Schema:",
	APKInfoCertificateFmt:       "Zertifikat #%d",
	APKInfoCertSubjectLabel:     "Subject:",
	APKInfoCertIssuerLabel:      "Aussteller:",
	APKInfoCertSerialLabel:      "Seriennummer:",
	APKInfoCertValidLabel:       "Gültig:",
	APKInfoCertSHA256Label:      "SHA-256:",
	APKInfoSigningV1OnlyLabel:   "v1 (JAR-Signatur) - Zertifikatsdetails werden nicht entschlüsselt",
	APKInfoSigningNoneLabel:     "Kein Signatur-Block gefunden (unsigniert oder unbekanntes Format)",

	ToolEmulatorsLabel: "Emulatoren",
	ToolEmulatorsDesc:  "Android Virtual Devices (AVDs) erstellen, verwalten und simulieren",

	AVDBadgeFmt: "AVD: %s",

	EmulatorsTitle:     "android-toolbox - Emulatoren",
	EmulatorsFooter:    "[n] neu   [enter] starten/stoppen   [d] löschen   [e] Specs bearbeiten   [g] GPS   [w] Netzwerk   [b] Akku   [esc] zurück   [ctrl+t] Werkzeug wechseln   [q] beenden",
	EmulatorsNone:      "Noch keine AVDs angelegt - 'n' drücken, um eines zu erstellen.",
	EmulatorNoToolsMsg: "sdkmanager/avdmanager nicht verfügbar - 'android-toolbox emulator setup' ausführen",

	FieldTarget:          "Target:",
	FieldDevice:          "Gerät:",
	FieldPath:            "Pfad:",
	FieldStatusRunning:   "Läuft",
	FieldStatusStopped:   "Gestoppt",
	FieldStatusBroken:    "Defekt",
	FieldName:            "Name:",
	FieldSystemImage:     "System-Image:",
	FieldSDCard:          "SD-Karte (MB):",
	FieldRAM:             "RAM (MB):",
	FieldHeap:            "Heap (MB):",
	FieldCPUCores:        "CPU-Kerne:",
	FieldStorage:         "Interner Speicher (MB):",
	FieldDensity:         "LCD-Dichte (dpi):",
	FieldLatitude:        "Breitengrad:",
	FieldLongitude:       "Längengrad:",
	FieldNetworkSpeed:    "Netzwerk-Geschwindigkeitsprofil:",
	FieldNetworkDelay:    "Netzwerk-Verzögerungsprofil:",
	FieldBatteryPercent:  "Akkustand (%):",
	FieldBatteryCharging: "Lädt:",
	ChargingYes:          "Ja",
	ChargingNo:           "Nein",

	EmulatorSpecsTitle: "Specs",

	WizardFieldRequiredMsg: "Dieses Feld wird benötigt.",
	WizardFieldNumberMsg:   "Eine Zahl eingeben.",
	WizardSearchHint:       "Tippen zum Suchen - esc löscht die Suche, ↑/↓ zum Blättern.",

	EmulatorNotRunningMsg:        "Dieses AVD läuft aktuell nicht.",
	EmulatorStartFailedNoToolMsg: "Die emulator-Binary ist nicht verfügbar - 'android-toolbox emulator setup' ausführen.",
	EmulatorBrokenMsg:            "%s kann nicht gestartet werden: %s",
	EmulatorAlreadyStartingFmt:   "%s startet bereits - bitte warten, bis es hochgefahren ist.",
	EmulatorBootWaitingFmt:       "%s startet - warte auf den Bootvorgang (kann dauern, besonders ohne Hardware-Beschleunigung)...",
	EmulatorBootedFmt:            "%s läuft jetzt.",
	EmulatorBootTimeoutFmt:       "%s ist nach 90s nicht erschienen - der Bootvorgang läuft evtl. noch (siehe Log: %s), oder Hardware-Beschleunigung (HAXM/WHPX/KVM) ist nicht eingerichtet.",
	EmulatorCrashedFmt:           "%s konnte nicht gestartet werden: %s (siehe Log: %s)",
	EmulatorExitedEarlyFmt:       "%s wurde vor Abschluss des Bootvorgangs beendet (siehe Log: %s)",
	EmulatorStoppedFmt:           "%s gestoppt.",
	EmulatorDeletedFmt:           "%s gelöscht.",
	EmulatorDeleteTitleFmt:       "AVD %q löschen?",
	EmulatorDeleteYes:            "Ja, löschen",
	EmulatorDeleteNo:             "Abbrechen",
	EmulatorGPSAppliedMsg:        "GPS-Position übernommen.",
	EmulatorNetworkAppliedMsg:    "Netzwerkprofil übernommen.",
	EmulatorBatteryAppliedMsg:    "Akkustatus übernommen.",
	EmulatorSpecsAppliedMsg:      "Specs gespeichert - AVD neu starten, damit die Änderung wirkt.",

	EmulatorCreateTitle:         "android-toolbox - Neuer Emulator",
	EmulatorCreateDefaultDevice: "(avdmanager-Standard)",
	EmulatorCreateStepFmt:       "Schritt %d/%d",
	EmulatorCreateLoadingMsg:    "Geräteprofile und System-Images werden geladen...",
	EmulatorCreateCreatingMsg:   "AVD wird erstellt...",
	EmulatorCreateDoneFmt:       "AVD %q erstellt.",
	EmulatorCreateFooter:        "[enter]/[esc] zurück zur Emulator-Liste",
}

// resolveUIText picks the UI string table for a Settings.Language() value
// ("en"/"de" - already normalized, see config.Settings.Language).
func resolveUIText(language string) uiText {
	if language == "de" {
		return uiTextDE
	}
	return uiTextEN
}
