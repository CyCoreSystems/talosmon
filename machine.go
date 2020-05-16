package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/CyCoreSystems/talosmon/machine"
	"github.com/rivo/tview"
	"github.com/rotisserie/eris"
	"github.com/talos-systems/talos/pkg/client"
)

var shortcuts = []rune{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z'}

type machineGrid struct {
	ui *tview.Grid

	machines []*machine.Manager

	selectedMachine *machine.Manager

	mu sync.RWMutex
}

func (g *machineGrid) selectMachine(index int) {
	g.mu.Lock()
	g.selectedMachine = g.machines[index]
	g.mu.Unlock()
}

func (g *machineGrid) machineDetail(t *tview.Table) {
	g.mu.RLock()
	m := g.selectedMachine
	g.mu.RUnlock()

	t.Clear()

	var j int

	t.SetCellSimple(j, 0, "[lightgreen]Name[-]")
	t.SetCellSimple(j, 1, m.Spec.Name)

	j++
	t.SetCellSimple(j, 0, "[lightgreen]FQDN[-]")
	t.SetCellSimple(j, 1, m.Spec.FQDN)

	j++
	t.SetCellSimple(j, 0, "[lightgreen]IPMI[-]")
	t.SetCellSimple(j, 1, m.Spec.IPMIAddr.String())

	j++
	t.SetCellSimple(j, 0, "[lightgreen]IPv4[-]")
	t.SetCellSimple(j, 1, m.Spec.IPv4Addr.String())

	j++
	t.SetCellSimple(j, 0, "[lightgreen]IPv6[-]")
	t.SetCellSimple(j, 1, m.Spec.IPv6Addr.String())
}

func machineStatusGrid(ctx context.Context, app *tview.Application, c *client.Client, machineSpecs []*machine.Spec) (tview.Primitive, error) {
	newLabel := func(text string) tview.Primitive {
		return tview.NewTextView().SetTextAlign(tview.AlignCenter).SetText(text)
	}

	list := tview.NewList().ShowSecondaryText(false)
	detail := tview.NewTable()
	status := tview.NewTable()

	ui := tview.NewGrid().
		SetRows(1, 15, 0).
		SetColumns(30, 0).
		SetBorders(true).
		AddItem(newLabel("Machines"), 0, 0, 1, 1, 0, 0, false).
		AddItem(newLabel("Detail"), 0, 1, 1, 1, 0, 0, false).
		AddItem(newLabel("Status"), 0, 2, 1, 1, 0, 0, false).
		AddItem(newLabel("Logs"), 2, 0, 1, 3, 0, 0, false).
		AddItem(list, 1, 0, 1, 1, 0, 0, true).
		AddItem(detail, 1, 1, 1, 1, 0, 0, false).
		AddItem(status, 1, 2, 1, 1, 0, 0, false)

	var machines []*machine.Manager
	for _, spec := range machineSpecs {
		m, err := machine.NewManager(ctx, c, spec)
		if err != nil {
			return nil, eris.Wrapf(err, "failed to create machine manager for %s", spec.Name)
		}

		machines = append(machines, m)
	}

	g := &machineGrid{
		ui:              ui,
		machines:        machines,
		selectedMachine: machines[0],
	}

	// Construct the machine status pane and updater
	machineStatus := newMachineStatus(ctx, app, status, g.selectedMachine)

	// Fill initial machine data
	for i, m := range machines {
		log.Printf("Adding %s(%s)", m.Spec.Name, m.Spec.FQDN)
		list.AddItem(m.Spec.Name, m.Spec.FQDN, shortcuts[i], func(i int, m *machine.Manager) func() {
			return func() {
				log.Println("selecting machine:", m.Spec.Name)
				g.selectMachine(i)
				g.machineDetail(detail)
				machineStatus.setMachine(m)
			}
		}(i, m))
	}

	return ui, nil
}

type machineStatus struct {
	app   *tview.Application
	table *tview.Table

	machine *machine.Manager

	mu sync.RWMutex
}

func newMachineStatus(ctx context.Context, app *tview.Application, t *tview.Table, m *machine.Manager) *machineStatus {
	s := new(machineStatus)

	s.app = app
	s.table = t
	s.machine = m

	go s.run(ctx)

	return s
}

func (s *machineStatus) setMachine(m *machine.Manager) {
	s.mu.Lock()
	s.machine = m
	s.mu.Unlock()
}

func (s *machineStatus) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
			s.render()
		}
	}
}

func statusIcon(up bool) string {
	color := "red"
	if up {
		color = "lightgreen"
	}
	return fmt.Sprintf("[%s]⬤ [-]", color)
}

func serviceStatusIcon(status string) string {
	var color string

	switch strings.ToLower(status) {
	case "running":
		color = "lightgreen"
	case "finished":
		color = "darkgreen"
	case "waiting":
		color = "yellow"
	case "stopped":
		color = "orange"
	case "skipped":
		color = "grey"
	case "failed":
		color = "red"
	case "unknown":
		color = "darkgrey"
	default:
		log.Println("unhandled state:", status)
		color = "darkgrey"
	}

	return fmt.Sprintf("[%s]⬤ [-]", color)
}

func (s *machineStatus) render() {
	s.app.QueueUpdateDraw(func() {
		s.mu.RLock()
		defer s.mu.RUnlock()

		var j int

		s.table.SetCellSimple(j, 0, statusIcon(s.machine.PingUp("ipmi")))
		s.table.SetCellSimple(j, 1, fmt.Sprintf("IPMI (%s)", s.machine.Spec.IPMIAddr))

		j++
		s.table.SetCellSimple(j, 0, statusIcon(s.machine.PingUp("ipv4")))
		s.table.SetCellSimple(j, 1, fmt.Sprintf("IPv4 (%s)", s.machine.Spec.IPv4Addr))

		j++
		s.table.SetCellSimple(j, 0, statusIcon(s.machine.PingUp("ipv6")))
		s.table.SetCellSimple(j, 1, fmt.Sprintf("IPv6 (%s)", s.machine.Spec.IPv6Addr))

		j++
		s.table.SetCellSimple(j, 0, "")
		s.table.SetCellSimple(j, 1, s.machine.Version())

		j++
		s.table.SetCellSimple(j, 0, serviceStatusIcon(s.machine.ServiceState("bootkube")))
		s.table.SetCellSimple(j, 1, "Service - Bootkube")

		j++
		s.table.SetCellSimple(j, 0, serviceStatusIcon(s.machine.ServiceState("kubelet")))
		s.table.SetCellSimple(j, 1, "Service - Kubelet")
	})
}
