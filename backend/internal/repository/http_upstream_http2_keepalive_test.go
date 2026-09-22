package repository

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnableHTTP2KeepAliveUsesConservativeTimeouts(t *testing.T) {
	transport := &http.Transport{}
	h2, err := enableHTTP2KeepAlive(transport)
	require.NoError(t, err)
	require.NotNil(t, h2)
	require.Equal(t, upstreamHTTP2ReadIdleTimeout, h2.ReadIdleTimeout)
	require.Equal(t, upstreamHTTP2PingTimeout, h2.PingTimeout)
}

func TestBuildUpstreamTransportEnablesHTTP2Negotiation(t *testing.T) {
	transport, err := buildUpstreamTransport(poolSettings{}, nil)
	require.NoError(t, err)
	require.True(t, transport.ForceAttemptHTTP2)
	require.Contains(t, transport.TLSNextProto, "h2")
	transport.CloseIdleConnections()
}
