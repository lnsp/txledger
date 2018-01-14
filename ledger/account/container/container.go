package container

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"

	"github.com/lnsp/txledger/ledger/account"
	"github.com/lnsp/txledger/ledger/hash"
	"github.com/pkg/errors"
)

// Container is a serializable wrapper for encrypted private keys.
type Container struct {
	PublicKey           string `json:"public"`
	EncryptedPrivateKey string `json:"private"`
}

// ReadFromFile decodes an account container from file.
func ReadFromFile(path string) (Container, error) {
	file, err := os.Open(path)
	if err != nil {
		return Container{}, errors.Wrap(err, "Could not create container")
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	container := Container{}
	if err := decoder.Decode(&container); err != nil {
		return container, errors.Wrap(err, "Could not decode file")
	}
	return container, nil
}

// WriteToFile encodes an account container to a file.
func WriteToFile(c Container, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return errors.Wrap(err, "Could not create file")
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	if err := encoder.Encode(c); err != nil {
		return errors.Wrap(err, "Could not encode container")
	}
	return nil
}

// Unlock decrypts the contained private key and returns the account.
func (c Container) Unlock(passphrase []byte) (*account.Private, error) {
	hasher := hash.New()
	hasher.Write(passphrase)
	hashedPassphrase := hasher.Sum()
	ciph, err := aes.NewCipher(hashedPassphrase)
	if err != nil {
		return nil, errors.Wrap(err, "Could not create ciphersuite")
	}
	gcm, err := cipher.NewGCM(ciph)
	if err != nil {
		return nil, errors.Wrap(err, "Could not create GCM")
	}
	ciphertext, err := hex.DecodeString(c.EncryptedPrivateKey)
	if err != nil {
		return nil, errors.Wrap(err, "Invalid encrypted private key format")
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("Encrypted private key too short")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	bytes, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, errors.New("Could not unseal container")
	}
	acc := account.NewPrivateFromBytes(bytes)
	return acc, nil
}

// New creates a new container with the given passphrase and private key.
func New(passphrase []byte, acc *account.Private) (Container, error) {
	hasher := hash.New()
	hasher.Write([]byte(passphrase))
	hashedPassphrase := hasher.Sum()
	ciph, err := aes.NewCipher(hashedPassphrase)
	if err != nil {
		return Container{}, errors.Wrap(err, "Could not create ciphersuite")
	}
	gcm, err := cipher.NewGCM(ciph)
	if err != nil {
		return Container{}, errors.Wrap(err, "Could not create GCM")
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return Container{}, errors.Wrap(err, "Could not generate nonce")
	}
	bytes := gcm.Seal(nonce, nonce, acc.Bytes(), nil)
	return Container{
		PublicKey:           hex.EncodeToString(acc.PublicKeyBytes()),
		EncryptedPrivateKey: hex.EncodeToString(bytes),
	}, nil
}
