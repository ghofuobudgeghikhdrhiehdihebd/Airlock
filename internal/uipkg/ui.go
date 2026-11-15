package uipkg

type Decision uint8

const (
	Approve Decision = 1
	Deny    Decision = 0
)

type Request struct {
	ID          string
	Action      string
	Description string
}

type UI interface {
	Initialize() error
	ShowIdle() error
	ShowRequest(req Request) error
	ShowDecision(decision Decision) error
	WaitForDecision() (Decision, error)
	Close() error
}
