package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/AaravShirvoikar/scatterfs/crypto"
	"github.com/AaravShirvoikar/scatterfs/internal/fileserver"
	"github.com/AaravShirvoikar/scatterfs/p2p"
	"github.com/AaravShirvoikar/scatterfs/storage"
)

func makeFileServer(listenAddr string, nodes ...string) *fileserver.FileServer {
	keyPath := fmt.Sprintf("encryption_keys/%s_key", listenAddr)

	if err := os.MkdirAll("encryption_keys", 0755); err != nil {
		log.Fatal(err)
	}

	var encKey []byte
	if _, err := os.Stat(keyPath); err == nil {
		keyData, err := os.ReadFile(keyPath)
		if err != nil {
			log.Fatal(err)
		}
		encKey = keyData
	} else if os.IsNotExist(err) {
		encKey = crypto.NewAESKey()
		if err := os.WriteFile(keyPath, encKey, 0600); err != nil {
			log.Fatal(err)
		}
	} else {
		log.Fatal(err)
	}

	storagePath := fmt.Sprintf("file_storage/%s_storage", listenAddr)

	tr := p2p.NewTCPTransport(listenAddr, p2p.DefaultHandshakeFunc, p2p.DefaultDecodeFunc, nil)
	s := storage.NewStorage(storagePath, storage.DefaultPathTransformFunc)

	fs := fileserver.NewFileServer(tr, s, nodes, encKey)

	tr.OnPeer = fs.OnPeer

	return fs
}

func main() {
	fileservers := []*fileserver.FileServer{}
	fileservers = append(fileservers, makeFileServer(":9000"))
	fileservers = append(fileservers, makeFileServer(":9001", ":9000"))
	fileservers = append(fileservers, makeFileServer(":9002", ":9000", ":9001"))

	for _, fs := range fileservers {
		go fs.Start()
		time.Sleep(time.Second * 2)
	}

	for {
		fmt.Println("\nSelect a File Server:")
		for i := range fileservers {
			fmt.Printf("[%d] FileServer %d\n", i+1, i+1)
		}
		fmt.Print("[0] Exit\n> ")

		var ch int
		_, err := fmt.Scanln(&ch)
		if err != nil || ch < 0 || ch > len(fileservers) {
			fmt.Println("Invalid choice.")
			continue
		}
		if ch == 0 {
			fmt.Println("Exiting program...")
			return
		}

		currFs := fileservers[ch-1]

		for {
			fmt.Printf("\nSelect an operation for FileServer %d:\n", ch)
			fmt.Println("[1] Store file")
			fmt.Println("[2] Get file")
			fmt.Println("[3] Delete file globally")
			fmt.Println("[4] Delete file locally")
			fmt.Print("[0] Back to server selection\n> ")

			var opCh int
			_, err := fmt.Scanln(&opCh)
			if err != nil || opCh < 0 || opCh > 4 {
				fmt.Println("Invalid choice.")
				continue
			}

			if opCh == 0 {
				break
			}

			switch opCh {
			case 1:
				var fileName, fileData string
				fmt.Print("Enter file name: ")
				fmt.Scanln(&fileName)
				reader := bufio.NewReader(os.Stdin)
				fmt.Print("Enter file data: ")
				fileData, err := reader.ReadString('\n')
				if err != nil {
					fmt.Println("Failed to read input:", err)
					continue
				}
				fileData = strings.TrimSpace(fileData)

				err = currFs.Store(fileName, bytes.NewReader([]byte(fileData)))
				time.Sleep(time.Second * 1)
				if err != nil {
					fmt.Println("Failed to store file:", err)
				} else {
					fmt.Println("File stored successfully.")
				}

			case 2:
				var fileName string
				fmt.Print("Enter file name: ")
				fmt.Scanln(&fileName)

				r, err := currFs.Get(fileName)
				time.Sleep(time.Second * 1)
				if err != nil {
					fmt.Println("Failed to get file:", err)
					continue
				}

				fileData, err := io.ReadAll(r)
				if err != nil {
					fmt.Println("Failed to file data:", err)
					continue
				}

				fmt.Println("File data:", string(fileData))

			case 3:
				var fileName string
				fmt.Print("Enter file name: ")
				fmt.Scanln(&fileName)

				err := currFs.Remove(fileName)
				time.Sleep(time.Second * 1)
				if err != nil {
					fmt.Println("Failed to delete file:", err)
				} else {
					fmt.Println("File deleted successfully.")
				}

			case 4:
				var fileName string
				fmt.Print("Enter file name: ")
				fmt.Scanln(&fileName)

				err := currFs.RemoveLocal(fileName)
				time.Sleep(time.Second * 1)
				if err != nil {
					fmt.Println("Failed to delete file:", err)
				} else {
					fmt.Println("File deleted successfully.")
				}
			}
		}
	}
}
