package proxy

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/elazarl/goproxy"
)

// CertManager handles CA certificate generation and loading
type CertManager struct {
	CertPath string
	KeyPath  string
}

func NewCertManager(dataDir string) *CertManager {
	return &CertManager{
		CertPath: filepath.Join(dataDir, "goproxy-ca.pem"),
		KeyPath:  filepath.Join(dataDir, "goproxy-ca.key"),
	}
}

// EnsureCert checks if CA cert exists, otherwise generates it
// EnsureCert checks if CA cert exists, otherwise generates it
func (m *CertManager) EnsureCert() error {
	certStat, certErr := os.Stat(m.CertPath)
	keyStat, keyErr := os.Stat(m.KeyPath)

	if os.IsNotExist(certErr) || os.IsNotExist(keyErr) {
		fmt.Println("Certificate or Key missing, generating new CA...")
		return m.GenerateCert()
	}

	if certStat.Size() == 0 || keyStat.Size() == 0 {
		fmt.Println("Certificate or Key empty, regenerating CA...")
		return m.GenerateCert()
	}

	fmt.Println("Loading existing CA from:", m.CertPath)
	return nil
}

// LoadToGoproxy loads the CA cert into goproxy/tlsConfig
func (m *CertManager) LoadToGoproxy() error {
	certBytes, err := os.ReadFile(m.CertPath)
	if err != nil {
		return err
	}
	keyBytes, err := os.ReadFile(m.KeyPath)
	if err != nil {
		return err
	}

	goproxyCert, err := tls.X509KeyPair(certBytes, keyBytes)
	if err != nil {
		return err
	}

	// Check if leaf cert generation logic is needed, but GoproxyCa is enough for CA
	if goproxyCert.Leaf == nil {
		leaf, err := x509.ParseCertificate(goproxyCert.Certificate[0])
		if err == nil {
			goproxyCert.Leaf = leaf
		}
	}

	goproxy.GoproxyCa = goproxyCert
	goproxy.OkConnect = &goproxy.ConnectAction{Action: goproxy.ConnectAccept, TLSConfig: goproxy.TLSConfigFromCA(&goproxyCert)}
	goproxy.MitmConnect = &goproxy.ConnectAction{Action: goproxy.ConnectMitm, TLSConfig: goproxy.TLSConfigFromCA(&goproxyCert)}
	goproxy.HTTPMitmConnect = &goproxy.ConnectAction{Action: goproxy.ConnectHTTPMitm, TLSConfig: goproxy.TLSConfigFromCA(&goproxyCert)}
	goproxy.RejectConnect = &goproxy.ConnectAction{Action: goproxy.ConnectReject, TLSConfig: goproxy.TLSConfigFromCA(&goproxyCert)}

	return nil
}

func (m *CertManager) GenerateCert() error {
	// Generate key
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	// Generate cert template
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Gaze Proxy CA"},
			CommonName:   "Gaze Proxy CA",
		},
		NotBefore:             time.Now().Add(-24 * time.Hour),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour), // 10 years
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return err
	}

	// Save cert
	certFile, err := os.Create(m.CertPath)
	if err != nil {
		return err
	}
	defer certFile.Close()
	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return err
	}

	// Save key
	keyFile, err := os.Create(m.KeyPath)
	if err != nil {
		return err
	}
	defer keyFile.Close()
	if err := pem.Encode(keyFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}); err != nil {
		return err
	}

	fmt.Printf("CA Certificate generated at: %s\n", m.CertPath)
	return nil
}
