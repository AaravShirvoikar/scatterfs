package main

import (
	"fmt"
	"io"
	"log"
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
	// for i := range 10 {
	// 	data := bytes.NewReader([]byte("random data"))
	// 	fs2.Store(fmt.Sprintf("%s_%d", key, i), data)
	// 	time.Sleep(time.Millisecond * 100)
	// }

	// data := bytes.NewReader([]byte("random data"))
	// fs2.Store(key, data)
	// time.Sleep(time.Millisecond * 100)

	r, err := fs2.Get(key)
	if err != nil {
		log.Fatal(err)
	}

	b, err := io.ReadAll(r)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(b))

	select {}
}
