package blockchaincomponent

// RFC 9381 ECVRF-P256-SHA256-TAI implementation.
//
// The implementation intentionally exposes only the fixed ciphersuite used by
// PoDL consensus. Inputs are public consensus seeds, so the variable-time
// try-and-increment encoding permitted by RFC 9381 is appropriate here.

import (
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
)

const (
	ECVRFP256SHA256TAI = "ECVRF-P256-SHA256-TAI"
	ecvrfP256SuiteByte = byte(0x01)
	ecvrfP256ProofLen  = 33 + 16 + 32
)

type p256Point struct {
	x *big.Int
	y *big.Int
}

func fixedWidthInt(v *big.Int, size int) []byte {
	out := make([]byte, size)
	if v == nil {
		return out
	}
	raw := v.Bytes()
	if len(raw) > size {
		raw = raw[len(raw)-size:]
	}
	copy(out[size-len(raw):], raw)
	return out
}

func parseP256Secret(secret []byte) (*big.Int, error) {
	if len(secret) != 32 {
		return nil, fmt.Errorf("P-256 VRF secret must be exactly 32 bytes")
	}
	x := new(big.Int).SetBytes(secret)
	n := elliptic.P256().Params().N
	if x.Sign() <= 0 || x.Cmp(n) >= 0 {
		return nil, fmt.Errorf("P-256 VRF secret is outside the scalar field")
	}
	return x, nil
}

func P256VRFPublicKey(secret []byte) ([]byte, error) {
	x, err := parseP256Secret(secret)
	if err != nil {
		return nil, err
	}
	px, py := elliptic.P256().ScalarBaseMult(fixedWidthInt(x, 32))
	return elliptic.MarshalCompressed(elliptic.P256(), px, py), nil
}

func parseP256Point(encoded []byte) (p256Point, error) {
	if len(encoded) != 33 {
		return p256Point{}, fmt.Errorf("compressed P-256 point must be 33 bytes")
	}
	x, y := elliptic.UnmarshalCompressed(elliptic.P256(), encoded)
	if x == nil || y == nil || !elliptic.P256().IsOnCurve(x, y) {
		return p256Point{}, fmt.Errorf("invalid compressed P-256 point")
	}
	return p256Point{x: x, y: y}, nil
}

func encodeP256Point(p p256Point) []byte {
	return elliptic.MarshalCompressed(elliptic.P256(), p.x, p.y)
}

func p256ScalarMult(p p256Point, scalar *big.Int) p256Point {
	x, y := elliptic.P256().ScalarMult(p.x, p.y, fixedWidthInt(scalar, 32))
	return p256Point{x: x, y: y}
}

func p256ScalarBaseMult(scalar *big.Int) p256Point {
	x, y := elliptic.P256().ScalarBaseMult(fixedWidthInt(scalar, 32))
	return p256Point{x: x, y: y}
}

func p256Subtract(a, b p256Point) (p256Point, error) {
	if a.x == nil || a.y == nil || b.x == nil || b.y == nil {
		return p256Point{}, fmt.Errorf("cannot subtract point at infinity")
	}
	p := elliptic.P256().Params().P
	negY := new(big.Int).Neg(b.y)
	negY.Mod(negY, p)
	x, y := elliptic.P256().Add(a.x, a.y, b.x, negY)
	if x == nil || y == nil {
		return p256Point{}, fmt.Errorf("ECVRF point subtraction produced infinity")
	}
	return p256Point{x: x, y: y}, nil
}

func ecvrfP256EncodeToCurve(publicKey, alpha []byte) (p256Point, error) {
	if _, err := parseP256Point(publicKey); err != nil {
		return p256Point{}, err
	}
	for counter := 0; counter <= 255; counter++ {
		material := make([]byte, 0, 2+len(publicKey)+len(alpha)+2)
		material = append(material, ecvrfP256SuiteByte, 0x01)
		material = append(material, publicKey...)
		material = append(material, alpha...)
		material = append(material, byte(counter), 0x00)
		hash := sha256.Sum256(material)
		candidate := append([]byte{0x02}, hash[:]...)
		if point, err := parseP256Point(candidate); err == nil {
			return point, nil
		}
	}
	return p256Point{}, fmt.Errorf("ECVRF encode-to-curve exhausted counter")
}

// ecvrfP256Nonce implements the RFC 6979 process selected by RFC 9381 for
// ECVRF-P256-SHA256-TAI. hString is already a 33-byte compressed curve point.
func ecvrfP256Nonce(secret *big.Int, hString []byte) *big.Int {
	q := elliptic.P256().Params().N
	// bits2octets from RFC 6979 section 2.3.4. P-256 has qlen=256.
	h1 := sha256.Sum256(hString)
	hInt := new(big.Int).SetBytes(h1[:])
	hInt.Mod(hInt, q)
	bx := append(fixedWidthInt(secret, 32), fixedWidthInt(hInt, 32)...)
	v := make([]byte, sha256.Size)
	for i := range v {
		v[i] = 0x01
	}
	k := make([]byte, sha256.Size)
	mac := hmac.New(sha256.New, k)
	_, _ = mac.Write(v)
	_, _ = mac.Write([]byte{0x00})
	_, _ = mac.Write(bx)
	k = mac.Sum(nil)
	mac = hmac.New(sha256.New, k)
	_, _ = mac.Write(v)
	v = mac.Sum(nil)
	mac = hmac.New(sha256.New, k)
	_, _ = mac.Write(v)
	_, _ = mac.Write([]byte{0x01})
	_, _ = mac.Write(bx)
	k = mac.Sum(nil)
	mac = hmac.New(sha256.New, k)
	_, _ = mac.Write(v)
	v = mac.Sum(nil)
	for {
		mac = hmac.New(sha256.New, k)
		_, _ = mac.Write(v)
		v = mac.Sum(nil)
		candidate := new(big.Int).SetBytes(v)
		if candidate.Sign() > 0 && candidate.Cmp(q) < 0 {
			return candidate
		}
		mac = hmac.New(sha256.New, k)
		_, _ = mac.Write(v)
		_, _ = mac.Write([]byte{0x00})
		k = mac.Sum(nil)
		mac = hmac.New(sha256.New, k)
		_, _ = mac.Write(v)
		v = mac.Sum(nil)
	}
}

func ecvrfP256Challenge(points ...p256Point) *big.Int {
	material := []byte{ecvrfP256SuiteByte, 0x02}
	for _, point := range points {
		material = append(material, encodeP256Point(point)...)
	}
	material = append(material, 0x00)
	digest := sha256.Sum256(material)
	return new(big.Int).SetBytes(digest[:16])
}

func ECVRFP256Prove(secret, alpha []byte) (proof []byte, output []byte, publicKey []byte, err error) {
	x, err := parseP256Secret(secret)
	if err != nil {
		return nil, nil, nil, err
	}
	publicKey, err = P256VRFPublicKey(secret)
	if err != nil {
		return nil, nil, nil, err
	}
	y, _ := parseP256Point(publicKey)
	h, err := ecvrfP256EncodeToCurve(publicKey, alpha)
	if err != nil {
		return nil, nil, nil, err
	}
	gamma := p256ScalarMult(h, x)
	nonce := ecvrfP256Nonce(x, encodeP256Point(h))
	u := p256ScalarBaseMult(nonce)
	v := p256ScalarMult(h, nonce)
	c := ecvrfP256Challenge(y, h, gamma, u, v)
	s := new(big.Int).Mul(c, x)
	s.Add(s, nonce)
	s.Mod(s, elliptic.P256().Params().N)
	proof = append(proof, encodeP256Point(gamma)...)
	proof = append(proof, fixedWidthInt(c, 16)...)
	proof = append(proof, fixedWidthInt(s, 32)...)
	output, err = ECVRFP256ProofToHash(proof)
	if err != nil {
		return nil, nil, nil, err
	}
	return proof, output, publicKey, nil
}

func decodeECVRFP256Proof(proof []byte) (p256Point, *big.Int, *big.Int, error) {
	if len(proof) != ecvrfP256ProofLen {
		return p256Point{}, nil, nil, fmt.Errorf("ECVRF P-256 proof must be %d bytes", ecvrfP256ProofLen)
	}
	gamma, err := parseP256Point(proof[:33])
	if err != nil {
		return p256Point{}, nil, nil, err
	}
	c := new(big.Int).SetBytes(proof[33:49])
	s := new(big.Int).SetBytes(proof[49:])
	if s.Cmp(elliptic.P256().Params().N) >= 0 {
		return p256Point{}, nil, nil, fmt.Errorf("ECVRF response scalar is outside the field")
	}
	return gamma, c, s, nil
}

func ECVRFP256ProofToHash(proof []byte) ([]byte, error) {
	gamma, _, _, err := decodeECVRFP256Proof(proof)
	if err != nil {
		return nil, err
	}
	material := []byte{ecvrfP256SuiteByte, 0x03}
	material = append(material, encodeP256Point(gamma)...)
	material = append(material, 0x00)
	digest := sha256.Sum256(material)
	return digest[:], nil
}

func ECVRFP256Verify(publicKey, alpha, proof []byte) ([]byte, bool) {
	y, err := parseP256Point(publicKey)
	if err != nil {
		return nil, false
	}
	gamma, c, s, err := decodeECVRFP256Proof(proof)
	if err != nil {
		return nil, false
	}
	h, err := ecvrfP256EncodeToCurve(publicKey, alpha)
	if err != nil {
		return nil, false
	}
	u, err := p256Subtract(p256ScalarBaseMult(s), p256ScalarMult(y, c))
	if err != nil {
		return nil, false
	}
	v, err := p256Subtract(p256ScalarMult(h, s), p256ScalarMult(gamma, c))
	if err != nil {
		return nil, false
	}
	if ecvrfP256Challenge(y, h, gamma, u, v).Cmp(c) != 0 {
		return nil, false
	}
	output, err := ECVRFP256ProofToHash(proof)
	return output, err == nil
}

func decodeFixedHex(value string, size int) ([]byte, error) {
	raw, err := hex.DecodeString(strings.TrimPrefix(strings.TrimSpace(value), "0x"))
	if err != nil || len(raw) != size {
		return nil, fmt.Errorf("expected %d-byte hex value", size)
	}
	return raw, nil
}
