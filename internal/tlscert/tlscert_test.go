package tlscert

import (
	"testing"
)

func TestGenerateInMemoryCertificate(t *testing.T) {
	cert, err := Generate("10.0.0.8")
	if err != nil {
		t.Fatal(err)
	}
	if len(cert.Certificate) == 0 {
		t.Fatal("expected certificate bytes")
	}
	if cert.PrivateKey == nil {
		t.Fatal("expected private key")
	}
	again, err := Generate("10.0.0.8")
	if err != nil {
		t.Fatal(err)
	}
	if string(cert.Certificate[0]) == string(again.Certificate[0]) {
		t.Fatal("in-memory certificates should be newly generated each time")
	}
}
