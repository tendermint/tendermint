package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/tendermint/tendermint/libs/log"
	tmnet "github.com/tendermint/tendermint/libs/net"
	"github.com/tendermint/tendermint/privval"
	grpcprivval "github.com/tendermint/tendermint/privval/grpc"
	privvalproto "github.com/tendermint/tendermint/proto/tendermint/privval"
)

var (
	// Create a metrics registry.
	reg = prometheus.NewRegistry()

	// Create some standard server metrics.
	grpcMetrics = grpc_prometheus.NewServerMetrics()
)

func main() {
	var (
		addr             = flag.String("addr", "127.0.0.1:26659", "Address to listen on (host:port)")
		chainID          = flag.String("chain-id", "mychain", "chain id")
		privValKeyPath   = flag.String("priv-key", "", "priv val key file path")
		privValStatePath = flag.String("priv-state", "", "priv val state file path")
		insecure         = flag.Bool("insecure", false, "allow server to run insecurely (no TLS)")
		certFile         = flag.String("certfile", "", "absolute path to server certificate")
		keyFile          = flag.String("keyfile", "", "absolute path to server key")
		rootCA           = flag.String("rootcafile", "", "absolute path to root CA")
		prometheusAddr   = flag.String("prometheus-addr", "", "address for prometheus endpoint (host:port)")
	)
	flag.Parse()

	logger, err := log.NewDefaultLogger(log.LogFormatPlain, log.LogLevelInfo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to construct logger: %v", err)
		os.Exit(1)
	}
	logger = logger.With("module", "priv_val")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger.Info(
		"Starting private validator",
		"addr", *addr,
		"chainID", *chainID,
		"privKeyPath", *privValKeyPath,
		"privStatePath", *privValStatePath,
		"insecure", *insecure,
		"certFile", *certFile,
		"keyFile", *keyFile,
		"rootCA", *rootCA,
	)

	pv, err := privval.LoadFilePV(*privValKeyPath, *privValStatePath)
	if err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}

	opts := []grpc.ServerOption{}
	if !*insecure {
		certificate, err := tls.LoadX509KeyPair(*certFile, *keyFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to load X509 key pair: %v", err)
			os.Exit(1)
		}

		certPool := x509.NewCertPool()
		bs, err := os.ReadFile(*rootCA)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to read client ca cert: %s", err)
			os.Exit(1)
		}

		if ok := certPool.AppendCertsFromPEM(bs); !ok {
			fmt.Fprintf(os.Stderr, "failed to append client certs")
			os.Exit(1)
		}

		tlsConfig := &tls.Config{
			ClientAuth:   tls.RequireAndVerifyClientCert,
			Certificates: []tls.Certificate{certificate},
			ClientCAs:    certPool,
			MinVersion:   tls.VersionTLS13,
		}

		creds := grpc.Creds(credentials.NewTLS(tlsConfig))
		opts = append(opts, creds)
		logger.Info("SignerServer: Creating security credentials")
	} else {
		logger.Info("SignerServer: You are using an insecure gRPC connection!")
	}

	// add prometheus metrics for unary RPC calls
	opts = append(opts, grpc.UnaryInterceptor(grpc_prometheus.UnaryServerInterceptor))

	ss := grpcprivval.NewSignerServer(logger, *chainID, pv)

	protocol, address := tmnet.ProtocolAndAddress(*addr)

	lis, err := net.Listen(protocol, address)
	if err != nil {
		fmt.Fprintf(os.Stderr, "SignerServer: Failed to listen %v", err)
		os.Exit(1)
	}

	s := grpc.NewServer(opts...)

	privvalproto.RegisterPrivValidatorAPIServer(s, ss)

	var httpSrv *http.Server
	if *prometheusAddr != "" {
		httpSrv = registerPrometheus(*prometheusAddr, s)
	}

	logger.Info("SignerServer: Starting grpc server")
	if err := s.Serve(lis); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to listen on port %s: %v", *addr, err)
		os.Exit(1)
	}

	opctx, opcancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer opcancel()
	go func() {
		<-opctx.Done()
		if *prometheusAddr != "" {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			if err := httpSrv.Shutdown(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "Unable to stop http server: %v", err)
				os.Exit(1)
			}
		}
		s.GracefulStop()
	}()

	// Run forever.
	select {}
}

func registerPrometheus(addr string, s *grpc.Server) *http.Server {
	// Initialize all metrics.
	grpcMetrics.InitializeMetrics(s)
	// create http server to serve prometheus
	httpServer := &http.Server{Handler: promhttp.HandlerFor(reg, promhttp.HandlerOpts{}), Addr: addr}

	go func() {
		if err := httpServer.ListenAndServe(); err != nil {
			fmt.Fprintf(os.Stderr, "Unable to start a http server: %v", err)
			os.Exit(1)
		}
	}()

	return httpServer
}
