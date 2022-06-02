package cmd

import (
	"bytes"
	"fmt"
	"io"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/spf13/cobra"

	"github.com/tendermint/tendermint/cmd/abcidump/parser"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
)

const (
	flagPort      = "port"
	flagInterface = "interface"
	flagPromisc   = "promisc"

	flagRequestType  = "request-type"
	flagResponseType = "response-type"
)

// CaptureCmd captures network traffic and dumps it as JSON message
type CaptureCmd struct {
	cmd       *cobra.Command
	Input     io.Reader
	Promisc   bool
	Port      uint16
	Interface string

	RequestType  string
	ResponseType string
}

// Command returns cobra command
func (captureCmd *CaptureCmd) Command() *cobra.Command {
	if captureCmd.cmd != nil {
		return captureCmd.cmd
	}

	captureCmd.cmd = &cobra.Command{
		Use:   "capture",
		Short: "capture TCP traffic, parse as ABCI protocol and display it in JSON format",

		RunE: captureCmd.RunE,
		PostRun: func(cmd *cobra.Command, args []string) {
			if captureCmd.Input != nil {
				if closer, ok := captureCmd.Input.(io.Closer); ok {
					closer.Close()
				}
			}
		},
	}

	captureCmd.cmd.Flags().StringVarP(&captureCmd.Interface, flagInterface, "i", "eth0", "interface to use")
	captureCmd.cmd.Flags().Uint16VarP(&captureCmd.Port, flagPort, "p", 26658, "Port number to use")
	captureCmd.cmd.Flags().StringVar(&captureCmd.RequestType, flagRequestType,
		"tendermint.abci.Request", "Protobuf type of the request")
	captureCmd.cmd.Flags().StringVar(&captureCmd.ResponseType, flagResponseType,
		"tendermint.abci.Response", "Protobuf type of the response")
	captureCmd.cmd.Flags().BoolVar(&captureCmd.Promisc, flagPromisc, false, "Enable promiscuous mode")

	return captureCmd.cmd
}

// RunE executes traffic capture
func (captureCmd *CaptureCmd) RunE(cmd *cobra.Command, args []string) error {
	logger.Debug("Starting packet capture", "port", captureCmd.Port)

	return captureCmd.capture()
}

func (captureCmd *CaptureCmd) capture() error {
	handle, err := pcap.OpenLive(captureCmd.Interface, 65535, captureCmd.Promisc, time.Second)
	if err != nil {
		return err
	}
	defer handle.Close()

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	for packet := range packetSource.Packets() {
		if err := captureCmd.processPacket(packet); err != nil {
			logger.Error("cannot parse packet", "error", err)
		}
	}

	return nil
}

func (captureCmd *CaptureCmd) processPacket(packet gopacket.Packet) error {
	payload, isRequest := captureCmd.parsePacket(packet)
	if len(payload) > 0 {
		in := bytes.NewBuffer(payload)
		p := parser.NewParser(in)
		p.Out = captureCmd.cmd.OutOrStdout()

		var msgType string
		if isRequest {
			msgType = captureCmd.RequestType
		} else {
			msgType = captureCmd.ResponseType
		}

		err := p.Parse(msgType)
		if err != nil {
			return fmt.Errorf("cannot parse message payload '%x' : %w", payload, err)
		}

		logger.Debug("packet processed", "isRequest", isRequest, "payload", payload)
	}

	return nil
}

func (captureCmd *CaptureCmd) parsePacket(packet gopacket.Packet) (payload tmbytes.HexBytes, isRequest bool) {
	tcpLayer := packet.Layer(layers.LayerTypeTCP)
	if tcpLayer == nil {
		return nil, false
	}

	payload = tcpLayer.LayerPayload()
	tcp, ok := tcpLayer.(*layers.TCP)
	if !ok {
		logger.Debug("received non-tcp packet", "packet", packet)
		return nil, false
	}

	if tcp.DstPort == layers.TCPPort(captureCmd.Port) {
		return payload, true
	}

	if tcp.SrcPort == layers.TCPPort(captureCmd.Port) {
		return payload, false
	}

	return nil, false
}
