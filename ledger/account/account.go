package account

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"math/big"

	"github.com/google/btree"

	"github.com/lnsp/txledger/ledger/hash"
)

// PrivateKeyCurve is the elliptic curve in use for private key creation.
var PrivateKeyCurve = elliptic.P256()

// NewPublic instantiates a new public account (key) from the given byte slice.
func NewPublic(key []byte) *Public {
	X := new(big.Int).SetBytes(key[:32])
	Y := new(big.Int).SetBytes(key[32:])
	return &Public{&ecdsa.PublicKey{
		Curve: PrivateKeyCurve,
		X:     X,
		Y:     Y,
	}}
}

// NewPrivate generates a new private-public key pair bound to an account.
func NewPrivate() *Private {
	key, err := ecdsa.GenerateKey(PrivateKeyCurve, rand.Reader)
	if err != nil {
		panic(err)
	}
	return &Private{key}
}

// NewPrivateFromBytes restores the private key from a slice of bytes.
func NewPrivateFromBytes(key []byte) *Private {
	X := new(big.Int).SetBytes(key[:32])
	Y := new(big.Int).SetBytes(key[32:64])
	D := new(big.Int).SetBytes(key[64:])
	return &Private{&ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{
			Curve: PrivateKeyCurve,
			X:     X,
			Y:     Y,
		},
		D: D,
	}}
}

// Account is a generic interface to an account that can send and receive funding.
type Account interface {
	PublicKeyBytes() []byte
	Address() []byte
	Verify(data, signature []byte) bool
}

// Public is a public account. Public accounts can only verify transactions.
type Public struct {
	key *ecdsa.PublicKey
}

// PublicKeyBytes retrieves the public key in a binary format.
func (a *Public) PublicKeyBytes() []byte {
	buffer := bytes.NewBuffer([]byte{})
	buffer.Write(a.key.X.Bytes())
	buffer.Write(a.key.Y.Bytes())
	return buffer.Bytes()
}

// Address gets the accounts verifiable address.
func (a *Public) Address() []byte {
	hasher := hash.New()
	hasher.Write(a.PublicKeyBytes())
	return hasher.Sum()
}

// Verify checks the validity of the signature on the given hash.
func (a *Public) Verify(hash, signature []byte) bool {
	r := new(big.Int).SetBytes(signature[:32])
	s := new(big.Int).SetBytes(signature[32:])
	return ecdsa.Verify(a.key, hash, r, s)
}

// String generates a human-readable address.
func (a *Public) String() string {
	return "0x" + hex.EncodeToString(a.Address())
}

// Private is a private account. A private account can verify and sign transactions.
type Private struct {
	key *ecdsa.PrivateKey
}

// Bytes generates a byte-representation of the private key.
func (a *Private) Bytes() []byte {
	buffer := bytes.NewBuffer([]byte{})
	buffer.Write(a.PublicKeyBytes())
	buffer.Write(a.key.D.Bytes())
	return buffer.Bytes()
}

// PublicKeyBytes retrieves the private keys public pair in a binary format.
func (a *Private) PublicKeyBytes() []byte {
	buffer := bytes.NewBuffer([]byte{})
	buffer.Write(a.key.PublicKey.X.Bytes())
	buffer.Write(a.key.PublicKey.Y.Bytes())
	return buffer.Bytes()
}

// Address returns the hashed public-key address.
func (a *Private) Address() []byte {
	hasher := hash.New()
	hasher.Write(a.PublicKeyBytes())
	return hasher.Sum()
}

// String generates a human-readable address for this private key.
func (a *Private) String() string {
	return "0x" + hex.EncodeToString(a.Address())
}

// Sign generates a signature for the given hash.
func (a *Private) Sign(hash []byte) []byte {
	r, s, err := ecdsa.Sign(rand.Reader, a.key, hash)
	if err != nil {
		panic(err)
	}
	buffer := bytes.NewBuffer([]byte{})
	buffer.Write(r.Bytes())
	buffer.Write(s.Bytes())
	return buffer.Bytes()
}

// Verify checks the validity of the signature on the hash.
func (a *Private) Verify(hash, signature []byte) bool {
	r := new(big.Int).SetBytes(signature[:32])
	s := new(big.Int).SetBytes(signature[32:])
	return ecdsa.Verify(&a.key.PublicKey, hash, r, s)
}

type AddressTreeItem struct {
	Address []byte
	Account Account
	Funds   uint64
}

func (item AddressTreeItem) Less(than btree.Item) bool {
	return bytes.Compare(item.Address, than.(AddressTreeItem).Address) < 0
}

func NewAddressTree() *btree.BTree {
	return btree.New(2)
}
