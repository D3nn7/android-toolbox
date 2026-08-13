package apkinfo

import (
	"reflect"
	"sort"
	"testing"
)

func TestExtractManifestInfo(t *testing.T) {
	data := newAXMLBuilder().build(sampleManifestTree())
	root, err := ParseManifestXML(data)
	if err != nil {
		t.Fatalf("ParseManifestXML: %v", err)
	}

	info := ExtractManifestInfo(root)

	if info.PackageName != "com.example.app" {
		t.Errorf("PackageName = %q, want com.example.app", info.PackageName)
	}
	if info.VersionCode != 42 {
		t.Errorf("VersionCode = %d, want 42", info.VersionCode)
	}
	if info.VersionName != "1.2.3" {
		t.Errorf("VersionName = %q, want 1.2.3", info.VersionName)
	}
	if info.MinSDK != 21 {
		t.Errorf("MinSDK = %d, want 21", info.MinSDK)
	}
	if info.TargetSDK != 33 {
		t.Errorf("TargetSDK = %d, want 33", info.TargetSDK)
	}

	wantPerms := []string{"android.permission.INTERNET", "android.permission.CAMERA"}
	gotPerms := append([]string(nil), info.Permissions...)
	sort.Strings(gotPerms)
	sort.Strings(wantPerms)
	if !reflect.DeepEqual(gotPerms, wantPerms) {
		t.Errorf("Permissions = %v, want %v", info.Permissions, wantPerms)
	}

	if len(info.Features) != 1 || info.Features[0] != "android.hardware.camera" {
		t.Errorf("Features = %v, want [android.hardware.camera]", info.Features)
	}

	if info.ApplicationLabel != "Example App" {
		t.Errorf("ApplicationLabel = %q, want Example App", info.ApplicationLabel)
	}

	wantActivities := []string{".MainActivity", ".SettingsActivity"}
	gotActivities := append([]string(nil), info.Activities...)
	sort.Strings(gotActivities)
	sort.Strings(wantActivities)
	if !reflect.DeepEqual(gotActivities, wantActivities) {
		t.Errorf("Activities = %v, want %v", info.Activities, wantActivities)
	}

	if info.MainActivity != ".MainActivity" {
		t.Errorf("MainActivity = %q, want .MainActivity", info.MainActivity)
	}
}

// TestExtractManifestInfoTargetSDKFallsBackToMinSDK proves an app that only
// declares minSdkVersion (no explicit targetSdkVersion, technically legal
// though unusual in practice) doesn't report TargetSDK as 0.
func TestExtractManifestInfoTargetSDKFallsBackToMinSDK(t *testing.T) {
	tree := testElem{
		Name: "manifest",
		Attrs: []testAttr{
			strAttr("package", "com.example.minimal"),
		},
		Children: []testElem{
			{Name: "uses-sdk", Attrs: []testAttr{intAttr("minSdkVersion", 24)}},
		},
	}
	data := newAXMLBuilder().build(tree)
	root, err := ParseManifestXML(data)
	if err != nil {
		t.Fatalf("ParseManifestXML: %v", err)
	}

	info := ExtractManifestInfo(root)
	if info.TargetSDK != 24 {
		t.Errorf("TargetSDK = %d, want 24 (falling back to MinSDK)", info.TargetSDK)
	}
}

// TestExtractManifestInfoNoLauncherActivity proves an app with activities
// but no MAIN/LAUNCHER intent-filter correctly reports no main activity,
// rather than guessing the first one.
func TestExtractManifestInfoNoLauncherActivity(t *testing.T) {
	tree := testElem{
		Name:  "manifest",
		Attrs: []testAttr{strAttr("package", "com.example.nolauncher")},
		Children: []testElem{
			{
				Name:  "application",
				Attrs: []testAttr{strAttr("label", "No Launcher")},
				Children: []testElem{
					{Name: "activity", Attrs: []testAttr{strAttr("name", ".HelperActivity")}},
				},
			},
		},
	}
	data := newAXMLBuilder().build(tree)
	root, err := ParseManifestXML(data)
	if err != nil {
		t.Fatalf("ParseManifestXML: %v", err)
	}

	info := ExtractManifestInfo(root)
	if info.MainActivity != "" {
		t.Errorf("MainActivity = %q, want empty (no MAIN/LAUNCHER intent-filter present)", info.MainActivity)
	}
}

// TestExtractManifestInfoRequiresBothActionAndCategoryInSameFilter proves
// MAIN and LAUNCHER only count when they come from the *same*
// intent-filter, not merely somewhere within the same activity.
func TestExtractManifestInfoRequiresBothActionAndCategoryInSameFilter(t *testing.T) {
	tree := testElem{
		Name:  "manifest",
		Attrs: []testAttr{strAttr("package", "com.example.split")},
		Children: []testElem{
			{
				Name: "application",
				Children: []testElem{
					{
						Name:  "activity",
						Attrs: []testAttr{strAttr("name", ".SplitActivity")},
						Children: []testElem{
							{Name: "intent-filter", Children: []testElem{
								{Name: "action", Attrs: []testAttr{strAttr("name", "android.intent.action.MAIN")}},
							}},
							{Name: "intent-filter", Children: []testElem{
								{Name: "category", Attrs: []testAttr{strAttr("name", "android.intent.category.LAUNCHER")}},
							}},
						},
					},
				},
			},
		},
	}
	data := newAXMLBuilder().build(tree)
	root, err := ParseManifestXML(data)
	if err != nil {
		t.Fatalf("ParseManifestXML: %v", err)
	}

	info := ExtractManifestInfo(root)
	if info.MainActivity != "" {
		t.Errorf("MainActivity = %q, want empty (MAIN and LAUNCHER are in separate intent-filters)", info.MainActivity)
	}
}
