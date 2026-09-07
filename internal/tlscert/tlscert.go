package tlscert

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"strings"
	"time"
)

func Generate(listenHost string) (tls.Certificate, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, err
	}
	now := time.Now()
	tpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			Organization: []string{"Netcatty Center"},
			CommonName:   "Netcatty Center",
		},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}
	addListenHost(tpl, listenHost)
	if host, herr := os.Hostname(); herr == nil {
		addDNS(tpl, host)
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	if len(certPEM) == 0 || len(keyPEM) == 0 {
		return tls.Certificate{}, fmt.Errorf("empty self-signed certificate")
	}
	return tls.X509KeyPair(certPEM, keyPEM)
}

func addListenHost(tpl *x509.Certificate, listenHost string) {
	host := strings.TrimSpace(listenHost)
	if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
		return
	}
	if ip := net.ParseIP(host); ip != nil {
		addIP(tpl, ip)
		return
	}
	addDNS(tpl, host)
}

func addDNS(tpl *x509.Certificate, name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	for _, existing := range tpl.DNSNames {
		if strings.EqualFold(existing, name) {
			return
		}
	}
	tpl.DNSNames = append(tpl.DNSNames, name)
}

func addIP(tpl *x509.Certificate, ip net.IP) {
	if ip == nil {
		return
	}
	for _, existing := range tpl.IPAddresses {
		if existing.Equal(ip) {
			return
		}
	}
	tpl.IPAddresses = append(tpl.IPAddresses, ip)
}
