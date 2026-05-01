// Copyright 2015 resumator authors.  All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
)

func (s *httpServer) route() {
	s.WrapLogHandlerFunc("/api/v1/tldr", s.tldrHandler)
	s.WrapLogHandlerFunc("/api/v1/stats", s.statsHandler)
}

func (s *httpServer) statsHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "hello, world\r\n")
}

func (s *httpServer) tldrHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "X-Requested-With, Content-Type")
		w.Header().Set("Access-Control-Allow-Method", "POST, GET, OPTIONS")
		b := r.URL.Query().Get("b")
		sm, err := Summarize(s.config.Sentences, b)
		if err != nil {
			log.Println(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Fprint(w, sm)
		break

	case "POST":
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Method", "POST, GET, OPTIONS")

		payload, err := ioutil.ReadAll(io.LimitReader(r.Body, 1048576))
		if err != nil {
			log.Println(err.Error())
			http.Error(w, "Error reading body", http.StatusInternalServerError)
			return
		}

		if err := r.Body.Close(); err != nil {
			log.Println(err.Error())
			http.Error(w, "Error closing body", http.StatusInternalServerError)
			return

		}

		if payload == nil {
			log.Println(err.Error())
			http.Error(w, "No payload", http.StatusNotFound)
			return
		}
		txtao := string(payload)

		sm, err := Summarize(s.config.Sentences, txtao)
		if err != nil {
			log.Println(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		/*
			if err = s.redis.Set("hello", sm); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		*/
		fmt.Fprint(w, sm)
		break
	case "OPTIONS":
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "X-Requested-With, Content-Type")
		w.Header().Set("Access-Control-Allow-Method", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Max-Age", "1728000")
		w.Header().Set("Content-Type", "text/plain charset=UTF-8")
		w.Header().Set("Content-Length", "0")
		w.WriteHeader(http.StatusNoContent)
		break
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

}
