package server

import (
	"github.com/sirupsen/logrus"
	"net/url"
)

type OptFunction func(*Server)

func WithDebugMode(s *Server) {
	s.debug = true
}

func WithHsToken(token string) func(s *Server) {
	return func(s *Server) {
		s.hsToken = token
	}
}

func WithAsToken(token string) func(s *Server) {
	return func(s *Server) {
		s.asToken = token
	}
}

func WithMatrixUrl(url *url.URL) func(s *Server) {
	return func(s *Server) {
		if url == nil {
			return
		}
		s.matrixUrl = *url
	}
}

func WithLogger(logger *logrus.Logger) func(s *Server) {
	return func(s *Server) {
		if logger == nil {
			return
		}
		s.logger = logger
	}
}
