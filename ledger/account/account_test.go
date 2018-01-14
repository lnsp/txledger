package account

import (
	"reflect"
	"testing"
)

func TestPublicPrivate(t *testing.T) {
	acc := NewPrivate()
	pub := acc.PublicKeyBytes()
	acc2 := NewPublic(pub)
	pub2 := acc2.PublicKeyBytes()
	priv := acc.Bytes()
	acc3 := NewPrivateFromBytes(priv)

	if !reflect.DeepEqual(pub, pub2) {
		t.Error("Public.PublicKeyBytes should match Private.PublicKeyBytes")
	}
	if !reflect.DeepEqual(acc.Address(), acc2.Address()) {
		t.Error("Public.Address should match Private.Address")
	}

	if !reflect.DeepEqual(acc.Bytes(), acc3.Bytes()) {
		t.Error("Private.Bytes should match serialized copy")
	}
	if !reflect.DeepEqual(acc.PublicKeyBytes(), acc3.PublicKeyBytes()) {
		t.Error("Private.PublicKeyBytes should match serialized copy")
	}

	data := []byte("example")
	sign := acc.Sign(data)
	if !acc.Verify(data, sign) {
		t.Error("Private key cannot verify own signature")
	}
	if !acc2.Verify(data, sign) {
		t.Error("Public key cannot verify own signature")
	}
}
