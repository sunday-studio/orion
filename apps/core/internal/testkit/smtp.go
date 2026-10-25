package testkit

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"fmt"
	"math/big"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// SMTPMessage is one accepted mail.
type SMTPMessage struct {
	From string
	To   string
	Data string
	Auth string // decoded AUTH PLAIN credentials, "" when none
}

// SMTPServer is a minimal in-process SMTP server for alert mail tests.
type SMTPServer struct {
	Host string
	Port int
	// TLSConfig is the client config that trusts the server's self-signed certificate.
	TLSConfig *tls.Config

	mu       sync.Mutex
	messages []SMTPMessage
	rejects  map[string]bool
	mode     string
	cert     tls.Certificate
	ln       net.Listener
}

// NewSMTPServer starts a server. mode is "none" (plaintext), "starttls" or "tls" (implicit).
func NewSMTPServer(t testing.TB, mode string) *SMTPServer {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "127.0.0.1"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, _ := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	leaf, _ := x509.ParseCertificate(der)
	pool := x509.NewCertPool()
	pool.AddCert(leaf)
	s := &SMTPServer{Host: "127.0.0.1", mode: mode, rejects: map[string]bool{},
		cert:      tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key},
		TLSConfig: &tls.Config{RootCAs: pool, ServerName: "127.0.0.1", MinVersion: tls.VersionTLS12}}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	if mode == "tls" {
		ln = tls.NewListener(ln, &tls.Config{Certificates: []tls.Certificate{s.cert}})
	}
	s.ln = ln
	s.Port = ln.Addr().(*net.TCPAddr).Port
	go s.serve()
	t.Cleanup(func() { _ = ln.Close() })
	return s
}

// RejectRecipient makes RCPT TO for the address fail with a 550.
func (s *SMTPServer) RejectRecipient(address string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rejects[strings.ToLower(address)] = true
}

// ClearRejects accepts every recipient again.
func (s *SMTPServer) ClearRejects() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rejects = map[string]bool{}
}

// Messages returns a copy of the accepted mail.
func (s *SMTPServer) Messages() []SMTPMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]SMTPMessage(nil), s.messages...)
}

func (s *SMTPServer) serve() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

func (s *SMTPServer) handle(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	say := func(format string, args ...any) { fmt.Fprintf(conn, format+"\r\n", args...) }
	say("220 fake ESMTP")
	var msg SMTPMessage
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimSpace(line)
		upper := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(upper, "EHLO"):
			say("250-fake")
			if s.mode == "starttls" {
				if _, isTLS := conn.(*tls.Conn); !isTLS {
					say("250-STARTTLS")
				}
			}
			say("250 AUTH PLAIN")
		case upper == "STARTTLS":
			say("220 go ahead")
			tlsConn := tls.Server(conn, &tls.Config{Certificates: []tls.Certificate{s.cert}})
			if tlsConn.Handshake() != nil {
				return
			}
			conn = tlsConn
			reader = bufio.NewReader(conn)
			say = func(format string, args ...any) { fmt.Fprintf(conn, format+"\r\n", args...) }
		case strings.HasPrefix(upper, "AUTH PLAIN"):
			raw, _ := base64.StdEncoding.DecodeString(strings.TrimSpace(line[len("AUTH PLAIN"):]))
			msg.Auth = strings.ReplaceAll(string(raw), "\x00", "|")
			say("235 ok")
		case strings.HasPrefix(upper, "MAIL FROM:"):
			msg.From = angle(line)
			say("250 ok")
		case strings.HasPrefix(upper, "RCPT TO:"):
			to := angle(line)
			s.mu.Lock()
			rejected := s.rejects[strings.ToLower(to)]
			s.mu.Unlock()
			if rejected {
				say("550 mailbox unavailable")
				continue
			}
			msg.To = to
			say("250 ok")
		case upper == "DATA":
			say("354 end with .")
			var data strings.Builder
			for {
				l, err := reader.ReadString('\n')
				if err != nil {
					return
				}
				if l == ".\r\n" {
					break
				}
				data.WriteString(l)
			}
			msg.Data = data.String()
			s.mu.Lock()
			s.messages = append(s.messages, msg)
			s.mu.Unlock()
			msg = SMTPMessage{Auth: msg.Auth}
			say("250 queued")
		case upper == "QUIT":
			say("221 bye")
			return
		default:
			say("250 ok")
		}
	}
}

func angle(line string) string {
	start, end := strings.Index(line, "<"), strings.Index(line, ">")
	if start < 0 || end < start {
		return ""
	}
	return line[start+1 : end]
}
