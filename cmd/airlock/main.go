package main

import (
	"github.com/ghofuobudgeghikhdrhiehdihebd/airlock/internal/ui"
)

func main() {
	ui, err := uipkg.Initialize()
	if err != nil {
		panic(err)
	}

}
