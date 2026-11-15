// Package button provides GPIO-backed, channel-native button inputs.
package button

import "github.com/warthog618/go-gpiocdev"

type Event uint8

const (
	Pressed Event = iota
	Released
)

type Button struct {
	Line   *gpiocdev.Line
	Events chan Event
	done   chan struct{}
}

// New requests an input line and starts the event-forwarding goroutine.
func New(chip string, offset int) (*Button, error) {
	raw := make(chan gpiocdev.LineEvent, 8)
	line, err := gpiocdev.RequestLine(chip, offset, gpiocdev.WithBothEdges,
		gpiocdev.WithEventHandler(func(event gpiocdev.LineEvent) { raw <- event }))
	if err != nil {
		return nil, err
	}
	b := &Button{Line: line, Events: make(chan Event, 8), done: make(chan struct{})}
	go func() {
		defer close(b.Events)
		for {
			select {
			case event := <-raw:
				if event.Type == gpiocdev.LineEventRisingEdge {
					b.Events <- Pressed
				} else {
					b.Events <- Released
				}
			case <-b.done:
				return
			}
		}
	}()
	return b, nil
}

func (b *Button) Close() error {
	close(b.done)
	return b.Line.Close()
}
