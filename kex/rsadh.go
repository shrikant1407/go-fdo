// SPDX-FileCopyrightText: (C) 2024 Intel Corporation
// SPDX-License-Identifier: Apache 2.0

package kex

import (
	"encoding"
	"fmt"
	"io"
	"log"
	"math/big"
	"strings"

	"github.com/fido-device-onboard/go-fdo/cbor"
)

var prime14, prime15 *big.Int

func init() {
	var ok bool
	prime14, ok = new(big.Int).SetString("FFFFFFFFFFFFFFFFC90FDAA22168C234C4C6628B80DC1CD129024E088A67CC74020BBEA63B139B22514A08798E3404DDEF9519B3CD3A431B302B0A6DF25F14374FE1356D6D51C245E485B576625E7EC6F44C42E9A637ED6B0BFF5CB6F406B7EDEE386BFB5A899FA5AE9F24117C4B1FE649286651ECE45B3DC2007CB8A163BF0598DA48361C55D39A69163FA8FD24CF5F83655D23DCA3AD961C62F356208552BB9ED529077096966D670C354E4ABC9804F1746C08CA18217C32905E462E36CE3BE39E772C180E86039B2783A2EC07A28FB5C55DF06F4C52C9DE2BCBF6955817183995497CEA956AE515D2261898FA051015728E5A8AACAA68FFFFFFFFFFFFFFFF", 16)
	if !ok {
		panic("invalid DH group 14 prime")
	}
	prime15, ok = new(big.Int).SetString("FFFFFFFFFFFFFFFFC90FDAA22168C234C4C6628B80DC1CD129024E088A67CC74020BBEA63B139B22514A08798E3404DDEF9519B3CD3A431B302B0A6DF25F14374FE1356D6D51C245E485B576625E7EC6F44C42E9A637ED6B0BFF5CB6F406B7EDEE386BFB5A899FA5AE9F24117C4B1FE649286651ECE45B3DC2007CB8A163BF0598DA48361C55D39A69163FA8FD24CF5F83655D23DCA3AD961C62F356208552BB9ED529077096966D670C354E4ABC9804F1746C08CA18217C32905E462E36CE3BE39E772C180E86039B2783A2EC07A28FB5C55DF06F4C52C9DE2BCBF6955817183995497CEA956AE515D2261898FA051015728E5A8AAAC42DAD33170D04507A33A85521ABDF1CBA64ECFB850458DBEF0A8AEA71575D060C7DB3970F85A6E1E4C7ABF5AE8CDB0933D71E8C94E04A25619DCEE3D2261AD2EE6BF12FFA06D98A0864D87602733EC86A64521F2B18177B200CBBE117577A615D6C770988C0BAD946E208E24FA074E5AB3143DB5BFCE0FD108E4B82D120A93AD2CAFFFFFFFFFFFFFFFF", 16)
	if !ok {
		panic("invalid DH group 15 prime")
	}
}

func init() {
	RegisterKeyExchangeSuite(
		string(DHKEXid14Suite),
		func(xA []byte, cipher CipherSuiteID) Session {
			var paramA *big.Int
			if xA != nil {
				paramA = new(big.Int).SetBytes(xA)
			}
			return &RSADHSession{
				p:         prime14,
				g:         2,
				paramSize: 32,

				xA: paramA,

				SessionCrypter: SessionCrypter{
					ID:     cipher,
					Cipher: cipher.Suite(),
					SEK:    []byte{},
					SVK:    []byte{},
				},
			}
		},
	)
	RegisterKeyExchangeSuite(
		string(DHKEXid15Suite),
		func(xA []byte, cipher CipherSuiteID) Session {
			var paramA *big.Int
			if xA != nil {
				paramA = new(big.Int).SetBytes(xA)
			}
			return &RSADHSession{
				p:         prime15,
				g:         2,
				paramSize: 96,

				xA: paramA,

				SessionCrypter: SessionCrypter{
					ID:     cipher,
					Cipher: cipher.Suite(),
					SEK:    []byte{},
					SVK:    []byte{},
				},
			}
		},
	)
}

// RSADHSession implements a Session using RSA keys from Diffie-Hellman key
// exchange. Sessions are created using [Suite.New].
type RSADHSession struct {
	// Static configuration
	p         *big.Int
	g         int
	paramSize int

	// Key exchange data
	a, xA *big.Int
	b, xB *big.Int

	// Session encrypt/decrypt data
	SessionCrypter
}

func (s RSADHSession) String() string {
	return fmt.Sprintf(`RSADH[
  p     %x
  g     %d
  size  %d
  a     %x
  xA    %x
  b     %x
  xB    %x
  %s
]`,
		bigIntBytes(s.p),
		s.g,
		s.paramSize,
		bigIntBytes(s.a),
		bigIntBytes(s.xA),
		bigIntBytes(s.b),
		bigIntBytes(s.xB),
		strings.ReplaceAll(s.SessionCrypter.String(), "\n", "\n  "),
	)
}

func bigIntBytes(i *big.Int) []byte {
	if i == nil {
		return nil
	}
	return i.Bytes()
}

// Parameter generates the exchange parameter to send to its peer. This
// function will generate a new parameter every time it is called. This
// method is used by both the client and server.
func (s *RSADHSession) Parameter(rand io.Reader) ([]byte, error) {
	// Create a random parameter x and compute exchange parameter with the
	// formula g^x mod p
	r := make([]byte, s.paramSize)
	if _, err := rand.Read(r); err != nil {
		return nil, err
	}
	g := new(big.Int).SetInt64(int64(s.g))
	x := new(big.Int).SetBytes(r)
	xX := new(big.Int).Exp(g, x, s.p)

	// Store the private (non-exchange) parameter and return the exchange
	// parameter
	if s.xA == nil {
		s.a = x
		return xX.Bytes(), nil
	}
	s.b = x

	// Compute session key
	sek, svk, err := rsaSymmetricKey(s.xA, s.b, s.p, s.Cipher)
	if err != nil {
		return nil, fmt.Errorf("error computing symmetric keys: %w", err)
	}
	s.SEK, s.SVK = sek, svk

	return xX.Bytes(), nil
}

// SetParameter sets the received parameter from the client. This method is
// only called by a server.
func (s *RSADHSession) SetParameter(xB []byte) error {
	s.xB = new(big.Int).SetBytes(xB)

	// Compute session key
	log.Printf("%#v", s)
	sek, svk, err := rsaSymmetricKey(s.xB, s.a, s.p, s.Cipher)
	if err != nil {
		return fmt.Errorf("error computing symmetric keys: %w", err)
	}
	s.SEK, s.SVK = sek, svk

	return nil
}

func rsaSymmetricKey(other, own, p *big.Int, cipher CipherSuite) (sek, svk []byte, err error) {
	// Compute shared secret
	shSe := new(big.Int).Exp(other, own, p).Bytes()

	// Derive a symmetric key
	sekSize, svkSize := cipher.EncryptAlg.KeySize(), uint16(0)
	if cipher.MacAlg != 0 {
		svkSize = cipher.MacAlg.KeySize()
	}
	symKey, err := kdf(cipher.PRFHash, shSe, []byte{}, (sekSize+svkSize)*8)
	if err != nil {
		return nil, nil, fmt.Errorf("kdf: %w", err)
	}

	return symKey[:sekSize], symKey[sekSize:], nil
}

type rsadhPersist struct {
	Prime     []byte
	Generator int

	ParamSize int
	ParamA    []byte
	ParamXA   []byte
	ParamB    []byte
	ParamXB   []byte

	Cipher CipherSuiteID
	SEK    []byte
	SVK    []byte
}

// MarshalCBOR implements [cbor.Marshaler].
func (s *RSADHSession) MarshalCBOR() ([]byte, error) {
	persist := rsadhPersist{
		Prime:     s.p.Bytes(),
		Generator: s.g,
		ParamSize: s.paramSize,

		Cipher: s.ID,
		SEK:    s.SEK,
		SVK:    s.SVK,
	}
	if s.a != nil {
		persist.ParamA = s.a.Bytes()
	}
	if s.xA != nil {
		persist.ParamXA = s.xA.Bytes()
	}
	if s.b != nil {
		persist.ParamB = s.b.Bytes()
	}
	if s.xB != nil {
		persist.ParamXB = s.xB.Bytes()
	}
	return cbor.Marshal(persist)
}

// UnmarshalCBOR implements [cbor.Unmarshaler].
func (s *RSADHSession) UnmarshalCBOR(data []byte) error {
	var persist rsadhPersist
	if err := cbor.Unmarshal(data, &persist); err != nil {
		return err
	}

	*s = RSADHSession{
		p:         new(big.Int).SetBytes(persist.Prime),
		g:         persist.Generator,
		paramSize: persist.ParamSize,

		SessionCrypter: SessionCrypter{
			ID:     persist.Cipher,
			Cipher: persist.Cipher.Suite(),
			SEK:    persist.SEK,
			SVK:    persist.SVK,
		},
	}
	if len(persist.ParamA) > 0 {
		s.a = new(big.Int).SetBytes(persist.ParamA)
	}
	if len(persist.ParamXA) > 0 {
		s.xA = new(big.Int).SetBytes(persist.ParamXA)
	}
	if len(persist.ParamB) > 0 {
		s.b = new(big.Int).SetBytes(persist.ParamB)
	}
	if len(persist.ParamXB) > 0 {
		s.xB = new(big.Int).SetBytes(persist.ParamXB)
	}

	return nil
}

var _ encoding.BinaryMarshaler = (*RSADHSession)(nil)
var _ encoding.BinaryUnmarshaler = (*RSADHSession)(nil)

// MarshalBinary implements encoding.BinaryMarshaler
func (s *RSADHSession) MarshalBinary() ([]byte, error) { return s.MarshalCBOR() }

// UnmarshalBinary implements encoding.BinaryUnmarshaler
func (s *RSADHSession) UnmarshalBinary(data []byte) error { return s.UnmarshalCBOR(data) }
