package keyman

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"net"
	"os"
	"testing"
	"time"

	"github.com/getlantern/testify/assert"
)

const (
	PK_FILE           = "testpk.pem"
	PK_FILE_ENCRYPTED = "testpk_encrypted.pem"
	CERT_FILE         = "testcert.pem"
	PASSWD            = "mypassword"

	ONE_WEEK  = 7 * 24 * time.Hour
	TWO_WEEKS = ONE_WEEK * 2
)

func TestRoundTrip(t *testing.T) {
	defer os.Remove(PK_FILE)
	defer os.Remove(CERT_FILE)

	pk, err := GeneratePK(1024)
	assert.NoError(t, err, "Unable to generate PK")

	err = pk.WriteToFile(PK_FILE)
	assert.NoError(t, err, "Unable to save PK")

	pk2, err := LoadPKFromFile(PK_FILE)
	assert.NoError(t, err, "Unable to load PK")
	assert.Equal(t, pk.PEMEncoded(), pk2.PEMEncoded(), "Loaded PK didn't match saved PK")

	err = pk.WriteToFileEncrypted(PK_FILE_ENCRYPTED, []byte(PASSWD), x509.PEMCipherAES256)
	assert.NoError(t, err, "Unable to save PK")

	_, err = LoadPKFromFileEncrypted(PK_FILE_ENCRYPTED, []byte("wrongpassword"))
	assert.Error(t, err, "Loading encrypted PK using wrong password should fail")
	pk4, err := LoadPKFromFileEncrypted(PK_FILE_ENCRYPTED, []byte(PASSWD))
	assert.NoError(t, err, "Unable to load encrypted PK")
	assert.Equal(t, pk.PEMEncoded(), pk4.PEMEncoded(), "Loaded encrypted PK didn't match saved PK")

	cert, err := pk.TLSCertificateFor("Test Org", "127.0.0.1", time.Now().Add(TWO_WEEKS), true, nil)
	assert.NoError(t, err, "Unable to generate self-signed certificate")

	numberOfIPSANs := len(cert.X509().IPAddresses)
	if numberOfIPSANs != 1 {
		t.Errorf("Wrong number of SANs, expected 1 got %d", numberOfIPSANs)
	} else {
		ip := cert.X509().IPAddresses[0]
		expectedIP := net.ParseIP("127.0.0.1")
		assert.Equal(t, expectedIP.String(), ip.String(), "Wrong IP SAN")
	}

	err = cert.WriteToFile(CERT_FILE)
	assert.NoError(t, err, "Unable to write certificate to file")

	cert2, err := LoadCertificateFromFile(CERT_FILE)
	assert.NoError(t, err, "Unable to load certificate from file")
	assert.Equal(t, cert.PEMEncoded(), cert2.PEMEncoded(), "Loaded certificate didn't match saved certificate")

	_, err = pk.Certificate(cert.X509(), cert)
	assert.NoError(t, err, "Unable to generate certificate signed by original certificate")

	pk3, err := GeneratePK(1024)
	assert.NoError(t, err, "Unable to generate PK 3")

	_, err = pk.CertificateForKey(cert.X509(), cert, &pk3.rsaKey.PublicKey)
	assert.NoError(t, err, "Unable to generate certificate for pk3")

	x509rt, err := LoadCertificateFromX509(cert.X509())
	assert.NoError(t, err, "Unable to load certificate from X509")
	assert.Equal(t, cert, x509rt, "X509 round tripped cert didn't match original")

	template := &x509.CertificateRequest{
		Subject: pkix.Name{
			Organization: []string{"Test Org"},
			CommonName:   "test.org",
		},
	}
	csr, err := pk2.CSR(template)
	if assert.NoError(t, err, "Unable to create CSR") {
		validUntil := time.Now().AddDate(2, 1, 1)
		cert3, err := pk.CertificateForCSR(csr, cert, validUntil)
		if assert.NoError(t, err, "Unable to create certificate for CSR") {
			assert.Equal(t, template.Subject.Organization, cert3.X509().Subject.Organization)
			assert.Equal(t, template.Subject.CommonName, cert3.X509().Subject.CommonName)
		}
	}
}
