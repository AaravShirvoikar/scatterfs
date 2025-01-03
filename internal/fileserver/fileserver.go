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

	"github.com/AaravShirvoikar/scatterfs/crypto"
	"github.com/AaravShirvoikar/scatterfs/p2p"
	"github.com/AaravShirvoikar/scatterfs/storage"
)

type FileServer struct {
	transport      p2p.Transport
	storage        *storage.Storage
	bootstrapNodes []string
	encKey         []byte
	peerLock       sync.Mutex
	peers          map[string]p2p.Peer
	quitChan       chan struct{}
}

func NewFileServer(transport p2p.Transport, storage *storage.Storage, nodes []string, encKey []byte) *FileServer {
	return &FileServer{
		transport:      transport,
		storage:        storage,
		bootstrapNodes: nodes,
		encKey:         encKey,
		peers:          make(map[string]p2p.Peer),
		quitChan:       make(chan struct{}),
	}
}

type Message struct {
	Payload any
}

type MessageGet struct {
	Key string
}

type MessageStore struct {
	Key  string
	Size int64
}

type MessageRemove struct {
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
		if err != nil {
			return nil, err
		}

		decBuff := new(bytes.Buffer)
		_, err = crypto.CopyDecrypt(s.encKey, r, decBuff)
		return decBuff, err
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

	timeout := time.After(time.Second * 3)
	done := make(chan bool)

	for _, peer := range s.peers {
		go func(peer p2p.Peer) {
			var fileSize int64
			if err := binary.Read(peer, binary.LittleEndian, &fileSize); err != nil {
				return
			}

			if fileSize == -1 {
				peer.CloseStream()
				return
			}

			encBuff := new(bytes.Buffer)
			if _, err := crypto.CopyEncrypt(s.encKey, io.LimitReader(peer, fileSize), encBuff); err != nil {
				return
			}

			if _, err := s.storage.Write(key, encBuff); err != nil {
				return
			}

			log.Printf("[%s] received %d bytes over the network from %s", s.transport.Addr(), fileSize, peer.RemoteAddr())

			peer.CloseStream()
			done <- true
		}(peer)
	}

	select {
	case <-done:
		_, r, err := s.storage.Read(key)
		if err != nil {
			return nil, err
		}

		decBuff := new(bytes.Buffer)
		_, err = crypto.CopyDecrypt(s.encKey, r, decBuff)
		return decBuff, err
	case <-timeout:
		return nil, fmt.Errorf("server timed out while fetching file %s from network", key)
	}
}

func (s *FileServer) Store(key string, r io.Reader) error {
	fileBuff := new(bytes.Buffer)
	tee := io.TeeReader(r, fileBuff)

	encBuff := new(bytes.Buffer)
	if _, err := crypto.CopyEncrypt(s.encKey, tee, encBuff); err != nil {
		return err
	}

	size, err := s.storage.Write(key, encBuff)
	if err != nil {
		return err
	}

	log.Printf("[%s] wrote %d bytes to storage", s.transport.Addr(), size)

	fileSize := fileBuff.Len()
	msg := Message{
		Payload: MessageStore{
			Key:  key,
			Size: int64(fileSize),
		},
	}

	if err := s.broadcast(&msg); err != nil {
		return err
	}

	time.Sleep(time.Millisecond * 500)

	peers := []io.Writer{}
	for _, peer := range s.peers {
		peers = append(peers, peer)
	}
	mw := io.MultiWriter(peers...)

	mw.Write([]byte{p2p.IncomingStream})
	_, err = io.Copy(mw, fileBuff)
	if err != nil {
		return err
	}

	return nil
}

func (s *FileServer) Remove(key string) error {
	if s.storage.Exists(key) {
		log.Printf("[%s] removed file %s", s.transport.Addr(), key)
		if err := s.storage.Delete(key); err != nil {
			return err
		}
	} else {
		log.Printf("[%s] does not have file %s", s.transport.Addr(), key)
	}

	msg := Message{
		Payload: MessageRemove{
			Key: key,
		},
	}

	return s.broadcast(&msg)
}

func (s *FileServer) RemoveLocal(key string) error {
	if s.storage.Exists(key) {
		log.Printf("[%s] removed file %s", s.transport.Addr(), key)
		return s.storage.Delete(key)
	}

	log.Printf("[%s] does not have file %s", s.transport.Addr(), key)

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
				log.Println(err)
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
	case MessageRemove:
		return s.handleMessageRemove(from, v)
	}

	return nil
}

func (s *FileServer) handleMessageGet(from string, msg MessageGet) error {
	if !s.storage.Exists(msg.Key) {
		peer, ok := s.peers[from]
		if !ok {
			return fmt.Errorf("peer not found")
		}

		fileSize := -1
		peer.Send([]byte{p2p.IncomingStream})
		binary.Write(peer, binary.LittleEndian, int64(fileSize))
		return fmt.Errorf("[%s] does not have file %s", s.transport.Addr(), msg.Key)
	}

	log.Printf("[%s] has file %s, serving over the network", s.transport.Addr(), msg.Key)

	_, r, err := s.storage.Read(msg.Key)
	if err != nil {
		return err
	}

	decBuff := new(bytes.Buffer)
	_, err = crypto.CopyDecrypt(s.encKey, r, decBuff)
	if err != nil {
		return err
	}

	fileSize := decBuff.Len()

	if rc, ok := r.(io.ReadCloser); ok {
		defer rc.Close()
	}

	peer, ok := s.peers[from]
	if !ok {
		return fmt.Errorf("peer not found")
	}

	peer.Send([]byte{p2p.IncomingStream})
	binary.Write(peer, binary.LittleEndian, int64(fileSize))
	n, err := io.Copy(peer, decBuff)
	if err != nil {
		return err
	}

	log.Printf("[%s] wrote %d bytes over the network to %s", s.transport.Addr(), n, from)

	return nil
}

func (s *FileServer) handleMessageStore(from string, msg MessageStore) error {
	peer, ok := s.peers[from]
	if !ok {
		return fmt.Errorf("peer not found")
	}

	encBuff := new(bytes.Buffer)
	_, err := crypto.CopyEncrypt(s.encKey, io.LimitReader(peer, msg.Size), encBuff)
	if err != nil {
		return err
	}

	n, err := s.storage.Write(msg.Key, encBuff)
	if err != nil {
		return err
	}

	log.Printf("[%s] received %d bytes and wrote %d bytes to storage", s.transport.Addr(), msg.Size, n)

	peer.CloseStream()

	return nil
}

func (s *FileServer) handleMessageRemove(from string, msg MessageRemove) error {
	log.Printf("[%s] received file delete request from %s", s.transport.Addr(), from)
	if s.storage.Exists(msg.Key) {
		log.Printf("[%s] removed file %s", s.transport.Addr(), msg.Key)
		return s.storage.Delete(msg.Key)
	}

	log.Printf("[%s] does not have file %s", s.transport.Addr(), msg.Key)

	return nil
}

func (s *FileServer) bootstrapNetwork() error {
	for _, addr := range s.bootstrapNodes {
		log.Printf("[%s] attempting to connect to %s", s.transport.Addr(), addr)
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

	log.Printf("[%s] connected to remote %s", s.transport.Addr(), peer.RemoteAddr())

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
	gob.Register(MessageRemove{})
}
