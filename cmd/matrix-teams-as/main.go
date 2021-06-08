package main

import (
	"github.com/alexflint/go-arg"
	server "github.com/fossteams/matrix-teams-as/internal"
	"github.com/sirupsen/logrus"
	"net/url"
)

var args struct {
	Debug bool `arg:"-D" default:"false"`
	MatrixUrl string `arg:"-u" default:"http://localhost:8008"`

	HsToken string `arg:"-s,--home-server-token"`
	AsToken string `arg:"-a,--application-service-token"`
}

func main() {
	arg.MustParse(&args)

	logger := logrus.New()

	var opts []server.OptFunction
	if args.Debug {
		opts = append(opts, server.WithDebugMode)
	}

	if args.MatrixUrl != "" {
		matrixUrl, err := url.Parse(args.MatrixUrl)
		if err != nil {
			logger.Fatalf("unable to parse matrix URL: %v", err)
		}
		opts = append(opts, server.WithMatrixUrl(matrixUrl))
	}

	if args.HsToken != "" {
		opts = append(opts, server.WithHsToken(args.HsToken))
	}

	if args.AsToken != "" {
		opts = append(opts, server.WithAsToken(args.AsToken))
	}

	opts = append(opts, server.WithLogger(logger))

	s, err := server.New("0.0.0.0:8003", opts...)
	if err != nil {
		logger.Fatalf("unable to create server: %v", err)
	}
	err = s.Run()

	if err != nil {
		logger.Fatalf("unable to start server: %v", err)
	}
}
