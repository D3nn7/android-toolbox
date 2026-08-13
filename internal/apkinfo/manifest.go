package apkinfo

// ManifestInfo is what can be extracted from AndroidManifest.xml alone
// (see the apkinfo package doc comment for why resource references like
// "@string/app_name" show up unresolved, e.g. as "@0x7f0e0012").
type ManifestInfo struct {
	PackageName string `json:"packageName"`
	VersionCode int64  `json:"versionCode"`
	VersionName string `json:"versionName"`

	MinSDK     int64 `json:"minSdk"`
	TargetSDK  int64 `json:"targetSdk"`
	CompileSDK int64 `json:"compileSdk"`

	Permissions []string `json:"permissions,omitempty"`
	Features    []string `json:"features,omitempty"`

	// ApplicationLabel is the raw android:label value on <application> -
	// either a literal string or an unresolved "@0x..." resource reference.
	ApplicationLabel string   `json:"applicationLabel"`
	Activities       []string `json:"activities,omitempty"`
	// MainActivity is the activity with an <intent-filter> containing both
	// action MAIN and category LAUNCHER, if any - the one that would
	// appear in the launcher/app drawer.
	MainActivity string `json:"mainActivity"`
}

// ExtractManifestInfo walks an already-parsed AndroidManifest.xml tree
// (see ParseManifestXML) and pulls out the fields most useful for an
// "APK info" report.
func ExtractManifestInfo(root *Node) ManifestInfo {
	var info ManifestInfo

	if a, ok := root.Attr("package"); ok {
		info.PackageName = a.Value
	}
	if a, ok := root.Attr("versionCode"); ok {
		info.VersionCode = a.IntValue
	}
	if a, ok := root.Attr("versionName"); ok {
		info.VersionName = a.Value
	}
	if a, ok := root.Attr("compileSdkVersion"); ok {
		info.CompileSDK = a.IntValue
	}

	if usesSDK := root.Child("uses-sdk"); usesSDK != nil {
		if a, ok := usesSDK.Attr("minSdkVersion"); ok {
			info.MinSDK = a.IntValue
		}
		if a, ok := usesSDK.Attr("targetSdkVersion"); ok {
			info.TargetSDK = a.IntValue
		} else {
			info.TargetSDK = info.MinSDK
		}
	}

	// uses-permission-sdk-23 is an older, less common variant (permission
	// only requested/enforced from API 23 onward) - included alongside the
	// regular form since both represent "this app declares this permission".
	for _, tag := range []string{"uses-permission", "uses-permission-sdk-23"} {
		for _, p := range root.ChildrenNamed(tag) {
			if a, ok := p.Attr("name"); ok {
				info.Permissions = append(info.Permissions, a.Value)
			}
		}
	}

	for _, f := range root.ChildrenNamed("uses-feature") {
		if a, ok := f.Attr("name"); ok {
			info.Features = append(info.Features, a.Value)
		}
	}

	if app := root.Child("application"); app != nil {
		if a, ok := app.Attr("label"); ok {
			info.ApplicationLabel = a.Value
		}
		for _, act := range app.ChildrenNamed("activity") {
			name, ok := act.Attr("name")
			if !ok {
				continue
			}
			info.Activities = append(info.Activities, name.Value)
			if info.MainActivity == "" && isLauncherActivity(act) {
				info.MainActivity = name.Value
			}
		}
	}

	return info
}

// isLauncherActivity reports whether activity has an <intent-filter>
// combining action android.intent.action.MAIN with category
// android.intent.category.LAUNCHER - the standard marker for "this is the
// activity the launcher/app drawer starts".
func isLauncherActivity(activity *Node) bool {
	for _, filter := range activity.ChildrenNamed("intent-filter") {
		hasMain, hasLauncher := false, false
		for _, a := range filter.ChildrenNamed("action") {
			if v, ok := a.Attr("name"); ok && v.Value == "android.intent.action.MAIN" {
				hasMain = true
			}
		}
		for _, c := range filter.ChildrenNamed("category") {
			if v, ok := c.Attr("name"); ok && v.Value == "android.intent.category.LAUNCHER" {
				hasLauncher = true
			}
		}
		if hasMain && hasLauncher {
			return true
		}
	}
	return false
}
