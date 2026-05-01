// Copyright 2015 resumator authors.  All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/fiorix/go-redis/redis"
)

var (
	VERSION = "0.0.1"
	APPNAME = "resumator"
)

func main() {
	configFile := flag.String("c", "resumator.conf", "")
	logFile := flag.String("l", "", "")
	flag.Usage = func() {
		fmt.Println("Usage: resumator [-c resumator.conf] [-l logfile]")
		os.Exit(1)
	}
	flag.Parse()

	var err error
	config, err := loadConfig(*configFile)
	if err != nil {
		log.Fatal(err)
	}

	// Initialize log.
	if *logFile != "" {
		setLog(*logFile)
	}

	// Set up databases.
	rc := redis.New(config.DB.Redis)

	log.Printf("%s %s", APPNAME, VERSION)

	// Start HTTP server.
	s := new(httpServer)
	s.init(config, rc, os.Stdout)
	go s.ListenAndServe()
	go s.ListenAndServeTLS()

	// Sleep forever.
	select {}
}

func setLog(filename string) {
	f := openLog(filename)
	log.SetOutput(f)
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGHUP)
	go func() {
		// Recycle log file on SIGHUP.
		var fb *os.File
		for {
			<-sigc
			fb = f
			f = openLog(filename)
			log.SetOutput(f)
			fb.Close()
		}
	}()
}

func openLog(filename string) *os.File {
	f, err := os.OpenFile(
		filename,
		os.O_WRONLY|os.O_CREATE|os.O_APPEND,
		0644,
	)
	if err != nil {
		log.SetOutput(os.Stderr)
		log.Fatal(err)
	}
	return f
}
