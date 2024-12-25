package fileserver

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/AaravShirvoikar/scatterfs/p2p"
	"github.com/AaravShirvoikar/scatterfs/storage"
)

type FileServer struct {
	transport      p2p.Transport
	storage        *storage.Storage
	bootstrapNodes []string
	peerLock       sync.Mutex
	peers          map[string]p2p.Peer
	quitChan       chan struct{}
}

func NewFileServer(transport p2p.Transport, storage *storage.Storage, nodes []string) *FileServer {
	return &FileServer{
		transport:      transport,
		storage:        storage,
		bootstrapNodes: nodes,
		peers:          make(map[string]p2p.Peer),
		quitChan:       make(chan struct{}),
	}
}

type Message struct {
	Payload any
}

type MessageStore struct {
	Key  string
	Size int64
}

type MessageGet struct {
	Key string
}

func (s *FileServer) broadcast(msg *Message) error {
	buff := new(bytes.Buffer)
	if err := gob.NewEncoder(buff).Encode(msg); err != nil {
		return err
	}

	for _, peer := range s.peers {
		peer.Send([]byte{p2p.IncomingMessage})
		if err := peer.Send(buff.Bytes()); err != nil {
			return err
		}
	}

	return nil
}

func (s *FileServer) Get(key string) (io.Reader, error) {
	if s.storage.Exists(key) {
		log.Printf("[%s] serving file %s locally", s.transport.Addr(), key)
		_, r, err := s.storage.Read(key)
		return r, err
	}

	log.Printf("[%s] does not have file %s locally, fetching from network", s.transport.Addr(), key)

	msg := Message{
		Payload: MessageGet{
			Key: key,
		},
	}

	if err := s.broadcast(&msg); err != nil {
		return nil, err
	}

	time.Sleep(time.Millisecond * 500)

	for _, peer := range s.peers {
		var fileSize int64
		binary.Read(peer, binary.LittleEndian, &fileSize)
		n, err := s.storage.Write(key, io.LimitReader(peer, fileSize))
		if err != nil {
			return nil, err
		}

		log.Printf("[%s] received %d bytes over the network from %s\n", s.transport.Addr(), n, peer.RemoteAddr())

		peer.CloseStream()
	}

	_, r, err := s.storage.Read(key)
	return r, err
}

func (s *FileServer) Store(key string, r io.Reader) error {
	fileBuff := new(bytes.Buffer)
	tee := io.TeeReader(r, fileBuff)

	size, err := s.storage.Write(key, tee)
	if err != nil {
		return err
	}

	log.Printf("[%s] wrote %d bytes to storage\n", s.transport.Addr(), size)

	msg := Message{
		Payload: MessageStore{
			Key:  key,
			Size: size,
		},
	}

	if err := s.broadcast(&msg); err != nil {
		return err
	}

	time.Sleep(time.Millisecond * 500)

	for _, peer := range s.peers {
		peer.Send([]byte{p2p.IncomingStream})
		_, err := io.Copy(peer, fileBuff)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *FileServer) loop() {
	defer func() {
		log.Printf("[%s] file server stopped", s.transport.Addr())
		s.transport.Close()
	}()

	for {
		select {
		case msg := <-s.transport.Consume():
			var m Message
			if err := gob.NewDecoder(bytes.NewReader(msg.Payload)).Decode(&m); err != nil {
				fmt.Println("error decoding:", err)
				continue
			}

			if err := s.handleMessage(msg.From, &m); err != nil {
				fmt.Println("error handling message:", err)
			}
		case <-s.quitChan:
			return
		}
	}
}

func (s *FileServer) handleMessage(from string, msg *Message) error {
	switch v := msg.Payload.(type) {
	case MessageGet:
		return s.handleMessageGet(from, v)
	case MessageStore:
		return s.handleMessageStore(from, v)
	}

	return nil
}

func (s *FileServer) handleMessageGet(from string, msg MessageGet) error {
	if !s.storage.Exists(msg.Key) {
		return fmt.Errorf("[%s] does not have file %s", s.transport.Addr(), msg.Key)
	}

	log.Printf("[%s] has file %s, serving over the network", s.transport.Addr(), msg.Key)

	fileSize, r, err := s.storage.Read(msg.Key)
	if err != nil {
		return err
	}

	if rc, ok := r.(io.ReadCloser); ok {
		defer rc.Close()
	}

	peer, ok := s.peers[from]
	if !ok {
		return fmt.Errorf("peer not found")
	}

	peer.Send([]byte{p2p.IncomingStream})
	binary.Write(peer, binary.LittleEndian, fileSize)
	n, err := io.Copy(peer, r)
	if err != nil {
		return err
	}

	log.Printf("[%s] wrote %d bytes over the network to %s\n", s.transport.Addr(), n, from)

	return nil
}

func (s *FileServer) handleMessageStore(from string, msg MessageStore) error {
	peer, ok := s.peers[from]
	if !ok {
		return fmt.Errorf("peer not found")
	}

	n, err := s.storage.Write(msg.Key, io.LimitReader(peer, msg.Size))
	if err != nil {
		return err
	}

	log.Printf("[%s] received and wrote %d bytes to storage\n", s.transport.Addr(), n)

	peer.CloseStream()

	return nil
}

func (s *FileServer) bootstrapNetwork() error {
	for _, addr := range s.bootstrapNodes {
		log.Println("attempting to connect to:", addr)
		go func(addr string) {
			if err := s.transport.Dial(addr); err != nil {
				fmt.Println("dial error:", err)
			}
		}(addr)
	}

	return nil
}

func (s *FileServer) OnPeer(peer p2p.Peer) error {
	s.peerLock.Lock()
	s.peers[peer.RemoteAddr().String()] = peer
	s.peerLock.Unlock()

	log.Println("connected to remote:", peer.RemoteAddr())

	return nil
}

func (s *FileServer) Start() error {
	if err := s.transport.ListenAndAccept(); err != nil {
		return err
	}

	s.bootstrapNetwork()

	s.loop()

	return nil
}

func (s *FileServer) Stop() {
	close(s.quitChan)
}

func init() {
	gob.Register(MessageGet{})
	gob.Register(MessageStore{})
}
