package p2p

import (
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
)

type TCPPeer struct {
	net.Conn
	incoming bool
	Wg       sync.WaitGroup
}

func NewTCPPeer(conn net.Conn, incoming bool) *TCPPeer {
	return &TCPPeer{
		Conn:     conn,
		incoming: incoming,
	}
}

func (p *TCPPeer) Send(b []byte) error {
	_, err := p.Conn.Write(b)
	return err
}

type OnPeerFunc func(Peer) error

type TCPTransport struct {
	listenAddr string
	handshake  HandshakeFunc
	decode     DecodeFunc
	listener   net.Listener
	msgChan    chan Message
	OnPeer     OnPeerFunc
}

func NewTCPTransport(addr string, handshake HandshakeFunc, decode DecodeFunc, onPeer OnPeerFunc) *TCPTransport {
	return &TCPTransport{
		listenAddr: addr,
		handshake:  handshake,
		decode:     decode,
		OnPeer:     onPeer,
		msgChan:    make(chan Message),
	}
}

func (t *TCPTransport) Consume() <-chan Message {
	return t.msgChan
}

func (t *TCPTransport) Close() error {
	return t.listener.Close()
}

func (t *TCPTransport) Dial(addr string) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}

	go t.handleConn(conn, false)

	return nil
}

func (t *TCPTransport) ListenAndAccept() error {
	ln, err := net.Listen("tcp", t.listenAddr)
	if err != nil {
		return err
	}
	t.listener = ln

	go t.acceptLoop()

	log.Println("transport listening on port:", t.listenAddr)

	return nil
}

func (t *TCPTransport) acceptLoop() {
	for {
		conn, err := t.listener.Accept()
		if errors.Is(err, net.ErrClosed) {
			return
		}

		if err != nil {
			fmt.Println("error accepting connection:", err)
			continue
		}

		go t.handleConn(conn, true)
	}
}

func (t *TCPTransport) handleConn(conn net.Conn, incoming bool) {
	defer conn.Close()

	peer := NewTCPPeer(conn, incoming)

	if err := t.handshake(peer); err != nil {
		fmt.Println("handshake failed:", err)
		return
	}

	if t.OnPeer != nil {
		if err := t.OnPeer(peer); err != nil {
			fmt.Println("on peer function failed:", err)
			return
		}
	}

	for {
		msg := Message{}
		if err := t.decode(conn, &msg); err != nil {
			fmt.Println("error decoding message:", err)
			return
		}

		msg.From = conn.RemoteAddr().String()
		peer.Wg.Add(1)
		t.msgChan <- msg
		peer.Wg.Wait()
	}
}
