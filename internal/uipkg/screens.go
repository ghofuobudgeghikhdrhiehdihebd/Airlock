package uipkg

import (
	"strings"
)

func (d *Display) ShowIdleScreen() error {
	if err := d.Clear(); err != nil {
		return err
	}

	if err := d.DrawText(8, 18, "AIRLOCK READY"); err != nil {
		return err
	}

	return d.DrawText(8, 40, "Waiting for request...")
}

func (d *Display) ShowRequestScreen(req Request) error {
	if err := d.Clear(); err != nil {
		return err
	}

	if err := d.DrawText(8, 12, "REQUEST PENDING"); err != nil {
		return err
	}

	if err := d.DrawText(8, 27, "Action:"); err != nil {
		return err
	}

	if err := d.DrawText(8, 40, truncate(req.Action, 16)); err != nil {
		return err
	}

	return d.DrawText(8, 55, "Approve or Deny?")
}

func (d *Display) ShowDecisionScreen(decision Decision) error {
	if err := d.Clear(); err != nil {
		return err
	}

	switch decision {
	case Approve:
		if err := d.DrawText(8, 22, "REQUEST"); err != nil {
			return err
		}
		return d.DrawText(8, 42, "APPROVED")

	case Deny:
		if err := d.DrawText(8, 22, "REQUEST"); err != nil {
			return err
		}
		return d.DrawText(8, 42, "DENIED")

	default:
		return d.DrawText(8, 32, "UNKNOWN DECISION")
	}
}

// truncate keeps text within the available display width.
func truncate(s string, maxChars int) string {
	s = strings.TrimSpace(s)

	if len(s) <= maxChars {
		return s
	}

	if maxChars <= 3 {
		return s[:maxChars]
	}

	return s[:maxChars-3] + "..."
}
