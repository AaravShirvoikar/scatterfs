package fileserver

import (
	"bytes"
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

func (s *FileServer) broadcast(msg *Message) error {
	peers := []io.Writer{}
	for _, peer := range s.peers {
		peers = append(peers, peer)
	}

	mw := io.MultiWriter(peers...)

	return gob.NewEncoder(mw).Encode(msg)
}

func (s *FileServer) StoreData(key string, r io.Reader) error {
	fileBuff := new(bytes.Buffer)
	tee := io.TeeReader(r, fileBuff)

	size, err := s.storage.Write(key, tee)
	if err != nil {
		return err
	}

	log.Printf("wrote %d bytes\n", size)

	msgBuff := new(bytes.Buffer)
	msg := &Message{
		Payload: MessageStore{
			Key:  key,
			Size: size,
		},
	}

	if err := gob.NewEncoder(msgBuff).Encode(msg); err != nil {
		return err
	}

	for _, peer := range s.peers {
		if err := peer.Send(msgBuff.Bytes()); err != nil {
			return err
		}
	}

	time.Sleep(time.Second * 3)

	for _, peer := range s.peers {
		_, err := io.Copy(peer, fileBuff)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *FileServer) Stop() {
	close(s.quitChan)
}

func (s *FileServer) OnPeer(peer p2p.Peer) error {
	s.peerLock.Lock()
	s.peers[peer.RemoteAddr().String()] = peer
	s.peerLock.Unlock()

	log.Println("connected to remote:", peer.RemoteAddr())

	return nil
}

func (s *FileServer) loop() {
	defer func() {
		log.Println("file server stopped")
		s.transport.Close()
	}()

	for {
		select {
		case msg := <-s.transport.Consume():
			var m Message
			if err := gob.NewDecoder(bytes.NewReader(msg.Payload)).Decode(&m); err != nil {
				fmt.Println(err)
				return
			}

			if err := s.handleMessage(msg.From, &m); err != nil {
				fmt.Println(err)
				return
			}
		case <-s.quitChan:
			return
		}
	}
}

func (s *FileServer) handleMessage(from string, msg *Message) error {
	switch v := msg.Payload.(type) {
	case MessageStore:
		return s.handleMessageStore(from, v)
	}

	return nil
}

func (s *FileServer) handleMessageStore(from string, msg MessageStore) error {
	peer, ok := s.peers[from]
	if !ok {
		return fmt.Errorf("peer not fount")
	}

	n, err := s.storage.Write(msg.Key, io.LimitReader(peer, msg.Size))
	if err != nil {
		return err
	}

	log.Printf("wrote %d bytes\n", n)

	peer.(*p2p.TCPPeer).Wg.Done()

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

func (s *FileServer) Start() error {
	if err := s.transport.ListenAndAccept(); err != nil {
		return err
	}

	s.bootstrapNetwork()

	s.loop()

	return nil
}

func init() {
	gob.Register(MessageStore{})
}
