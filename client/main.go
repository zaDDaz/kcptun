package main

import (
	"crypto/aes"
	"crypto/cipher"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"time"

	"github.com/codegangsta/cli"
	"github.com/xtaci/kcp-go"
)

var (
	iv = []byte{147, 243, 201, 109, 83, 207, 190, 153, 204, 106, 86, 122, 71, 135, 200, 20}
)

type secureConn struct {
	encoder cipher.Stream
	decoder cipher.Stream
	conn    net.Conn
}

func newSecureConn(key string, conn net.Conn) *secureConn {
	sc := new(secureConn)
	sc.conn = conn
	commkey := make([]byte, 32)
	copy(commkey, []byte(key))

	// encoder
	block, err := aes.NewCipher(commkey)
	if err != nil {
		log.Println(err)
		return nil
	}
	sc.encoder = cipher.NewCFBEncrypter(block, iv)

	// decoder
	block, err = aes.NewCipher(commkey)
	if err != nil {
		log.Println(err)
		return nil
	}
	sc.decoder = cipher.NewCFBDecrypter(block, iv)
	return sc
}

func (sc *secureConn) Read(p []byte) (n int, err error) {
	n, err = sc.conn.Read(p)
	if err == nil {
		sc.decoder.XORKeyStream(p[:n], p[:n])
	}
	return
}

func (sc *secureConn) Write(p []byte) (n int, err error) {
	sc.encoder.XORKeyStream(p, p)
	return sc.conn.Write(p)
}

func handleClient(p1 *net.TCPConn, remote string, key string) {
	log.Println("stream opened")
	defer log.Println("stream closed")

	// p1
	p1.SetNoDelay(false)
	defer p1.Close()

	// connect to kcp server
	kcpserver, err := kcp.DialEncrypted(kcp.MODE_FAST, remote, key)
	kcpserver.SetWindowSize(128, 1024)
	if err != nil {
		log.Println(err)
	}
	defer kcpserver.Close()

	// p2
	p2 := newSecureConn(key, kcpserver)

	// start tunnel
	p1die := make(chan struct{})
	go func() {
		n, err := io.Copy(p1, p2)
		log.Println("from p2 -> p1 written:", n, "error:", err)
		close(p1die)
	}()

	p2die := make(chan struct{})
	go func() {
		n, err := io.Copy(p2, p1)
		log.Println("from p1 -> p2 written:", n, "error:", err)
		close(p2die)
	}()

	// wait for tunnel termination
	select {
	case <-p1die:
	case <-p2die:
	}
}

func checkError(err error) {
	if err != nil {
		log.Println(err)
		os.Exit(-1)
	}
}

func main() {
	rand.Seed(int64(time.Now().Nanosecond()))
	myApp := cli.NewApp()
	myApp.Name = "kcptun"
	myApp.Usage = "kcptun client"
	myApp.Version = "1.0"
	myApp.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "localaddr,l",
			Value: ":12948",
			Usage: "local listen addr:",
		},
		cli.StringFlag{
			Name:  "remoteaddr, r",
			Value: "vps:29900",
			Usage: "kcp server addr",
		},
		cli.StringFlag{
			Name:  "key",
			Value: "it's a secrect",
			Usage: "key for communcation, must be the same as kcptun server",
		},
	}
	myApp.Action = func(c *cli.Context) {
		addr, err := net.ResolveTCPAddr("tcp", c.String("localaddr"))
		checkError(err)
		listener, err := net.ListenTCP("tcp", addr)
		checkError(err)
		log.Println("listening on:", listener.Addr())
		for {
			conn, err := listener.AcceptTCP()
			if err != nil {
				log.Println("accept failed:", err)
				continue
			}
			go handleClient(conn, c.String("remoteaddr"), c.String("key"))
		}
	}
	myApp.Run(os.Args)
}
