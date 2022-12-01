package digitalocean

import (
	"context"
	"fmt"

	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
	"github.com/tendermint/tendermint/test/e2e/pkg/infra"
	e2essh "github.com/tendermint/tendermint/test/e2e/pkg/ssh"
	"golang.org/x/crypto/ssh"
)

const (
	sshPort     = 22
	testappName = "testappd"
)

var _ infra.Provider = &Provider{}

// Provider implements a docker-compose backed infrastructure provider.
type Provider struct {
	Testnet            *e2e.Testnet
	InfrastructureData e2e.InfrastructureData
	SSHConfig          *ssh.ClientConfig
}

// Noop currently. Setup is performed externally to the e2e test tool.
func (p *Provider) Setup() error {
	return nil
}

// Noop currently. Node creation is currently performed externally to the e2e test tool.
func (p Provider) CreateNode(ctx context.Context, n *e2e.Node) error {
	return nil
}
func (p Provider) StartTendermint(ctx context.Context, n *e2e.Node) error {
	return e2essh.Exec(p.SSHConfig, fmt.Sprintf("%s:%d", n.IP, sshPort), fmt.Sprintf("systemctl start %s", testappName))
}
func (p Provider) TerminateTendermint(ctx context.Context, n *e2e.Node) error {
	return e2essh.Exec(p.SSHConfig, fmt.Sprintf("%s:%d", n.IP, sshPort), fmt.Sprintf("systemctl -s SIGTERM %s", testappName))
}
func (p Provider) KillTendermint(ctx context.Context, n *e2e.Node) error {
	return e2essh.Exec(p.SSHConfig, fmt.Sprintf("%s:%d", n.IP, sshPort), fmt.Sprintf("systemctl -s SIGKILL %s", testappName))
}
func (p Provider) Disconnect(ctx context.Context, n *e2e.Node) error {
	return e2essh.MultiExec(p.SSHConfig, fmt.Sprintf("%s:%d", n.IP, sshPort),
		"iptables -A INPUT -p tcp --destination-port 26656 -j DROP",
		"iptables -A OUTPUT -p tcp --destination-port 26656 -j DROP",
		"service iptables save",
	)
}
func (p Provider) Connect(ctx context.Context, n *e2e.Node) error {
	return e2essh.MultiExec(p.SSHConfig, fmt.Sprintf("%s:%d", n.IP, sshPort),
		"iptables -D INPUT -p tcp --destination-port 26656 -j DROP",
		"iptables -D OUTPUT -p tcp --destination-port 26656 -j DROP",
		"service iptables save",
	)
}
