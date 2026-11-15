//go:build !linux

// Package uipkg provides the physical Airlock controls. Development hosts do
// not have the Linux GPIO interface, so use a no-op implementation there.
package uipkg

type UI struct{}

func Initialize() (*UI, error) { return &UI{}, nil }

func (ui *UI) Close() error { return nil }
