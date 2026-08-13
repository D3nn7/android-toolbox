// Package ai generates new actions from a free-text request by delegating
// to a locally installed AI CLI. The concrete CLI used is pluggable via the
// Provider interface and a name-based registry, so a different AI tool can
// be added later without touching callers.
package ai

import (
	"context"

	"android-toolbox/internal/actions"
)

// Param mirrors actions.Param in the AI's JSON response contract.
type Param struct {
	Name    string `json:"name"`
	Label   string `json:"label"`
	Default string `json:"default"`
}

// ActionDraft is the JSON-shaped action a provider generates from a user
// request. It mirrors actions.Action but stays decoupled from it (plain
// strings, no validation tags) since it represents unverified AI output.
type ActionDraft struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Category    string  `json:"category"`
	Tool        string  `json:"tool"`
	Command     string  `json:"command"`
	Params      []Param `json:"params"`
	Confirm     bool    `json:"confirm"`
	Interactive bool    `json:"interactive"`
}

// ToAction converts the draft into a real actions.Action. The caller is
// still responsible for validating it (e.g. via actions.Append) before
// persisting it - this conversion does not itself guarantee validity.
func (d ActionDraft) ToAction() actions.Action {
	params := make([]actions.Param, len(d.Params))
	for i, p := range d.Params {
		params[i] = actions.Param{Name: p.Name, Label: p.Label, Default: p.Default}
	}
	return actions.Action{
		ID:          d.ID,
		Name:        d.Name,
		Description: d.Description,
		Category:    d.Category,
		Tool:        actions.Tool(d.Tool),
		Command:     d.Command,
		Params:      params,
		Confirm:     d.Confirm,
		Interactive: d.Interactive,
	}
}

// ActionDraftFromAction is ToAction's inverse: it lets a caller hand an
// existing action to a provider as edit context via GenerateRequest.
func ActionDraftFromAction(a actions.Action) ActionDraft {
	params := make([]Param, len(a.Params))
	for i, p := range a.Params {
		params[i] = Param{Name: p.Name, Label: p.Label, Default: p.Default}
	}
	return ActionDraft{
		ID:          a.ID,
		Name:        a.Name,
		Description: a.Description,
		Category:    a.Category,
		Tool:        string(a.Tool),
		Command:     a.Command,
		Params:      params,
		Confirm:     a.Confirm,
		Interactive: a.Interactive,
	}
}

// GenerateRequest carries what the provider needs to produce a good draft.
type GenerateRequest struct {
	UserPrompt  string
	ExistingIDs []string

	// ExistingAction is set when the user wants an existing action revised
	// rather than a new one created. Providers should keep its ID unchanged
	// and only adjust what the prompt actually asks for.
	ExistingAction *ActionDraft
}

// Provider is one AI backend capable of turning a free-text request into an
// ActionDraft.
type Provider interface {
	Name() string
	Available() error
	GenerateAction(ctx context.Context, req GenerateRequest) (ActionDraft, error)
}
