package docker

import (
	"bytes"
	"os"
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
		"debugPort": func(index int) int {
			return 40000 + index + 1
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
{{- range $index, $node :=  .Nodes }}
  {{ .Name }}:
    labels:
      e2e: true
    container_name: {{ .Name }}
    image: tenderdash/e2e-node
{{- if isBuiltin $.ABCIProtocol .Mode }}
    entrypoint: /usr/bin/entrypoint-builtin
{{- else if .LogLevel }}
    command: start --log-level {{ .LogLevel }}
{{- end }}
    init: true
{{- if $.Debug }}
    environment:
    - DEBUG=1
    - DEBUG_PORT={{ debugPort $index }}
{{- end }}
    ports:
    - 26656
    - {{ if .ProxyPort }}{{ addUint32 .ProxyPort 1000 }}:{{ end }}26660
    - {{ if .ProxyPort }}{{ .ProxyPort }}:{{ end }}26657
    - 6060
{{- if $.Debug }}
    - {{ debugPort $index }}:{{ debugPort $index }}
    security_opt:
      - "seccomp:unconfined"
    cap_add:
      - SYS_PTRACE
{{- end }}
    volumes:
    - ./{{ .Name }}:/tenderdash
{{- if ne $.PreCompiledAppPath "" }}
    - {{ $.PreCompiledAppPath }}:/usr/bin/app
{{- end }}
    networks:
      {{ $.Name }}:
        ipv{{ if $.IPv6 }}6{{ else }}4{{ end}}_address: {{ .IP }}

{{end}}`)
	if err != nil {
		return nil, err
	}
	data := &struct {
		*e2e.Testnet
		PreCompiledAppPath string
		Debug              bool
	}{
		Testnet:            testnet,
		PreCompiledAppPath: os.Getenv("PRE_COMPILED_APP_PATH"),
		Debug:              os.Getenv("DEBUG") != "",
	}
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
