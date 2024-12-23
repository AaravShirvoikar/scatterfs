package p2p

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTCPTransport(t *testing.T) {
	tr := NewTCPTransport(":9000", DefaultHandshakeFunc, DefaultDecodeFunc, nil)

	assert.Equal(t, ":9000", tr.listenAddr)

	assert.Nil(t, tr.ListenAndAccept())
}
