package agent

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

type CertManager struct {
	certDir string
}

func NewCertManager(certDir string) *CertManager {
	return &CertManager{certDir: certDir}
}

func (cm *CertManager) EnsureCA() error {
	caCertPath := filepath.Join(cm.certDir, "ca.crt")
	caKeyPath := filepath.Join(cm.certDir, "ca.key")

	if _, err := os.Stat(caCertPath); err == nil {
		return nil
	}

	if err := os.MkdirAll(cm.certDir, 0755); err != nil {
		return fmt.Errorf("create cert dir: %w", err)
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate ca key: %w", err)
	}

	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "DeviceGrid CA", Organization: []string{"DeviceGrid"}},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(10 * 365 * 24 * time.Hour),
		IsCA:         true,
		KeyUsage:     x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return fmt.Errorf("create ca cert: %w", err)
	}

	if err := writePEM(caCertPath, "CERTIFICATE", certDER); err != nil {
		return err
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return fmt.Errorf("marshal ca key: %w", err)
	}
	if err := writePEM(caKeyPath, "EC PRIVATE KEY", keyDER); err != nil {
		return err
	}

	return nil
}

func (cm *CertManager) EnsureServerCert(host string) error {
	return cm.ensureCert("server", []string{host})
}

func (cm *CertManager) LoadServerTLSConfig() (*tls.Config, error) {
	return cm.LoadTLSConfig("server")
}

func (cm *CertManager) LoadTLSConfig(prefix string) (*tls.Config, error) {
	certPath := filepath.Join(cm.certDir, prefix+".crt")
	keyPath := filepath.Join(cm.certDir, prefix+".key")
	caPath := filepath.Join(cm.certDir, "ca.crt")

	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("load %s cert: %w", prefix, err)
	}

	caPEM, err := os.ReadFile(caPath)
	if err != nil {
		return nil, fmt.Errorf("read ca: %w", err)
	}
	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("parse ca cert")
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caPool,
		MinVersion:   tls.VersionTLS13,
	}, nil
}

func (cm *CertManager) ensureCert(prefix string, dnsNames []string) error {
	certPath := filepath.Join(cm.certDir, prefix+".crt")
	keyPath := filepath.Join(cm.certDir, prefix+".key")

	if _, err := os.Stat(certPath); err == nil {
		return nil
	}

	caCertPEM, err := os.ReadFile(filepath.Join(cm.certDir, "ca.crt"))
	if err != nil {
		return fmt.Errorf("read ca cert: %w", err)
	}
	caBlock, _ := pem.Decode(caCertPEM)
	caCert, err := x509.ParseCertificate(caBlock.Bytes)
	if err != nil {
		return fmt.Errorf("parse ca cert: %w", err)
	}

	caKeyPEM, err := os.ReadFile(filepath.Join(cm.certDir, "ca.key"))
	if err != nil {
		return fmt.Errorf("read ca key: %w", err)
	}
	caKeyBlock, _ := pem.Decode(caKeyPEM)
	caKey, err := x509.ParseECPrivateKey(caKeyBlock.Bytes)
	if err != nil {
		return fmt.Errorf("parse ca key: %w", err)
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate key: %w", err)
	}

	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "DeviceGrid " + prefix, Organization: []string{"DeviceGrid"}},
		DNSNames:     dnsNames,
		IPAddresses:  nil,
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
	if err != nil {
		return fmt.Errorf("create cert: %w", err)
	}
	if err := writePEM(certPath, "CERTIFICATE", certDER); err != nil {
		return err
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return fmt.Errorf("marshal key: %w", err)
	}
	if err := writePEM(keyPath, "EC PRIVATE KEY", keyDER); err != nil {
		return err
	}

	return nil
}

func writePEM(path, blockType string, der []byte) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()
	return pem.Encode(f, &pem.Block{Type: blockType, Bytes: der})
}
