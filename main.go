package main

import (
	"bytes"
	"time"

	"github.com/AaravShirvoikar/scatterfs/internal/fileserver"
	"github.com/AaravShirvoikar/scatterfs/p2p"
	"github.com/AaravShirvoikar/scatterfs/storage"
)

func makeFileServer(listenAddr, root string, nodes ...string) *fileserver.FileServer {
	tr := p2p.NewTCPTransport(listenAddr, p2p.DefaultHandshakeFunc, p2p.DefaultDecodeFunc, nil)
	s := storage.NewStorage(root, storage.DefaultPathTransformFunc)

	fs := fileserver.NewFileServer(tr, s, nodes)

	tr.OnPeer = fs.OnPeer

	return fs
}

func main() {
	fs1 := makeFileServer(":9000", "9000_storage")
	fs2 := makeFileServer(":9001", "9001_storage", ":9000")

	go func() {
		fs1.Start()
	}()
	time.Sleep(time.Second * 2)

	go fs2.Start()
	time.Sleep(time.Second * 2)

	key := "randomkey"
	data := bytes.NewReader([]byte("random data"))

	fs2.StoreData(key, data)

	select {}
}
