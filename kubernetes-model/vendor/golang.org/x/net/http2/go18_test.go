/**
 * Copyright (C) 2015 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *         http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build go1.8

package http2

import (
	"crypto/tls"
	"net/http"
	"testing"
	"time"
)

// Tests that http2.Server.IdleTimeout is initialized from
// http.Server.{Idle,Read}Timeout. http.Server.IdleTimeout was
// added in Go 1.8.
func TestConfigureServerIdleTimeout_Go18(t *testing.T) {
	const timeout = 5 * time.Second
	const notThisOne = 1 * time.Second

	// With a zero http2.Server, verify that it copies IdleTimeout:
	{
		s1 := &http.Server{
			IdleTimeout: timeout,
			ReadTimeout: notThisOne,
		}
		s2 := &Server{}
		if err := ConfigureServer(s1, s2); err != nil {
			t.Fatal(err)
		}
		if s2.IdleTimeout != timeout {
			t.Errorf("s2.IdleTimeout = %v; want %v", s2.IdleTimeout, timeout)
		}
	}

	// And that it falls back to ReadTimeout:
	{
		s1 := &http.Server{
			ReadTimeout: timeout,
		}
		s2 := &Server{}
		if err := ConfigureServer(s1, s2); err != nil {
			t.Fatal(err)
		}
		if s2.IdleTimeout != timeout {
			t.Errorf("s2.IdleTimeout = %v; want %v", s2.IdleTimeout, timeout)
		}
	}

	// Verify that s1's IdleTimeout doesn't overwrite an existing setting:
	{
		s1 := &http.Server{
			IdleTimeout: notThisOne,
		}
		s2 := &Server{
			IdleTimeout: timeout,
		}
		if err := ConfigureServer(s1, s2); err != nil {
			t.Fatal(err)
		}
		if s2.IdleTimeout != timeout {
			t.Errorf("s2.IdleTimeout = %v; want %v", s2.IdleTimeout, timeout)
		}
	}
}

func TestCertClone(t *testing.T) {
	c := &tls.Config{
		GetClientCertificate: func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			panic("shouldn't be called")
		},
	}
	c2 := cloneTLSConfig(c)
	if c2.GetClientCertificate == nil {
		t.Error("GetClientCertificate is nil")
	}
}
