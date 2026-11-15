package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ghofuobudgeghikhdrhiehdihebd/airlock/internal/btserial"
	"github.com/ghofuobudgeghikhdrhiehdihebd/airlock/internal/uipkg"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ui, err := uipkg.Initialize()
	if err != nil {
		panic(err)
	}
	defer ui.Close()
	if err := btserial.Run(ctx, btserial.Config{Device: os.Getenv("AIRLOCK_BT_DEVICE")}); err != nil {
		log.Printf("bluetooth console stopped: %v", err)
	}
}
