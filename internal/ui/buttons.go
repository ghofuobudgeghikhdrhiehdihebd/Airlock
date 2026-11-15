package ui

import (
	"fmt"
	"time"

	"github.com/warthog618/go-gpiocdev"
)

const (
	approveGPIO = 17
	denyGPIO    = 27

	pollInterval = 10 * time.Millisecond
	debounceTime = 40 * time.Millisecond
)

// Buttons manages the physical APPROVE and DENY buttons.
type Buttons struct {
	approve *gpiocdev.Line
	deny    *gpiocdev.Line
}

// NewButtons initializes both GPIO buttons.
func NewButtons() (*Buttons, error) {
	approve, err := gpiocdev.RequestLine(
		"gpiochip0",
		approveGPIO,
		gpiocdev.AsInput,
		gpiocdev.WithPullUp,
		gpiocdev.AsActiveLow,
		gpiocdev.WithConsumer("airlock-approve"),
	)
	if err != nil {
		return nil, fmt.Errorf("initialize approve button: %w", err)
	}

	deny, err := gpiocdev.RequestLine(
		"gpiochip0",
		denyGPIO,
		gpiocdev.AsInput,
		gpiocdev.WithPullUp,
		gpiocdev.AsActiveLow,
		gpiocdev.WithConsumer("airlock-deny"),
	)
	if err != nil {
		_ = approve.Close()
		return nil, fmt.Errorf("initialize deny button: %w", err)
	}

	return &Buttons{
		approve: approve,
		deny:    deny,
	}, nil
}

// WaitForDecision waits until a button is pressed and debounced.
func (b *Buttons) WaitForDecision() (Decision, error) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	var candidate Decision
	var stable Decision
	var candidateSince time.Time

	for range ticker.C {
		approveValue, err := b.approve.Value()
		if err != nil {
			return "", fmt.Errorf("read approve button: %w", err)
		}

		denyValue, err := b.deny.Value()
		if err != nil {
			return "", fmt.Errorf("read deny button: %w", err)
		}

		var current Decision

		switch {
		case approveValue == 1 && denyValue == 0:
			current = Approve

		case denyValue == 1 && approveValue == 0:
			current = Deny

		default:
			// Neither button, or both buttons, are pressed.
			current = ""
		}

		now := time.Now()

		// A change in button state restarts the debounce timer.
		if current != candidate {
			candidate = current
			candidateSince = now
			continue
		}

		// Ignore the state until it has remained stable.
		if now.Sub(candidateSince) < debounceTime {
			continue
		}

		// Only respond to a new stable press.
		if candidate != stable {
			stable = candidate

			if stable == Approve || stable == Deny {
				return stable, nil
			}
		}
	}

	return "", fmt.Errorf("button polling stopped unexpectedly")
}

// Close releases both GPIO lines.
func (b *Buttons) Close() error {
	var approveErr, denyErr error

	if b.approve != nil {
		approveErr = b.approve.Close()
		b.approve = nil
	}

	if b.deny != nil {
		denyErr = b.deny.Close()
		b.deny = nil
	}

	if approveErr != nil {
		return approveErr
	}

	return denyErr
}
