package main

import (
	"errors"
	"fmt"
	"log"
	"mime"
	"os"
	"strings"

	"github.com/ruffrey/smtpd"
)

func main() {
	server := smtpd.NewServer(messageHandler)
	server.MaxSize = 5 * 1024 * 1024

	server.Extend("PROXY", &proxyHandler{})

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Printf("Listening on %s", addr)
	err := server.ListenAndServe(addr)
	log.Fatalf("Server exited %v", err)
}

func messageHandler(msg *smtpd.Message) error {
	to := msg.To[0] // can panic
	log.Printf("New message to: %q from: %q subject: %q body: %s", to, msg.From, msg.Subject, msg.RawBody)

	parts, err := msg.Parts()
	if err != nil {
		return err
	}
	var (
		image, description []byte
	)
	for _, part := range parts {
		mediaType, _, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
		if err != nil {
			return err
		}
		if strings.HasPrefix(mediaType, "text/plain") {
			description = part.Body
		}
		if strings.HasPrefix(mediaType, "image/") {
			image = part.Body
		}
		if description != nil && image != nil {
			break
		}
	}
	if description == nil || image == nil {
		return errors.New("missing description or image")
	}
	log.Printf("description: %s, len(image): %d", description, len(image))
	return nil
}

type proxyHandler struct {
	// FIXME(cjh): How do we know the upstream IPs to trust?
	// TrustIPs []string
}

// Handle implements the expected method for a smtp handler
func (p *proxyHandler) Handle(conn *smtpd.Conn, methodBody string) error {
	remoteIP := strings.Split(conn.RemoteAddr().String(), ":")[0]
	log.Printf("Remote IP: %s", remoteIP)
	// if !sliceContains(p.TrustIPs, remoteIP) {
	// 	return errors.New("PROXY not allowed from '" + remoteIP + "'")
	// }

	phead, err := newProxyHeaderV1(methodBody)
	if err != nil {
		return err
	}

	// isHealthCheck := sliceContains(p.TrustIPs, phead.EndUserIP)
	// if isHealthCheck {
	// 	return nil
	// }

	conn.ForwardedForIP = phead.EndUserIP
	return nil
}

// EHLO also exports expected behavior
func (p *proxyHandler) EHLO() string {
	return "PROXY"
}

func newProxyHeaderV1(methodBody string) (*ProxyHeaderV1, error) {
	// methodBody: "TCP4 209.85.214.42 45.76.28.175 33372 25"
	//				0	 1			   2			3     4
	// 					 src	  	   dest         src   dest
	methodBodyParts := strings.Split(methodBody, " ")
	if len(methodBodyParts) < 5 {
		return nil, fmt.Errorf("PROXY v1 format is invalid, %s", methodBody)
	}
	return &ProxyHeaderV1{
		ProtoName:   methodBodyParts[0],
		EndUserIP:   methodBodyParts[1],
		ProxyIP:     methodBodyParts[2],
		EndUserPort: methodBodyParts[3],
		ProxyPort:   methodBodyParts[4],
	}, nil
}

type ProxyHeaderV1 struct {
	ProtoName   string
	EndUserIP   string
	EndUserPort string
	ProxyIP     string
	ProxyPort   string
}
