package machine

import (
	"context"
	"log"
	"net"
	"sync"
	"time"

	"github.com/rotisserie/eris"
	"github.com/sparrc/go-ping"
	"github.com/talos-systems/talos/api/machine"
	"github.com/talos-systems/talos/pkg/client"
)

const StateUnknown = "unknown"

const pingThreshold = 2 * time.Second

const serviceCheckInterval = 2 * time.Second

// Manager provides a common management structure for keeping track of a
// machine, its configuration, and its status.
type Manager struct {
	Spec   *Spec
	Status *Status

	client *client.Client

	pingers        map[string]*ping.Pinger
	pingTimestamps map[string]time.Time

	mu sync.RWMutex
}

// NewManager creates and runs a new manager for the given machine
func NewManager(ctx context.Context, c *client.Client, spec *Spec) (m *Manager, err error) {
	if spec == nil {
		return nil, eris.New("spec is required")
	}
	if c == nil {
		return nil, eris.New("talos client is required")
	}

	m = &Manager{
		Spec:           spec,
		Status:         new(Status),
		client:         c,
		pingers:        make(map[string]*ping.Pinger),
		pingTimestamps: make(map[string]time.Time),
	}

	if err = m.addPinger("ipmi", spec.IPMIAddr.String()); err != nil {
		m.Stop()
		return nil, eris.Wrapf(err, "failed to add pinger for %q", spec.Name)
	}

	if err = m.addPinger("ipv4", spec.IPv4Addr.String()); err != nil {
		m.Stop()
		return nil, eris.Wrapf(err, "failed to add pinger for %q", spec.Name)
	}

	if err = m.addPinger("ipv6", spec.IPv6Addr.String()); err != nil {
		m.Stop()
		return nil, eris.Wrapf(err, "failed to add pinger for %q", spec.Name)
	}

	go m.watchServices(ctx)

	return m, nil
}

func (m *Manager) addPinger(kind string, addr string) error {
	p, err := ping.NewPinger(addr)
	if err != nil {
		return eris.Wrapf(err, "failed to create %q pinger", kind)
	}

	p.OnRecv = func(p *ping.Packet) {
		m.mu.Lock()
		m.pingTimestamps[kind] = time.Now()
		m.mu.Unlock()
	}

	m.mu.Lock()
	m.pingers[kind] = p
	m.mu.Unlock()

	go p.Run()

	return nil
}

func (m *Manager) updateServiceStatus(ctx context.Context) {
	ctx = client.WithNodes(ctx, m.Spec.Name)

	m.Status.mu.Lock()
	defer m.Status.mu.Unlock()

	// Reset all states first
	for _, s := range m.Status.Services {
		s.State = StateUnknown
	}

	resp, err := m.client.ServiceList(ctx)
	if err == nil {
		for _, msg := range resp.Messages {
			m.Status.Services = msg.Services
		}
	}
}

func (m *Manager) updateVersion(ctx context.Context) {
	ctx = client.WithNodes(ctx, m.Spec.Name)

	m.Status.mu.Lock()
	defer m.Status.mu.Unlock()

	resp, err := m.client.Version(ctx)
	if err == nil {
		for _, msg := range resp.GetMessages() {
			m.Status.TalosVersion = msg.GetVersion().GetTag()
		}
	}
}

func (m *Manager) watchServices(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(serviceCheckInterval):
			m.updateServiceStatus(ctx)
			m.updateVersion(ctx)
		}
	}
}

// Stop shuts down the Manager
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, v := range m.pingers {
		v.Stop()
	}
	m.pingers = nil

	if m.client != nil {
		m.client.Close() // nolint
	}
}

// PingStatus indicates whether the given ping kind (ipmi,ipv4,ipv6) is up
func (m *Manager) PingUp(kind string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ts, ok := m.pingTimestamps[kind]
	if !ok {
		log.Printf("failed to find ping timestamp for %q", kind)
		return false
	}

	return time.Since(ts) < pingThreshold
}

// ServiceState returns the status of the given service
func (m *Manager) ServiceState(name string) string {
	m.Status.mu.RLock()
	defer m.Status.mu.RUnlock()

	for _, s := range m.Status.Services {
		if s.Id == name {
			return s.State
		}
	}
	return StateUnknown
}

// Version returns the Talos version of the machine
func (m *Manager) Version() string {
	m.Status.mu.RLock()
	defer m.Status.mu.RUnlock()

	return m.Status.TalosVersion
}

// Spec is the specification of a machine
type Spec struct {
	// Name is the simple hostname of the machine
	Name string `json:"name" yaml:"name"`

	// FQDN is the fully-qualified domain name of the machine
	FQDN string `json:"fqdn" yaml:"fqdn"`

	// IPMIAddr is the host/IP of the IPMI interface for the machine
	IPMIAddr net.IP `json:"ipmiAddr" yaml:"ipmiAddr"`

	// IPv4Addr is the main IPv4 address of the machine
	IPv4Addr net.IP `json:"ipv4Addr" yaml:"ipv4Addr"`

	// IPv6Addr is the main IPv6 address of the machine
	IPv6Addr net.IP `json:"ipv6Addr" yaml:"ipv6Addr"`
}

// Status is the status of a machine
type Status struct {

	// IPMI indicates whether the IPMI interface is responsive
	IPMI bool

	// HostV4 indicates whether the IPv4 address of the host is responsive
	HostV4 bool

	// HostV6 indicates whether the IPv6 address of the host is responsive
	HostV6 bool

	// TalosVersion indicates the Talos version which is currently installed (if any)
	TalosVersion string

	// Services stores the most recent service states
	Services []*machine.ServiceInfo

	mu sync.RWMutex
}
