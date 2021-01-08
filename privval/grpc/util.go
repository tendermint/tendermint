package grpc

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"os"
	"time"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/tendermint/tendermint/libs/log"
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
