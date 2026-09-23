package create

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/michaelquigley/pfxlog"
	"github.com/stretchr/testify/assert"
)

// These constants are hard-coded so that they may serve as a notifier. When the constants change in the source from
// which they came, we may need to update any related documentation. If the constant is changed, the tests should fail
// as they are expecting these hard-coded values. In which case, we update the hard-coded values and check any docs.
const (
	testDefaultCtrlEdgeAdvertisedPort         = "1280"
	testDefaultCtrlBindAddress                = "0.0.0.0"
	testDefaultCtrlAdvertisedPort             = "6262"
	testDefaultBoltCheckInterval              = "30s"
	testDefaultBoltCheckTimeout               = "20s"
	testDefaultBoltCheckInitialDelay          = "30s"
	testDefaultEdgeOptionsIdleTimeout         = "5000ms"
	testDefaultEdgeOptionsReadTimeout         = "5000ms"
	testDefaultEdgeOptionsWriteTimeout        = "100000ms"
	testDefaultEdgeOptionsMinTLSVersion       = "TLS1.2"
	testDefaultEdgeOptionsMaxTLSVersion       = "TLS1.3"
	testDefaultEdgeIdentityEnrollmentDuration = time.Duration(180) * time.Minute
	testDefaultEdgeIdentityEnrollmentStr      = "180m"
	testDefaultEdgeRouterEnrollmentDuration   = time.Duration(180) * time.Minute
	testDefaultEdgeRouterEnrollmentStr        = "180m"
)

func getEnvironmentVariables() []string {
	return []string{
		"ZT_HOME",
		"ZT_ROUTER_NAME",
		"ZT_ROUTER_PORT",
		"ZT_PKI_CTRL_CERT",
		"ZT_PKI_CTRL_SERVER_CERT",
		"ZT_PKI_CTRL_KEY",
		"ZT_PKI_CTRL_CA",
		"ZT_CTRL_BIND_ADDRESS",
		"ZT_CTRL_ADVERTISED_ADDRESS",
		"ZT_CTRL_DATABASE_FILE",
		"ZT_CTRL_EDGE_ALT_ADVERTISED_ADDRESS",
		"ZT_CTRL_ADVERTISED_PORT",
		"ZT_CONSOLE_LOCATION",
		"ZT_PKI_SIGNER_CERT",
		"ZT_PKI_SIGNER_KEY",
		"ZT_CTRL_EDGE_ADVERTISED_ADDRESS",
		"ZT_CTRL_EDGE_ADVERTISED_PORT",
		"ZT_PKI_EDGE_CA",
		"ZT_PKI_EDGE_KEY",
		"ZT_PKI_EDGE_SERVER_CERT",
		"ZT_PKI_EDGE_CERT",
		"ZT_ROUTER_IDENTITY_CERT",
		"ZT_ROUTER_IDENTITY_SERVER_CERT",
		"ZT_ROUTER_IDENTITY_KEY",
		"ZT_ROUTER_IDENTITY_CA",
		"ZT_ROUTER_IP_OVERRIDE",
		"ZT_ROUTER_TPROXY_RESOLVER",
		"ZT_ROUTER_DNS_IP_RANGE",
		"ZT_NETWORK_NAME",
		"ZT_EDGE_IDENTITY_ENROLLMENT_DURATION",
		"ZT_ROUTER_ENROLLMENT_DURATION",
		"ZT_ROUTER_ADVERTISED_ADDRESS",
		"ZT_ROUTER_LISTENER_BIND_PORT",
		"ZT_PKI_ALT_SERVER_CERT",
		"ZT_PKI_ALT_SERVER_KEY",
		"ZT_CTRL_EDGE_BIND_ADDRESS",
		"ZT_ROUTER_CSR_C",
		"ZT_ROUTER_CSR_ST",
		"ZT_ROUTER_CSR_L",
		"ZT_ROUTER_CSR_O",
		"ZT_ROUTER_CSR_OU",
		"ZT_ROUTER_CSR_SANS_DNS",
	}
}

func unsetZitiEnv() {
	// Unset environment variables
	envVars := getEnvironmentVariables()
	for i := 0; i < len(envVars); i++ {
		_ = os.Unsetenv(envVars[i])
	}
}

// Test that all ZT_* variables are included in the values for output
func TestNoUnknownOutputEnvVariablesExist(t *testing.T) {
	// Get the list of ZT_* environment variables
	allEnvVars := getEnvironmentVariables()

	// Run the environment options command and capture stdout
	cmd := NewCmdCreateConfigEnvironment()
	cmd.SetArgs([]string{"-o", "stdout"})
	output := captureOutput(func() {
		_ = cmd.Execute()
	})

	// Split the output on newlines
	lines := strings.Split(output, "\n")
	// Check that every known environment variable is represented in the env file output
	prefix := "ZT_"
	var unknownValues []string
	for _, line := range lines {
		// Only look at lines with a ZT_* env var
		if !strings.Contains(line, prefix) {
			continue
		}
		// Strip out the variable name and see if it's a known value
		start := strings.Index(line, prefix)
		end := strings.Index(line, "=")
		if end < 0 {
			// If there's no assignment, assume a variable name was referenced in a comment and ignore
			continue
		}
		envVar := strings.TrimSpace(line[start:end])
		if !contains(allEnvVars, envVar) {
			unknownValues = append(unknownValues, envVar)
		}
	}

	assert.Zero(t, len(unknownValues))
	for _, value := range unknownValues {
		fmt.Printf("The variable %s was found in env command output but was not expected.\n  -If this is a new variable, add it to the ZT env variables list in create_config_test.getEnvironmentVariables().\n  -If this variable was removed, remove the variable from the env command output.\n", value)
	}
}

// Test that all known ZT_* variables are included in the env file
func TestAllKnownEnvVariablesAreFoundInOutput(t *testing.T) {
	// Get the list of ZT_* environment variables
	allEnvVars := getEnvironmentVariables()

	// Run the environment options command and capture stdout
	cmd := NewCmdCreateConfigEnvironment()
	cmd.SetArgs([]string{"-o", "stdout"})
	output := captureOutput(func() {
		_ = cmd.Execute()
	})

	// Split the output on newlines
	lines := strings.Split(output, "\n")
	// Check that every known environment variable is represented in the env file output
	var unfoundVariables []string
	for _, variable := range allEnvVars {
		found := false
		for _, line := range lines {
			// Check if the line contains an assignment for this env variable
			assignment := variable + "="
			if strings.Contains(line, assignment) {
				found = true
				break
			}
		}
		if !found {
			unfoundVariables = append(unfoundVariables, variable)
		}
	}

	assert.Zero(t, len(unfoundVariables))
	for _, value := range unfoundVariables {
		fmt.Printf("The variable %s was expected in env command output but was not found.\n  -If this is variable was removed, remove it from the ZT env variables list in create_config_test.\n  -If this is a new variable, add it to the env command output.\n", value)
	}
}

// Test that all known ZT_* variables are included in the help output
func TestAllKnownEnvVariablesAreFoundInHelpOutput(t *testing.T) {
	// Get the list of ZT_* environment variables
	allEnvVars := getEnvironmentVariables()

	// Run the environment options command and capture stdout from help
	cmd := NewCmdCreateConfigEnvironment()
	cmd.SetArgs([]string{"-h"})
	output := captureOutput(func() {
		_ = cmd.Execute()
	})

	// Split the output on newlines
	lines := strings.Split(output, "\n")
	// Check that every known environment variable is represented in the help output
	var unfoundVariables []string
	for _, variable := range allEnvVars {
		found := false
		for _, line := range lines {
			// Check if the line contains an assignment for this env variable
			if strings.HasPrefix(line, variable) {
				found = true
				break
			}
		}
		if !found {
			unfoundVariables = append(unfoundVariables, variable)
		}
	}

	assert.Zero(t, len(unfoundVariables))
	for _, value := range unfoundVariables {
		fmt.Printf("The variable %s was expected in env command's help output but was not found.\n  -If this is variable was removed, remove it from the ZT env variables list in create_config_test.getEnvironmentVariables().\n  -If this is a new variable, add it to the env command's help output.\n", value)
	}
}

// Test that all ZT_* variables are included in the values for help output
func TestNoUnknownHelpEnvVariablesExist(t *testing.T) {
	// Get the list of ZT_* environment variables
	allEnvVars := getEnvironmentVariables()

	// Run the environment options command and capture stdout from help
	cmd := NewCmdCreateConfigEnvironment()
	cmd.SetArgs([]string{"-h"})
	output := captureOutput(func() {
		_ = cmd.Execute()
	})

	// Split the output on newlines
	lines := strings.Split(output, "\n")
	// Check that every known environment variable is represented in the env file output
	prefix := "ZT_"
	var unknownValues []string
	for _, line := range lines {
		// Only look at lines with a ZT_* env var
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		// Strip out the variable name and see if it's a known value
		start := strings.Index(line, prefix)
		end := strings.Index(line, " ")
		if end < 0 {
			// If there's no assignment, assume a variable name was referenced in a comment and ignore
			continue
		}
		envVar := strings.TrimSpace(line[start:end])
		if !contains(allEnvVars, envVar) {
			unknownValues = append(unknownValues, envVar)
		}
	}

	assert.Zero(t, len(unknownValues))
	for _, value := range unknownValues {
		fmt.Printf("The variable %s was found in env command's help output but was not expected.\n  -If this is a new variable, add it to the ZT env variables list in create_config_test.\n  -If this variable was removed, remove the variable from the env command's help output.\n", value)
	}
}

func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
}

// captureOutput hot-swaps os.Stdout in order to redirect all output to a memory buffer. Where possible, do not use
// this function and instead create optional arguments/configuration to redirect output to io.Writer instances. This
// should only be used for functionality that we do not control. Many instances of its usage are unnecessary and should
// be remedied with the aforementioned solution where possible.
func captureOutput(function func()) string {
	oldStdOut := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	defer func() {
		_ = r.Close()
	}()

	type readResult struct {
		out []byte
		err error
	}

	defer func() {
		os.Stdout = oldStdOut
	}()

	var output []byte
	var outputErr error

	outChan := make(chan *readResult, 1)

	// Start reading before writing, so we do not create backpressure that is never relieved in OSs with smaller buffers
	// than the resulting configuration file (i.e. Windows). Go will not yield to other routines unless there is
	// a system call. The fake os.Stdout will never yield and some code paths executed as `function()` may not
	// have syscalls.
	go func() {
		output, outputErr = io.ReadAll(r)
		outChan <- &readResult{
			output,
			outputErr,
		}
	}()

	function()

	os.Stdout = oldStdOut
	_ = w.Close()

	result := <-outChan

	if result == nil {
		panic("no output")
	}

	if result.err != nil {
		panic(result.err)
	}

	return string(result.out)
}

func setEnvByMap(m map[string]string) {
	for k, v := range m {
		if err := os.Setenv(k, v); err != nil {
			pfxlog.Logger().WithError(err).WithField("key", k).Error("failed to set env var")
		}
	}
}
