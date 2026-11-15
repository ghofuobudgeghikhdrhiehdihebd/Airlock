// Package display provides text operations for an SSD1306 over I²C.
package display

import (
	"image"
	"image/draw"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
	"periph.io/x/conn/v3/i2c"
	"periph.io/x/conn/v3/i2c/i2creg"
	"periph.io/x/devices/v3/ssd1306"
	"periph.io/x/devices/v3/ssd1306/image1bit"
)

type Display struct {
	Bus    i2c.BusCloser
	Device *ssd1306.Dev
	Image  *image.Gray
	Face   font.Face
	Cursor image.Point
}

// New opens the named I²C registry bus and initializes an SSD1306 device.
func New(busName string, opts *ssd1306.Opts) (*Display, error) {
	bus, err := i2creg.Open(busName)
	if err != nil {
		return nil, err
	}
	dev, err := ssd1306.NewI2C(bus, opts)
	if err != nil {
		_ = bus.Close()
		return nil, err
	}
	return &Display{Bus: bus, Device: dev, Image: image.NewGray(dev.Bounds()), Face: basicfont.Face7x13}, nil
}

func (d *Display) Print(text string) error {
	drawer := &font.Drawer{Dst: d.Image, Src: image.White, Face: d.Face,
		Dot: fixed.P(d.Cursor.X, d.Cursor.Y+13)}
	drawer.DrawString(text)
	d.Cursor.X += font.MeasureString(d.Face, text).Ceil()
	return d.flush()
}

func (d *Display) Clear() error {
	d.Image = image.NewGray(d.Device.Bounds())
	d.Cursor = image.Point{}
	return d.flush()
}

// MoveCursor positions the text cursor using zero-based character cells.
func (d *Display) MoveCursor(row, column int) { d.Cursor = image.Pt(column*8, row*13) }

func (d *Display) flush() error {
	bmp := image1bit.NewVerticalLSB(d.Device.Bounds())
	draw.Draw(bmp, bmp.Bounds(), d.Image, image.Point{}, draw.Src)
	_, err := d.Device.Write(bmp.Pix)
	return err
}

func (d *Display) Close() error {
	if err := d.Device.Halt(); err != nil {
		_ = d.Bus.Close()
		return err
	}
	return d.Bus.Close()
}
