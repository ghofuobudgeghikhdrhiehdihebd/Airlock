package uipkg

import (
	"errors"
	"image"
	"image/color"
	"image/draw"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	"periph.io/x/conn/v3/i2c"
	"periph.io/x/conn/v3/i2c/i2creg"
	"periph.io/x/devices/v3/ssd1306"
)

const (
	displayWidth  = 128
	displayHeight = 64
)

// Display represents the physical OLED display.
type Display struct {
	bus    i2c.BusCloser
	device *ssd1306.Dev
	canvas *image.RGBA
}

// Initialize opens the I2C connection and initializes the OLED.
func (d *Display) Initialize() error {
	bus, err := i2creg.Open("/dev/i2c-1")
	if err != nil {
		return err
	}

	device, err := ssd1306.NewI2C(bus, &ssd1306.Opts{
		W: displayWidth,
		H: displayHeight,
	})
	if err != nil {
		_ = bus.Close()
		return err
	}

	d.bus = bus
	d.device = device
	d.canvas = image.NewRGBA(image.Rect(0, 0, displayWidth, displayHeight))

	return d.Clear()
}

// Clear removes everything currently displayed.
func (d *Display) Clear() error {
	if d.device == nil {
		return errors.New("display not initialized")
	}

	draw.Draw(
		d.canvas,
		d.canvas.Bounds(),
		image.NewUniform(color.Black),
		image.Point{},
		draw.Src,
	)

	return d.device.Draw(d.canvas.Bounds(), d.canvas, image.Point{})
}

// DrawText renders text at the specified coordinates.
// The y coordinate represents the text baseline.
func (d *Display) DrawText(x, y int, text string) error {
	if d.device == nil {
		return errors.New("display not initialized")
	}

	drawer := font.Drawer{
		Dst:  d.canvas,
		Src:  image.NewUniform(color.White),
		Face: basicfont.Face7x13,
		Dot:  fixed.P(x, y),
	}

	drawer.DrawString(text)

	return d.device.Draw(d.canvas.Bounds(), d.canvas, image.Point{})
}

// Close shuts down the display and releases the I2C connection.
func (d *Display) Close() error {
	var err error

	if d.device != nil {
		err = d.device.Halt()
		d.device = nil
	}

	if d.bus != nil {
		busErr := d.bus.Close()
		d.bus = nil

		if err == nil {
			err = busErr
		}
	}

	return err
}
