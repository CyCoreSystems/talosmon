package main

import (
	"context"
	"log"
	"os"

	"github.com/CyCoreSystems/talosmon/config"
	"github.com/rivo/tview"
	"github.com/talos-systems/talos/pkg/client"
)

func main() {
	logOut, err := os.Create("debug.log")
	if err != nil {
		log.Panic("failed to open debug log file:", err)
	}

	log.SetOutput(logOut)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := tview.NewApplication()

	cfg, err := config.Load("/home/scmccord/.config/talosmon/config.yaml")
	if err != nil {
		log.Panic("failed to open config file:", err)
	}

	c, err := client.New(ctx)
	if err != nil {
		log.Panic("failed to create Talos client:", err)
	}

	grid, err := machineStatusGrid(ctx, app, c, cfg.Clusters[0].Machines)
	if err != nil {
		log.Panic("failed to construct machine grid:", err)
	}

	if err := app.SetRoot(grid, true).SetFocus(grid).Run(); err != nil {
		panic(err)
	}
}
