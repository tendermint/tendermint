package ssh

import (
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func Exec(cfg *ssh.ClientConfig, addr, cmd string) error {
	c, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return err
	}
	s, err := c.NewSession()
	defer s.Close()
	err = s.Run(cmd)
	if err != nil {
		return err
	}
	return nil
}

func NewClientConfig(key string) (*ssh.ClientConfig, error) {
	hkc, err := knownhosts.New(filepath.Join(os.Getenv("HOME"), ".ssh", "known_hosts"))
	if err != nil {
		return nil, err
	}
	return &ssh.ClientConfig{
		HostKeyCallback: hkc,
	}, nil
}
