//go:build linux

package uipkg

import (
	"log"

	"github.com/warthog618/go-gpiocdev"
	"periph.io/x/conn/v3/i2c/i2creg"
	"periph.io/x/devices/v3/ssd1306"
)

type UI struct {
	ButtonAccept *gpiocdev.Line
	ButtonReject *gpiocdev.Line
	Display      *ssd1306.Dev
}

func Initialize() (ui *UI, err error) {
	ui = &UI{}
	ui.ButtonAccept, err = gpiocdev.RequestLine("gpiochip0", 11)
	if err != nil {
		return nil, err
	}
	ui.ButtonReject, err = gpiocdev.RequestLine("gpiochip0", 13)
	if err != nil {
		return nil, err
	}
	bus, err := i2creg.Open("/dev/i2c-1")
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = bus.Close()
	}()
	ui.Display, err = ssd1306.NewI2C(bus, &ssd1306.Opts{
		W: 128,
		H: 64,
	})
	if err != nil {
		return nil, err
	}
	return ui, nil
}

func (ui *UI) Close() (err error) {
	err = ui.ButtonAccept.Close()
	if err != nil {
		return err
	}
	err = ui.ButtonReject.Close()
	if err != nil {
		return err
	}
	return nil
}
