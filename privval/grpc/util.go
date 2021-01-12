package grpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"os"
	"time"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/log"
	tmnet "github.com/tendermint/tendermint/libs/net"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

// DefaultDialOptions constructs a list of grpc dial options
func DefaultDialOptions(
	extraOpts ...grpc.DialOption,
) []grpc.DialOption {
	const (
		retries            = 50 // 50 * 100ms = 5s total
		timeout            = 1 * time.Second
		maxCallRecvMsgSize = 1 << 20 // Default 5Mb
	)

	var kacp = keepalive.ClientParameters{
		Time:    10 * time.Second, // send pings every 10 seconds if there is no activity
		Timeout: 2 * time.Second,  // wait 2 seconds for ping ack before considering the connection dead
	}

	opts := []grpc_retry.CallOption{
		grpc_retry.WithBackoff(grpc_retry.BackoffExponential(timeout)),
	}

	dialOpts := []grpc.DialOption{
		grpc.WithKeepaliveParams(kacp),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(maxCallRecvMsgSize),
			grpc_retry.WithMax(retries),
		),
		grpc.WithUnaryInterceptor(
			grpc_retry.UnaryClientInterceptor(opts...),
		),
	}

	dialOpts = append(dialOpts, extraOpts...)

	return dialOpts
}

func GenerateTLS(certPath, keyPath, ca string, log log.Logger) grpc.DialOption {
	certificate, err := tls.LoadX509KeyPair(
		certPath,
		keyPath,
	)
	if err != nil {
		log.Error("error", err)
		os.Exit(1)
	}

	certPool := x509.NewCertPool()
	bs, err := ioutil.ReadFile(ca)
	if err != nil {
		log.Error("failed to read ca cert:", "error", err)
		os.Exit(1)
	}

	ok := certPool.AppendCertsFromPEM(bs)
	if !ok {
		log.Error("failed to append certs")
		os.Exit(1)
	}

	transportCreds := credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{certificate},
		RootCAs:      certPool,
		MinVersion:   tls.VersionTLS13,
	})

	return grpc.WithTransportCredentials(transportCreds)
}

// DialRemoteSigner is  a generalized function to dial the gRPC server.
func DialRemoteSigner(
	config *cfg.Config,
	chainID string,
	logger log.Logger,
) (*SignerClient, error) {
	var transportSecurity grpc.DialOption
	if config.BaseConfig.ArePrivValidatorClientSecurityOptionsPresent() {
		transportSecurity = GenerateTLS(config.PrivValidatorClientCertificateFile(),
			config.PrivValidatorClientKeyFile(), config.PrivValidatorRootCAFile(), logger)
	} else {
		transportSecurity = grpc.WithInsecure()
		logger.Info("Using an insecure gRPC connection!")
	}

	dialOptions := DefaultDialOptions()
	if config.Instrumentation.Prometheus {
		grpcMetrics := grpc_prometheus.DefaultClientMetrics
		dialOptions = append(dialOptions, grpc.WithUnaryInterceptor(grpcMetrics.UnaryClientInterceptor()))
	}

	dialOptions = append(dialOptions, transportSecurity)

	ctx := context.Background()
	_, address := tmnet.ProtocolAndAddress(config.PrivValidatorListenAddr)
	conn, err := grpc.DialContext(ctx, address, dialOptions...)
	if err != nil {
		logger.Error("unable to connect to server", "target", address, "err", err)
	}

	return NewSignerClient(conn, chainID, logger)
}
