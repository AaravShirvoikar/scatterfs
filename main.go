package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/AaravShirvoikar/scatterfs/crypto"
	"github.com/AaravShirvoikar/scatterfs/internal/fileserver"
	"github.com/AaravShirvoikar/scatterfs/p2p"
	"github.com/AaravShirvoikar/scatterfs/storage"
)

func makeFileServer(listenAddr, root string, nodes ...string) *fileserver.FileServer {
	tr := p2p.NewTCPTransport(listenAddr, p2p.DefaultHandshakeFunc, p2p.DefaultDecodeFunc, nil)
	s := storage.NewStorage(root, storage.DefaultPathTransformFunc)

	fs := fileserver.NewFileServer(tr, s, nodes, crypto.NewAESKey())

	tr.OnPeer = fs.OnPeer

	return fs
}

func main() {
	fs1 := makeFileServer(":9000", "9000_storage")
	fs2 := makeFileServer(":9001", "9001_storage", ":9000")
	fs3 := makeFileServer(":9002", "9002_storage", ":9000", ":9001")

	go fs1.Start()
	time.Sleep(time.Second * 2)

	go fs2.Start()
	time.Sleep(time.Second * 2)

	go fs3.Start()
	time.Sleep(time.Second * 2)

	key := "randomkey"
	// for i := range 10 {
	// 	data := bytes.NewReader([]byte("random data"))
	// 	fs2.Store(fmt.Sprintf("%s_%d", key, i), data)
	// 	time.Sleep(time.Millisecond * 100)
	// }

	data := bytes.NewReader([]byte("random data"))
	fs3.Store(key, data)
	time.Sleep(time.Millisecond * 100)

	if err := os.RemoveAll("9002_storage"); err != nil {
		log.Fatal(err)
	}
	if err := os.RemoveAll("9001_storage"); err != nil {
		log.Fatal(err)
	}

	r, err := fs3.Get(key)
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
