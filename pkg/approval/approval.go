// Package approval waits for physical-button approval of AI tool calls.
package approval

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/ghofuobudgeghikhdrhiehdihebd/airlock/pkg/button"
	"github.com/ghofuobudgeghikhdrhiehdihebd/airlock/pkg/display"
	"github.com/ghofuobudgeghikhdrhiehdihebd/airlock/pkg/schemas"
)

// Interface approves or rejects one tool request.
type Interface interface {
	Approve(context.Context, schemas.McpCallRequest) (bool, error)
}

// Buttons renders a tool request and waits for a physical decision. Confirm
// corresponds to the right button; Deny corresponds to the left button.
type Buttons struct {
	Confirm *button.Button
	Deny    *button.Button
	Display *display.Display
	mu      sync.Mutex
}

func (b *Buttons) Approve(ctx context.Context, request schemas.McpCallRequest) (bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.Confirm == nil || b.Deny == nil || b.Display == nil {
		return false, errors.New("approval: buttons and display are required")
	}
	drain(b.Confirm.Events)
	drain(b.Deny.Events)
	if err := b.Display.Clear(); err != nil {
		return false, err
	}
	b.Display.MoveCursor(0, 0)
	if err := b.Display.Print(fmt.Sprintf("Tool request:\n%s\nR=yes L=no", request.Call.Tool)); err != nil {
		return false, err
	}

	for {
		select {
		case event, ok := <-b.Confirm.Events:
			if !ok {
				return false, errors.New("approval: confirm button closed")
			}
			if event == button.Pressed {
				return true, nil
			}
		case event, ok := <-b.Deny.Events:
			if !ok {
				return false, errors.New("approval: deny button closed")
			}
			if event == button.Pressed {
				return false, nil
			}
		case <-ctx.Done():
			return false, ctx.Err()
		}
	}
}

func drain(events <-chan button.Event) {
	for {
		select {
		case <-events:
		default:
			return
		}
	}
}
