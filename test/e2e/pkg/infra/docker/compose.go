package docker

import (
	"bytes"
	"text/template"

	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
)

// makeDockerCompose generates a Docker Compose config for a testnet.
func makeDockerCompose(testnet *e2e.Testnet) ([]byte, error) {
	// Must use version 2 Docker Compose format, to support IPv6.
	tmpl, err := template.New("docker-compose").Funcs(template.FuncMap{
		"addUint32": func(x, y uint32) uint32 {
			return x + y
		},
		"isBuiltin": func(protocol e2e.Protocol, mode e2e.Mode) bool {
			return mode == e2e.ModeLight || protocol == e2e.ProtocolBuiltin
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
{{- if isBuiltin $.ABCIProtocol .Mode }}
    entrypoint: /usr/bin/entrypoint-builtin
{{- else if .LogLevel }}
    command: start --log-level {{ .LogLevel }}
{{- end }}
    init: true
    ports:
    - 26656
    - {{ if .ProxyPort }}{{ addUint32 .ProxyPort 1000 }}:{{ end }}26660
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
