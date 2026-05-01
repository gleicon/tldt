//go:build ignore

// Copyright 2015 resumator authors.  All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"log"
	"net/http"
	"os"

	"github.com/fiorix/go-redis/redis"
	"github.com/gorilla/handlers"
)

type httpServer struct {
	config    *configFile
	redis     *redis.Client
	logStream *os.File
}

func (s *httpServer) init(cf *configFile, rc *redis.Client, ls *os.File) {
	s.config = cf
	s.redis = rc
	s.logStream = ls

	// Initialize http handlers.
	s.route()
}

func (s *httpServer) ListenAndServe() {
	if s.config.HTTP.Addr == "" {
		return
	}
	srv := http.Server{
		Addr: s.config.HTTP.Addr,
	}
	log.Println("Starting HTTP server on", s.config.HTTP.Addr)
	log.Fatal(srv.ListenAndServe())
}

func (s *httpServer) ListenAndServeTLS() {
	if s.config.HTTPS.Addr == "" {
		return
	}
	srv := http.Server{
		Addr: s.config.HTTPS.Addr,
	}
	log.Println("Starting HTTPS server on", s.config.HTTPS.Addr)
	log.Fatal(srv.ListenAndServeTLS(
		s.config.HTTPS.CertFile,
		s.config.HTTPS.KeyFile,
	))
}

func (s *httpServer) WrapLogHandlerFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	http.Handle(pattern, handlers.CombinedLoggingHandler(s.logStream, http.HandlerFunc(handler)))
}

func (s *httpServer) WrapLogHandler(pattern string, handler http.Handler) {
	http.Handle(pattern, handlers.CombinedLoggingHandler(s.logStream, handler))
}
