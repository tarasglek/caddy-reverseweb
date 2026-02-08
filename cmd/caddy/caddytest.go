// This file is adapted from Caddy's caddytest test harness:
// https://github.com/caddyserver/caddy/blob/master/caddytest/caddytest.go
//
// Original work Copyright (c) Caddy Authors.
// Licensed under the Apache License, Version 2.0.
//
// Local modifications were made for this repository's integration tests.
package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	caddycmd "github.com/caddyserver/caddy/v2/cmd"
)

// Config stores configuration for running tests
type Config struct {
	AdminPort          int
	TestRequestTimeout time.Duration
	LoadRequestTimeout time.Duration
}

// Default test configuration
var Default = Config{
	AdminPort:          2999,
	TestRequestTimeout: 10 * time.Second,
	LoadRequestTimeout: 5 * time.Second,
}

// Tester represents a test client for Caddy
type Tester struct {
	Client       *http.Client
	configLoaded bool
	t            testing.TB
	config       Config
}

// NewTester creates a new test client
func NewTester(t testing.TB) *Tester {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("failed to create cookiejar: %s", err)
	}

	return &Tester{
		Client: &http.Client{
			Transport: CreateTestingTransport(),
			Jar:       jar,
			Timeout:   Default.TestRequestTimeout,
		},
		configLoaded: false,
		t:            t,
		config:       Default,
	}
}

// InitServer configures Caddy with the given config
func (tc *Tester) InitServer(rawConfig string, configType string) {
	if testing.Short() {
		tc.t.SkipNow()
		return
	}

	if err := tc.validatePrerequisites(); err != nil {
		tc.t.Skipf("skipping test: %s", err)
		return
	}

	tc.t.Cleanup(func() {
		if tc.t.Failed() && tc.configLoaded {
			res, err := http.Get(fmt.Sprintf("http://localhost:%d/config/", tc.config.AdminPort))
			if err != nil {
				tc.t.Log("unable to read current config")
				return
			}
			defer res.Body.Close()
			body, _ := io.ReadAll(res.Body)

			var out bytes.Buffer
			_ = json.Indent(&out, body, "", "  ")
			tc.t.Logf("----------- failed with config -----------\n%s", out.String())
		}
	})

	// Normalize JSON config
	if configType == "json" {
		var conf any
		if err := json.Unmarshal([]byte(rawConfig), &conf); err != nil {
			tc.t.Fatalf("invalid JSON: %v", err)
		}
		c, err := json.Marshal(conf)
		if err != nil {
			tc.t.Fatalf("cannot marshal config: %v", err)
		}
		rawConfig = string(c)
	}

	client := &http.Client{
		Timeout: tc.config.LoadRequestTimeout,
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("http://localhost:%d/load", tc.config.AdminPort), strings.NewReader(rawConfig))
	if err != nil {
		tc.t.Fatalf("failed to create request: %s", err)
	}

	if configType == "json" {
		req.Header.Add("Content-Type", "application/json")
	} else {
		req.Header.Add("Content-Type", "text/"+configType)
	}

	res, err := client.Do(req)
	if err != nil {
		tc.t.Fatalf("unable to contact caddy server: %s", err)
	}

	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		tc.t.Fatalf("unable to read response: %s", err)
	}

	if res.StatusCode != 200 {
		tc.t.Fatalf("config load failed (status %d): %s", res.StatusCode, string(body))
	}

	tc.configLoaded = true
}

const initConfig = `{
	admin localhost:%d
}
`

func (tc *Tester) validatePrerequisites() error {
	if tc.isCaddyAdminRunning() == nil {
		return nil
	}

	// Start Caddy in-process
	f, err := os.CreateTemp("", "caddy-config-*.caddyfile")
	if err != nil {
		return err
	}
	tc.t.Cleanup(func() {
		os.Remove(f.Name())
	})

	if _, err := fmt.Fprintf(f, initConfig, tc.config.AdminPort); err != nil {
		return err
	}

	// Start in-process Caddy server
	os.Args = []string{"caddy", "run", "--config", f.Name(), "--adapter", "caddyfile"}
	go func() {
		caddycmd.Main()
	}()

	// Wait for Caddy to start
	for retries := 10; retries > 0 && tc.isCaddyAdminRunning() != nil; retries-- {
		time.Sleep(500 * time.Millisecond)
	}

	return tc.isCaddyAdminRunning()
}

func (tc *Tester) isCaddyAdminRunning() error {
	client := &http.Client{
		Timeout: tc.config.LoadRequestTimeout,
	}
	resp, err := client.Get(fmt.Sprintf("http://localhost:%d/config/", tc.config.AdminPort))
	if err != nil {
		return fmt.Errorf("caddy not running on localhost:%d", tc.config.AdminPort)
	}
	resp.Body.Close()
	return nil
}

// CreateTestingTransport creates a transport that redirects all dial to localhost
func CreateTestingTransport() *http.Transport {
	dialer := net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 5 * time.Second,
	}

	dialContext := func(ctx context.Context, network, addr string) (net.Conn, error) {
		parts := strings.Split(addr, ":")
		destAddr := fmt.Sprintf("127.0.0.1:%s", parts[len(parts)-1])
		return dialer.DialContext(ctx, network, destAddr)
	}

	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
	}
}

// AssertGetResponse makes a GET request and asserts the response
func (tc *Tester) AssertGetResponse(requestURI string, expectedStatusCode int, expectedBody string) (*http.Response, string) {
	tc.t.Helper()

	req, err := http.NewRequest("GET", requestURI, nil)
	if err != nil {
		tc.t.Fatalf("unable to create request: %s", err)
	}

	return tc.AssertResponse(req, expectedStatusCode, expectedBody)
}

// AssertResponse executes a request and asserts the response
func (tc *Tester) AssertResponse(req *http.Request, expectedStatusCode int, expectedBody string) (*http.Response, string) {
	tc.t.Helper()

	resp, err := tc.Client.Do(req)
	if err != nil {
		tc.t.Fatalf("failed to call server: %s", err)
	}
	defer resp.Body.Close()

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		tc.t.Fatalf("unable to read response body: %s", err)
	}

	body := string(bytes)

	if expectedStatusCode != resp.StatusCode {
		tc.t.Errorf("requesting %q expected status %d but got %d (body: %s)", req.URL.RequestURI(), expectedStatusCode, resp.StatusCode, body)
	}

	if expectedBody != "" && !strings.Contains(body, expectedBody) {
		tc.t.Errorf("requesting %q expected body to contain %q but got %q", req.URL.RequestURI(), expectedBody, body)
	}

	return resp, body
}

// AssertResponseCode executes a request and only checks the status code
func (tc *Tester) AssertResponseCode(req *http.Request, expectedStatusCode int) *http.Response {
	tc.t.Helper()

	resp, err := tc.Client.Do(req)
	if err != nil {
		tc.t.Fatalf("failed to call server: %s", err)
	}

	if expectedStatusCode != resp.StatusCode {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		tc.t.Errorf("requesting %q expected status %d but got %d (body: %s)", req.URL.RequestURI(), expectedStatusCode, resp.StatusCode, string(body))
	}

	return resp
}

// getIntegrationDir returns the directory of the current test file
func getIntegrationDir() string {
	_, filename, _, ok := runtime.Caller(1)
	if !ok {
		panic("unable to determine current file path")
	}
	return path.Dir(filename)
}

// validateTestPrerequisites is a simplified version for non-caddytest needs
func validateTestPrerequisites(tc *Tester) error {
	// Check certificates are found in caddytest directory
	certs := []string{"/caddy.localhost.crt", "/caddy.localhost.key"}
	integrationDir := getIntegrationDir()

	for _, certName := range certs {
		if _, err := os.Stat(integrationDir + certName); errors.Is(err, fs.ErrNotExist) {
			// Certificates not found, but that's OK for our tests
			// We don't use HTTPS in reverse-bin tests
		}
	}

	return tc.isCaddyAdminRunning()
}

// DeepEqual is a helper for comparing values
func DeepEqual(expected, actual any) bool {
	return reflect.DeepEqual(expected, actual)
}
