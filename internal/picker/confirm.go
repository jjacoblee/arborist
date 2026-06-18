package picker

import (
	"context"
	"errors"
	"fmt"

	"github.com/charmbracelet/huh"
)

// Confirmer asks the user a yes/no question. It is separate from Selector so
// commands can prompt for confirmation (for example before deleting worktrees)
// behind a testable interface.
type Confirmer interface {
	// Confirm shows prompt and returns the user's choice. A canceled prompt
	// (Esc/Ctrl+C) returns (false, nil) — treated as "no".
	Confirm(ctx context.Context, prompt string) (bool, error)
}

// HuhConfirmer is a Confirmer backed by huh. The zero value is ready to use and
// defaults to "no".
type HuhConfirmer struct{}

// Confirm implements Confirmer.
func (HuhConfirmer) Confirm(ctx context.Context, prompt string) (bool, error) {
	var ok bool
	field := huh.NewConfirm().Title(prompt).Value(&ok)

	if err := huh.NewForm(huh.NewGroup(field)).RunWithContext(ctx); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return false, nil
		}
		return false, fmt.Errorf("run confirmation prompt: %w", err)
	}
	return ok, nil
}
