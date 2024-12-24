package p2p

import (
	"io"
)

const (
	IncomingMessage = 0x1
	IncomingStream  = 0x2
)

type Message struct {
	From    string
	Payload []byte
	Stream  bool
}

type DecodeFunc func(io.Reader, *Message) error

func DefaultDecodeFunc(r io.Reader, msg *Message) error {
	peek := make([]byte, 1)
	if _, err := r.Read(peek); err != nil {
		return err
	}

	stream := peek[0] == IncomingStream
	if stream {
		msg.Stream = true
		return nil
	}

	buff := make([]byte, 1028)
	n, err := r.Read(buff)
	if err != nil {
		return err
	}

	msg.Payload = buff[:n]
	return nil
}
