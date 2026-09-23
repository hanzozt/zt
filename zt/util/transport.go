// Package util provides utility functions for the ZT CLI, including HTTP transport
// creation and configuration for communicating over ZT networks.
package util

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/hanzozt/sdk-golang/zt"
	"github.com/hanzozt/zt/v2/zt/constants"
)

// NewTransportFromSlice creates an HTTP transport configured to route
// connections through a ZT network. The provided bytes should contain a JSON-encoded
// ZT configuration.
//
// By default, urls are expected to leverage intercepts. Create a service and assign an appropriate
// intercept config and use the intercept address when dialing.
//
// To support addressable terminators-based dialing a user should be specified in the URL. This activates
// the dial-by-identity functionality. In this mode the url should be in the form of
// "identity-to-dial@service-name-to-dial". The transport uses the Proxy hook to extract user identity
// information from request URLs and passes it to ZT dial operation via DialOptions.
//
// Returns an error if the configuration is invalid or ZT context creation fails.
func NewTransportFromSlice(bytes []byte, terminator string) (*http.Transport, error) {
	ztx, cerr := NewContextFromSlice(bytes)
	if cerr != nil {
		return nil, cerr
	}

	ztTransport := http.DefaultTransport.(*http.Transport).Clone()

	opts := zt.DialOptions{
		Identity: terminator,
	}
	ztTransport.DialContext = NewDialContext(ztx, opts)

	return ztTransport, nil
}

// TransportFromEnv creates a ZT-enabled HTTP transport by reading a
// base64-encoded ZT identity from the default environment variable
// (CliNetworkIdVarName from constants).
//
// Returns (nil, nil) if the environment variable is not set, or (transport, error)
// if there's an issue creating the transport.
func TransportFromEnv(terminator string) (*http.Transport, error) {
	return TransportFromEnvByName(constants.CliNetworkIdVarName, terminator)
}

// TransportFromEnvByName creates a ZT-enabled HTTP transport by reading
// a base64-encoded ZT identity from the specified environment variable.
//
// The environment variable should contain a base64-encoded ZT configuration.
// Returns (nil, nil) if the environment variable is not set, or (transport, error)
// if there are issues with decoding or configuration creation.
func TransportFromEnvByName(envVarName string, terminator string) (*http.Transport, error) {
	data, err := ConfigFromEnvByName(envVarName)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}
	return NewTransportFromSlice(data, terminator)
}

// NewTransportFromFile creates a ZT-enabled HTTP transport by reading
// a ZT configuration from a file. The file should contain JSON-encoded ZT
// configuration data.
//
// Returns an error if the file cannot be read or contains invalid configuration.
func NewTransportFromFile(pathToFile string, terminator string) (*http.Transport, error) {
	data, err := os.ReadFile(pathToFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read zt identity file %s: %v", pathToFile, err)
	}
	return NewTransportFromSlice(data, terminator)
}

// NewDialContext creates a dial context function that routes connections through
// a ZT network. The returned function can be used as the DialContext for http.Transport.
//
// If opts.Identity is specified, the function performs addressable terminator-based dialing
// by extracting the hostname from the address and passing it to ZT. Otherwise, it uses
// the fallback dialer from the context collection.
func NewDialContext(zc zt.Context, opts zt.DialOptions) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		dialer := ztCliContextCollection.NewDialerWithFallback(ctx, &net.Dialer{})
		if opts.Identity != "" {
			hostParts := strings.Split(addr, ":")
			return zc.DialWithOptions(hostParts[0], &opts)
		} else {
			return dialer.Dial(network, addr)
		}
	}
}

// NewContextFromSlice creates a ZT context from JSON-encoded configuration bytes.
// The context is configured to retrieve all config types and is added to the global context
// collection. Services are loaded and validated before returning.
//
// Returns an error if the configuration is invalid, context creation fails, or services
// cannot be retrieved.
func NewContextFromSlice(bytes []byte) (zt.Context, error) {
	if len(bytes) == 0 {
		return nil, nil
	}
	cfg := &zt.Config{}
	if err := json.Unmarshal(bytes, &cfg); err != nil {
		return nil, err
	}
	cfg.ConfigTypes = append(cfg.ConfigTypes, "all")

	zc, zce := zt.NewContext(cfg)
	if zce != nil {
		return nil, fmt.Errorf("failed to create zt context: %v", zce)
	}
	ztCliContextCollection.Add(zc)

	if _, se := zc.GetServices(); se != nil {
		return nil, fmt.Errorf("failed to get zt services: %v", se)
	}

	_, se := zc.GetServices() // loads all the services
	if se != nil {
		return nil, fmt.Errorf("failed to get zt services: %v", se)
	}

	return zc, nil
}

// ConfigFromEnv reads a base64-encoded ZT configuration from the default
// environment variable (CliNetworkIdVarName from constants).
//
// Returns (nil, nil) if the environment variable is not set, or (config, error)
// if there are issues with decoding.
func ConfigFromEnv() ([]byte, error) {
	return ConfigFromEnvByName(constants.CliNetworkIdVarName)
}

// ConfigFromEnvByName reads a base64-encoded ZT configuration from the specified
// environment variable.
//
// Returns (nil, nil) if the environment variable is not set, or (config, error) if there
// are issues decoding the base64-encoded configuration.
func ConfigFromEnvByName(envVarName string) ([]byte, error) {
	b64Zid := os.Getenv(envVarName)
	if b64Zid == "" {
		return nil, nil
	}
	idReader := base64.NewDecoder(base64.StdEncoding, strings.NewReader(b64Zid))
	data, err := io.ReadAll(idReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read and decode zt identity: %v", err)
	}
	return data, nil
}
