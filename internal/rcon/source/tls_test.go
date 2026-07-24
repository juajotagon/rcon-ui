package source

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/juajotagon/rcon-ui/internal/rcon"
)

// RCON sends its password in cleartext in the first packet, so a server exposed
// beyond a trusted network is normally fronted by a TLS-terminating proxy.
// These tests cover that transport. The dialect inside the tunnel is unchanged,
// which is the property worth pinning: the same client code must work whether or
// not TLS is in play.

// newTestCert issues a self-signed certificate for the given name.
func newTestCert(tb testing.TB, name string) (tls.Certificate, *x509.CertPool) {
	tb.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		tb.Fatalf("generate key: %v", err)
	}

	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:              []string{name},
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		tb.Fatalf("create certificate: %v", err)
	}

	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		tb.Fatalf("parse certificate: %v", err)
	}

	pool := x509.NewCertPool()
	pool.AddCert(leaf)

	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}, pool
}

// startTLSProxy fronts a plain RCON server with TLS, exactly as a real
// TLS-terminating proxy does: it decrypts and forwards the same bytes.
func startTLSProxy(tb testing.TB, backend string, cert tls.Certificate) string {
	tb.Helper()

	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}})
	if err != nil {
		tb.Fatalf("tls listen: %v", err)
	}
	tb.Cleanup(func() { ln.Close() })

	go func() {
		for {
			client, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer client.Close()

				upstream, err := net.Dial("tcp", backend)
				if err != nil {
					return
				}
				defer upstream.Close()

				done := make(chan struct{}, 2)
				pipe := func(dst, src net.Conn) {
					buf := make([]byte, 4096)
					for {
						n, err := src.Read(buf)
						if n > 0 {
							if _, werr := dst.Write(buf[:n]); werr != nil {
								break
							}
						}
						if err != nil {
							break
						}
					}
					done <- struct{}{}
				}
				go pipe(upstream, client)
				go pipe(client, upstream)
				<-done
			}()
		}
	}()

	return ln.Addr().String()
}

func TestDialOverTLS(t *testing.T) {
	const name = "rcon.test"

	backend := newFakeServer(t, fakeOpts{password: testPassword})
	cert, pool := newTestCert(t, name)
	proxyAddr := startTLSProxy(t, backend.addr(), cert)

	// Trust the test CA for the duration of this test. Production verifies
	// against the system pool; overriding it here is what lets a self-signed
	// certificate stand in for a real proxy.
	t.Cleanup(swapRootCAs(pool))

	c, err := Dial(t.Context(), rcon.Target{
		Addr:       proxyAddr,
		Password:   testPassword,
		TLS:        true,
		ServerName: name,
	})
	if err != nil {
		t.Fatalf("dial over TLS: %v", err)
	}
	defer c.Close()

	got, err := c.Execute(t.Context(), "list")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if want := "ok: list"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// Long replies must reassemble identically through TLS, since record
// boundaries do not align with RCON packet boundaries.
func TestTLSMultiPacketResponse(t *testing.T) {
	const name = "rcon.test"
	want := longBody(3)

	backend := newFakeServer(t, fakeOpts{
		password:       testPassword,
		doubleSentinel: true,
		handler:        func(string) string { return want },
	})
	cert, pool := newTestCert(t, name)
	proxyAddr := startTLSProxy(t, backend.addr(), cert)
	t.Cleanup(swapRootCAs(pool))

	c, err := Dial(t.Context(), rcon.Target{
		Addr: proxyAddr, Password: testPassword, TLS: true, ServerName: name,
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()

	got, err := c.Execute(t.Context(), "help")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got != want {
		t.Errorf("response truncated through TLS: got %d bytes, want %d", len(got), len(want))
	}
}

// A wrong server name must fail. Skipping verification would leave the password
// exposed to anyone able to intercept the connection, which is the entire thing
// TLS was added to prevent.
func TestTLSRejectsWrongServerName(t *testing.T) {
	backend := newFakeServer(t, fakeOpts{password: testPassword})
	cert, pool := newTestCert(t, "rcon.test")
	proxyAddr := startTLSProxy(t, backend.addr(), cert)
	t.Cleanup(swapRootCAs(pool))

	_, err := Dial(t.Context(), rcon.Target{
		Addr: proxyAddr, Password: testPassword, TLS: true, ServerName: "wrong.test",
	})
	if err == nil {
		t.Fatal("expected a certificate verification failure")
	}
	if !strings.Contains(err.Error(), "TLS handshake") {
		t.Errorf("error should identify the handshake as the failure: %v", err)
	}
	// The password must not appear in an error that may be logged.
	if strings.Contains(err.Error(), testPassword) {
		t.Error("error message leaked the password")
	}
}

// Plain RCON against a TLS listener must fail cleanly rather than hanging: a
// misconfigured profile is a common mistake and should say so quickly.
func TestPlainDialAgainstTLSListenerFails(t *testing.T) {
	backend := newFakeServer(t, fakeOpts{password: testPassword})
	cert, _ := newTestCert(t, "rcon.test")
	proxyAddr := startTLSProxy(t, backend.addr(), cert)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	_, err := Dial(ctx, rcon.Target{
		Addr: proxyAddr, Password: testPassword, Timeout: 2 * time.Second,
	})
	if err == nil {
		t.Fatal("expected plain RCON against a TLS listener to fail")
	}
}

// swapRootCAs installs a trust store for the duration of a test and returns a
// function restoring the previous one.
func swapRootCAs(pool *x509.CertPool) func() {
	previous := rootCAs
	rootCAs = pool
	return func() { rootCAs = previous }
}
