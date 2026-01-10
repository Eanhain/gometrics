package signature

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net/http"
	"os"
)

func GetRSAKey(rsa string) (*rsa.PrivateKey, error) {
	rsaKey, err := os.ReadFile(rsa)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(rsaKey)
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	return key, nil

}

func GetRSAPubKey(rsa string) (*rsa.PublicKey, error) {
	rsaCert, err := os.ReadFile(rsa)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(rsaCert)
	key, err := x509.ParsePKCS1PublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	return key, nil

}

func EncryptByRSA(payload []byte, rsaCert *rsa.PublicKey) ([]byte, error) {
	rng := rand.Reader
	return rsa.EncryptOAEP(sha256.New(), rng, rsaCert, payload, nil)
}

func DecryptByKey(payload []byte, rsaKey *rsa.PrivateKey) ([]byte, error) {
	return rsa.DecryptOAEP(sha256.New(), nil, rsaKey, payload, nil)
}

func DecryptRSAHandler(keyPath string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			pKey, err := GetRSAKey(keyPath)
			if err != nil || keyPath == "" || r.Method == "GET" {
				next.ServeHTTP(w, r)
				return
			}

			byteBody, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			decBody, err := DecryptByKey(byteBody, pKey)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(decBody))
			next.ServeHTTP(w, r)
		})
	}
}
