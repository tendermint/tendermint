package ssh

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
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

func MultiExec(cfg *ssh.ClientConfig, addr string, cmds ...string) error {
	c, err := ssh.Dial("tcp", addr, cfg)
	s, err := c.NewSession()
	if err != nil {
		return err
	}
	defer s.Close()
	for _, cmd := range cmds {
		err := s.Run(cmd)
		if err != nil {
			return err
		}
	}
	return nil
}

func NewClientConfig() (*ssh.ClientConfig, error) {
	ss := os.Getenv("SSH_AUTH_SOCK")
	if ss == "" {
		return nil, errors.New("SSH_AUTH_SOCK environment variable is empty. Is the ssh-agent running?")
	}
	c, err := net.Dial("unix", ss)
	if err != nil {
		return nil, err
	}
	ac := agent.NewClient(c)
	hkc, err := knownhosts.New(filepath.Join(os.Getenv("HOME"), ".ssh", "known_hosts"))
	if err != nil {
		return nil, err
	}
	signers, err := ac.Signers()
	if err != nil {
		return nil, err
	}
	am := make([]ssh.AuthMethod, 0, len(signers))
	for _, signer := range signers {
		am = append(am, ssh.PublicKeys(signer))
	}
	return &ssh.ClientConfig{
		User:              "root",
		HostKeyCallback:   hkcWrapper(hkc),
		Auth:              am,
		HostKeyAlgorithms: []string{ssh.KeyAlgoED25519},
	}, nil
}

func hkcWrapper(hkc ssh.HostKeyCallback) ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := hkc(hostname, remote, key)
		if err == nil {
			return nil
		}
		ke := &knownhosts.KeyError{}
		if errors.As(err, &ke) && len(ke.Want) == 0 {
			h, _, err := net.SplitHostPort(hostname)
			if err != nil {
				panic(fmt.Errorf("hostname incorrectly formatted: %w", err))
			}
			log.Printf("ignoring knownhosts error for unknown host: %s", h)
			return nil
		}

		return err
	}
}
