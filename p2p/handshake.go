package p2p

type HandshakeFunc func(Peer) error

func DefaultHandshakeFunc(Peer) error { return nil }
