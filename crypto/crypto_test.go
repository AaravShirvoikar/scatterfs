package crypto

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCrypto(t *testing.T) {
	payload := "random data"
	src := bytes.NewReader([]byte(payload))
	dst := new(bytes.Buffer)
	key := NewAESKey()

	_, err := CopyEncrypt(key, src, dst)

	assert.Nil(t, err)

	out := new(bytes.Buffer)
	n, err := CopyDecrypt(key, dst, out)

	assert.Nil(t, err)
	assert.Equal(t, 16+len(payload), n)
	assert.Equal(t, payload, out.String())
}
