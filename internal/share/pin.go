package share

import (
	"crypto/rand"
	"fmt"
	"regexp"
)

var pinPattern = regexp.MustCompile(`^\d{6}$`)

func GeneratePIN() (string, error) {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	n := int(buf[0])<<16 | int(buf[1])<<8 | int(buf[2])
	return fmt.Sprintf("%06d", n%1000000), nil
}

func ValidPIN(pin string) bool {
	return pinPattern.MatchString(pin)
}
