package enroll

import (
	"testing"

	"github.com/hanzozt/zt/v2/router/env"
	"github.com/stretchr/testify/require"
)

func TestCsrSubjectLeavesOutUnset(t *testing.T) {
	c := &env.Csr{Country: "US", Organization: "Hanzo AI", OrganizationalUnit: "ZT"}
	require.Equal(t, "CN=r,OU=ZT,O=Hanzo AI,C=US", csrSubject("r", c).String())
}
