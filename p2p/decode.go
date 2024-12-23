package p2p

import (
	"io"
)

type Message struct {
	From    string
	Payload []byte
}

type DecodeFunc func(io.Reader, *Message) error

func DefaultDecodeFunc(r io.Reader, msg *Message) error {
	const bufferSize = 1028
	buff := make([]byte, bufferSize)
	n, err := r.Read(buff)
	if err != nil {
		return err
	}

	msg.Payload = buff[:n]
	return nil
}
