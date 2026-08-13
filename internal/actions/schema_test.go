package actions

import "testing"

func TestLivePreviewEligible(t *testing.T) {
	cases := []struct {
		name string
		a    Action
		want bool
	}{
		{"plain live-preview action", Action{LivePreview: true, Tool: ToolADB}, true},
		{"flag not set", Action{Tool: ToolADB}, false},
		{"needs params", Action{LivePreview: true, Tool: ToolADB, Params: []Param{{Name: "x"}}}, false},
		{"needs confirmation", Action{LivePreview: true, Tool: ToolADB, Confirm: true}, false},
		{"interactive session", Action{LivePreview: true, Tool: ToolADB, Interactive: true}, false},
		{"scrcpy action", Action{LivePreview: true, Tool: ToolScrcpy}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.a.LivePreviewEligible(); got != c.want {
				t.Fatalf("LivePreviewEligible() = %v, want %v", got, c.want)
			}
		})
	}
}
