package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"time"

	"github.com/codegangsta/cli"
	"github.com/hashicorp/yamux"
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
	commkey := sha256.Sum256([]byte(key))

	// encoder
	block, err := aes.NewCipher(commkey[:])
	if err != nil {
		log.Println(err)
		return nil
	}
	sc.encoder = cipher.NewCFBEncrypter(block, iv)

	// decoder
	block, err = aes.NewCipher(commkey[:])
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

func (sc *secureConn) Close() (err error) {
	return sc.conn.Close()
}

func main() {
	rand.Seed(int64(time.Now().Nanosecond()))
	myApp := cli.NewApp()
	myApp.Name = "kcptun"
	myApp.Usage = "kcptun server"
	myApp.Version = "1.0"
	myApp.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "listen,l",
			Value: ":29900",
			Usage: "kcp server listen addr:",
		},
		cli.StringFlag{
			Name:  "target, t",
			Value: "127.0.0.1:12948",
			Usage: "target server addr",
		},
		cli.StringFlag{
			Name:  "key",
			Value: "it's a secrect",
			Usage: "key for communcation, must be the same as kcptun client",
		},
	}
	myApp.Action = func(c *cli.Context) {
		lis, err := kcp.ListenEncrypted(kcp.MODE_FAST, c.String("listen"), c.String("key"))
		if err != nil {
			log.Fatal(err)
		}

		log.Println("listening on ", lis.Addr())
		for {
			if conn, err := lis.Accept(); err == nil {
				conn.SetWindowSize(1024, 128)
				// p1
				p1 := newSecureConn(c.String("key"), conn)
				mux, err := yamux.Server(p1, nil)
				if err != nil {
					log.Println(err)
					continue
				}
				go handleMux(mux, c.String("target"))
			} else {
				log.Println(err)
			}
		}
	}
	myApp.Run(os.Args)
}

func handleMux(sess *yamux.Session, target string) {
	for {
		stream, err := sess.Accept()
		if err != nil {
			log.Println(err)
			return
		}
		go handleClient(stream, target)
	}
}

func handleClient(p1 net.Conn, target string) {
	log.Println("stream opened")
	defer log.Println("stream closed")

	// p2
	p2, err := net.Dial("tcp", target)
	if err != nil {
		log.Println(err)
		return
	}
	if tcpconn, ok := p2.(*net.TCPConn); ok {
		tcpconn.SetNoDelay(false)
	}
	defer p2.Close()

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
