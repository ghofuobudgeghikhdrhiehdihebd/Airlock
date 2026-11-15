package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"

	"periph.io/x/devices/v3/ssd1306"
	"periph.io/x/host/v3"

	"github.com/ghofuobudgeghikhdrhiehdihebd/airlock/pkg/ai"
	"github.com/ghofuobudgeghikhdrhiehdihebd/airlock/pkg/approval"
	"github.com/ghofuobudgeghikhdrhiehdihebd/airlock/pkg/button"
	"github.com/ghofuobudgeghikhdrhiehdihebd/airlock/pkg/display"
	"github.com/ghofuobudgeghikhdrhiehdihebd/airlock/pkg/schemas"
	"github.com/ghofuobudgeghikhdrhiehdihebd/airlock/pkg/serial"
)

// Airlock owns the host serial, AI I/O, AI tool, and user-approval interfaces.
type Airlock struct {
	Serial   *serial.Server
	IO       ai.IOInterface
	Tool     ai.ToolInterface
	Approval approval.Interface

	sessionMu sync.Mutex
	session   *serial.Session
}

func (a *Airlock) Run(ctx context.Context) error {
	if a.Serial == nil || a.IO == nil || a.Tool == nil || a.Approval == nil {
		return errors.New("airlock: serial, AI I/O, AI tool, and approval interfaces are required")
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	toolErr := make(chan error, 1)
	go func() { toolErr <- a.Tool.Listen(runCtx) }()

	serialErr := a.Serial.Listen(runCtx)
	cancel()
	if serialErr != nil && !errors.Is(serialErr, context.Canceled) {
		return serialErr
	}
	if err := <-toolErr; err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return ctx.Err()
}

func (a *Airlock) handleSerial(ctx context.Context, session *serial.Session) error {
	request, err := session.Receive()
	if err != nil {
		return session.Send(errorMessage(fmt.Errorf("airlock: decode serial request: %w", err)))
	}
	if !a.claimSession(session) {
		return session.Send(errorMessage(errors.New("airlock: another PC-driver request is active")))
	}
	defer a.releaseSession(session)

	err = a.IO.Stream(ctx, request, func(output string) error {
		return session.Send(schemas.Message{Type: "output", String: output})
	})
	if err != nil {
		return session.Send(errorMessage(fmt.Errorf("airlock: exchange with AI device: %w", err)))
	}
	return session.Send(schemas.Message{Type: "done"})
}

func (a *Airlock) handleToolRequest(ctx context.Context, request schemas.Message) (schemas.Message, error) {
	approved, err := a.Approval.Approve(ctx, request.Request)
	if err != nil {
		return toolFailure(request, err.Error()), nil
	}
	if !approved {
		return toolFailure(request, "tool request denied by user"), nil
	}

	session := a.activeSession()
	if session == nil {
		return toolFailure(request, "host PC driver is not connected"), nil
	}
	if err := session.Send(schemas.Message{Type: "tool-request", Request: request.Request}); err != nil {
		return toolFailure(request, fmt.Sprintf("send tool request to host: %v", err)), nil
	}
	response, err := session.Receive()
	if err != nil {
		return toolFailure(request, fmt.Sprintf("read tool result from host: %v", err)), nil
	}
	if response.Type != "tool-result" {
		return toolFailure(request, "host returned an unexpected message type"), nil
	}
	return response, nil
}

func (a *Airlock) claimSession(session *serial.Session) bool {
	a.sessionMu.Lock()
	defer a.sessionMu.Unlock()
	if a.session != nil {
		return false
	}
	a.session = session
	return true
}

func (a *Airlock) releaseSession(session *serial.Session) {
	a.sessionMu.Lock()
	defer a.sessionMu.Unlock()
	if a.session == session {
		a.session = nil
	}
}

func (a *Airlock) activeSession() *serial.Session {
	a.sessionMu.Lock()
	defer a.sessionMu.Unlock()
	return a.session
}

func errorMessage(err error) schemas.Message {
	return schemas.Message{Type: "error", String: err.Error()}
}

func toolFailure(request schemas.Message, reason string) schemas.Message {
	return schemas.Message{Type: "tool-result", Response: schemas.McpCallResult{
		Server: request.Request.Server,
		Tool:   request.Request.Call.Tool,
		Error:  reason,
	}}
}

func main() {
	approver, closeHardware, err := newHardwareApprover()
	if err != nil {
		panic(err)
	}
	defer closeHardware()

	app := &Airlock{IO: ai.NewIOClient(), Approval: approver}
	app.Serial = serial.New()
	app.Serial.Handler = app.handleSerial
	app.Tool = ai.NewToolServer(app.handleToolRequest)
	if err := app.Run(context.Background()); err != nil && !errors.Is(err, context.Canceled) {
		panic(err)
	}
}

func newHardwareApprover() (*approval.Buttons, func(), error) {
	if _, err := host.Init(); err != nil {
		return nil, nil, fmt.Errorf("airlock: initialize hardware host: %w", err)
	}
	chip := os.Getenv("AIRLOCK_GPIO_CHIP")
	confirmOffset, err := requiredOffset("AIRLOCK_CONFIRM_GPIO_OFFSET")
	if err != nil {
		return nil, nil, err
	}
	denyOffset, err := requiredOffset("AIRLOCK_DENY_GPIO_OFFSET")
	if err != nil {
		return nil, nil, err
	}
	if chip == "" {
		return nil, nil, errors.New("airlock: AIRLOCK_GPIO_CHIP is required")
	}
	confirm, err := button.New(chip, confirmOffset)
	if err != nil {
		return nil, nil, fmt.Errorf("airlock: open confirm button: %w", err)
	}
	deny, err := button.New(chip, denyOffset)
	if err != nil {
		_ = confirm.Close()
		return nil, nil, fmt.Errorf("airlock: open deny button: %w", err)
	}
	opts := ssd1306.DefaultOpts
	display, err := display.New(os.Getenv("AIRLOCK_I2C_BUS"), &opts)
	if err != nil {
		_ = deny.Close()
		_ = confirm.Close()
		return nil, nil, fmt.Errorf("airlock: open display: %w", err)
	}
	closeHardware := func() {
		_ = display.Close()
		_ = deny.Close()
		_ = confirm.Close()
	}
	return &approval.Buttons{Confirm: confirm, Deny: deny, Display: display}, closeHardware, nil
}

func requiredOffset(name string) (int, error) {
	value := os.Getenv(name)
	if value == "" {
		return 0, fmt.Errorf("airlock: %s is required", name)
	}
	offset, err := strconv.Atoi(value)
	if err != nil || offset < 0 {
		return 0, fmt.Errorf("airlock: %s must be a non-negative integer", name)
	}
	return offset, nil
}
