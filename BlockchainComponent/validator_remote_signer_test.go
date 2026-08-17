package blockchaincomponent

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type signerTestPKI struct {
	caFile, serverCert, serverKey, clientCert, clientKey string
}

func writeSignerTestPEM(t *testing.T, path, kind string, der []byte) {
	t.Helper()
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: kind, Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
}

func createSignerTestPKI(t *testing.T) signerTestPKI {
	t.Helper()
	dir := t.TempDir()
	now := time.Now()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTemplate := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "PoDL signer test CA"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}

	issue := func(serial int64, commonName string, usages []x509.ExtKeyUsage, dns []string, ips []net.IP) ([]byte, *ecdsa.PrivateKey) {
		key, keyErr := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if keyErr != nil {
			t.Fatal(keyErr)
		}
		template := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: commonName}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: usages, DNSNames: dns, IPAddresses: ips}
		certificate, certErr := x509.CreateCertificate(rand.Reader, template, caTemplate, &key.PublicKey, caKey)
		if certErr != nil {
			t.Fatal(certErr)
		}
		return certificate, key
	}
	serverDER, serverKey := issue(2, "localhost", []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, []string{"localhost"}, []net.IP{net.ParseIP("127.0.0.1")})
	clientDER, clientKey := issue(3, "podl-chain-node", []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, nil, nil)

	pki := signerTestPKI{caFile: filepath.Join(dir, "ca.pem"), serverCert: filepath.Join(dir, "server.pem"), serverKey: filepath.Join(dir, "server-key.pem"), clientCert: filepath.Join(dir, "client.pem"), clientKey: filepath.Join(dir, "client-key.pem")}
	writeSignerTestPEM(t, pki.caFile, "CERTIFICATE", caDER)
	writeSignerTestPEM(t, pki.serverCert, "CERTIFICATE", serverDER)
	serverPrivate, err := x509.MarshalECPrivateKey(serverKey)
	if err != nil {
		t.Fatal(err)
	}
	writeSignerTestPEM(t, pki.serverKey, "EC PRIVATE KEY", serverPrivate)
	writeSignerTestPEM(t, pki.clientCert, "CERTIFICATE", clientDER)
	clientPrivate, err := x509.MarshalECPrivateKey(clientKey)
	if err != nil {
		t.Fatal(err)
	}
	writeSignerTestPEM(t, pki.clientKey, "EC PRIVATE KEY", clientPrivate)
	return pki
}

func TestRemoteValidatorSignerMTLSEndToEnd(t *testing.T) {
	pki := createSignerTestPKI(t)
	local, _ := newTestValidatorSigner(t, filepath.Join(t.TempDir(), "signer-slashing.json"))
	serverTLS, err := ValidatorSignerServerTLSConfig(ValidatorSignerTLSFiles{CertificateFile: pki.serverCert, KeyFile: pki.serverKey, ClientCAFile: pki.caFile})
	if err != nil {
		t.Fatal(err)
	}
	if serverTLS.MinVersion != tls.VersionTLS13 || serverTLS.ClientAuth != tls.RequireAndVerifyClientCert {
		t.Fatal("signer server did not enforce TLS 1.3 and verified client certificates")
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("sandbox does not permit the loopback mTLS integration listener: %v", err)
	}
	server := httptest.NewUnstartedServer(NewValidatorSignerHandler(local, true))
	server.Listener = listener
	server.TLS = serverTLS
	server.StartTLS()
	defer server.Close()

	remote, err := NewRemoteValidatorSigner(context.Background(), RemoteValidatorSignerConfig{URL: server.URL, CAFile: pki.caFile, ClientCertificateFile: pki.clientCert, ClientKeyFile: pki.clientKey, ServerName: "localhost", Timeout: 3 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer remote.Close()
	if remote.Address() != local.Address() || !remote.Status(context.Background()).Healthy {
		t.Fatal("remote signer identity/status mismatch")
	}

	vote := ConsensusVote{Height: 300, Round: 2, Step: StepPrevote, BlockHash: "0xremote"}
	if err := SignConsensusVoteWithSigner(context.Background(), &vote, remote); err != nil {
		t.Fatal(err)
	}
	if !VerifyConsensusVote(vote) {
		t.Fatal("mTLS remote signature did not verify")
	}
	conflict := ConsensusVote{Height: 300, Round: 2, Step: StepPrevote, BlockHash: "0xconflict"}
	if err := SignConsensusVoteWithSigner(context.Background(), &conflict, remote); err == nil {
		t.Fatal("remote slashing protection accepted a conflicting vote")
	}
	alpha := []byte("mTLS RFC 9381 proof")
	result, err := remote.ProveVRF(context.Background(), alpha, "301/0")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyValidatorVRFResult(remote.Address(), alpha, result) {
		t.Fatal("mTLS remote VRF proof did not verify locally")
	}

	caPEM, err := os.ReadFile(pki.caFile)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		t.Fatal("test CA parse failed")
	}
	unauthenticated := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: roots, ServerName: "localhost"}}, Timeout: time.Second}
	if _, err := unauthenticated.Get(server.URL + "/v1/status"); err == nil {
		t.Fatal("signer server accepted a TLS client without a certificate")
	}
}

func TestRemoteSignerRejectsInsecureNonLoopback(t *testing.T) {
	if _, err := NewRemoteValidatorSigner(context.Background(), RemoteValidatorSignerConfig{URL: "http://192.0.2.10:9100", AllowInsecureLoopback: true}); err == nil {
		t.Fatal("non-loopback plaintext remote signer was accepted")
	}
}
