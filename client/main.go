package main

import (
	"crypto/aes"
	"crypto/cipher"
	"io"
	"log"
	"net"
	"os"

	"github.com/codegangsta/cli"
	"github.com/xtaci/kcp-go"
)

var (
	ch_buf chan []byte
	iv     []byte = []byte{147, 243, 201, 109, 83, 207, 190, 153, 204, 106, 86, 122, 71, 135, 200, 20}
)

type SecureConn struct {
	encoder cipher.Stream
	decoder cipher.Stream
	conn    net.Conn
}

func NewSecureConn(key string, conn net.Conn) *SecureConn {
	sc := new(SecureConn)
	sc.conn = conn
	commkey := make([]byte, 32)
	copy(commkey, []byte(key))

	// encoder
	block, err := aes.NewCipher(commkey)
	if err != nil {
		log.Println(err)
		return nil
	}
	sc.encoder = cipher.NewCTR(block, iv)

	// decoder
	block, err = aes.NewCipher(commkey)
	if err != nil {
		log.Println(err)
		return nil
	}
	sc.decoder = cipher.NewCTR(block, iv)
	return sc
}

func (sc *SecureConn) Read(p []byte) (n int, err error) {
	n, err = sc.conn.Read(p)
	if err == nil {
		sc.decoder.XORKeyStream(p[:n], p[:n])
	}
	return
}

func (sc *SecureConn) Write(p []byte) (n int, err error) {
	sc.encoder.XORKeyStream(p, p)
	return sc.conn.Write(p)
}

type Tunnel struct {
	die chan struct{}
	p1  *SecureConn
	p2  *SecureConn
}

func NewTunnel(key string, p1, p2 net.Conn) *Tunnel {
	t := new(Tunnel)
	t.die = make(chan struct{})
	t.p1 = NewSecureConn(key, p1)
	t.p2 = NewSecureConn(key, p2)
	return t
}

// Open establishes bi-directional communcation of p1,p2
func (t *Tunnel) Open() {
	die := make(chan bool, 2)
	go func() {
		n, err := io.Copy(t.p1, t.p2)
		log.Println("from p1 -> p2 written:", n, "error:", err)
		die <- true
	}()

	go func() {
		n, err := io.Copy(t.p2, t.p1)
		log.Println("from p2 -> p1 written:", n, "error:", err)
		die <- true
	}()

	<-die
}

func main() {
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

func handleClient(p1 *net.TCPConn, remote string, key string) {
	log.Println("stream opened")
	defer log.Println("stream closed")

	// p1
	p1.SetNoDelay(false)
	defer p1.Close()

	// p2
	p2, err := kcp.DialEncrypted(kcp.MODE_FAST, remote, key)
	p2.SetWindowSize(128, 1024)
	if err != nil {
		log.Println(err)
	}
	defer p2.Close()

	t := NewTunnel(key, p1, p2)
	t.Open()
}

func checkError(err error) {
	if err != nil {
		log.Println(err)
		os.Exit(-1)
	}
}
