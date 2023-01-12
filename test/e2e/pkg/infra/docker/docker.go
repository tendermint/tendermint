package docker

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"text/template"

	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
	"github.com/tendermint/tendermint/test/e2e/pkg/infra"
)

var _ infra.Provider = &Provider{}

// Provider implements a docker-compose backed infrastructure provider.
type Provider struct {
	Testnet *e2e.Testnet
}

// Setup generates the docker-compose file and write it to disk, erroring if
// any of these operations fail.
func (p *Provider) Setup() error {
	compose, err := dockerComposeBytes(p.Testnet)
	if err != nil {
		return err
	}
	//nolint: gosec
	// G306: Expect WriteFile permissions to be 0600 or less
	err = os.WriteFile(filepath.Join(p.Testnet.Dir, "docker-compose.yml"), compose, 0644)
	if err != nil {
		return err
	}
	return nil
}

// dockerComposeBytes generates a Docker Compose config file for a testnet and returns the
// file as bytes to be written out to disk.
func dockerComposeBytes(testnet *e2e.Testnet) ([]byte, error) {
	// Must use version 2 Docker Compose format, to support IPv6.
	tmpl, err := template.New("docker-compose").Funcs(template.FuncMap{
		"misbehaviorsToString": func(misbehaviors map[int64]string) string {
			str := ""
			for height, misbehavior := range misbehaviors {
				// after the first behavior set, a comma must be prepended
				if str != "" {
					str += ","
				}
				heightString := strconv.Itoa(int(height))
				str += misbehavior + "," + heightString
			}
			return str
		},
	}).Parse(`version: '2.4'

networks:
  {{ .Name }}:
    labels:
      e2e: true
    driver: bridge
{{- if .IPv6 }}
    enable_ipv6: true
{{- end }}
    ipam:
      driver: default
      config:
      - subnet: {{ .IP }}

services:
{{- range .Nodes }}
  {{ .Name }}:
    labels:
      e2e: true
    container_name: {{ .Name }}
    image: tendermint/e2e-node
{{- if eq .ABCIProtocol "builtin" }}
    entrypoint: /usr/bin/entrypoint-builtin
{{- else if .Misbehaviors }}
    entrypoint: /usr/bin/entrypoint-maverick
    command: ["node", "--misbehaviors", "{{ misbehaviorsToString .Misbehaviors }}"]
{{- end }}
    init: true
    ports:
    - 26656
    - {{ if .ProxyPort }}{{ .ProxyPort }}:{{ end }}26657
    - 6060
    volumes:
    - ./{{ .Name }}:/tendermint
    networks:
      {{ $.Name }}:
        ipv{{ if $.IPv6 }}6{{ else }}4{{ end}}_address: {{ .IP }}

{{end}}`)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, testnet)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
