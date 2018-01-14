package transaction

import (
	"reflect"
	"testing"

	"github.com/lnsp/txledger/ledger/account"
)

func TestTransaction(t *testing.T) {
	p := account.NewPrivate()
	gen := NewCoinbase(12, p, 100)
	gen2 := New().SetBytes(gen.Bytes())
	if !reflect.DeepEqual(gen, gen2) {
		t.Error("TX.Bytes not inversible")
	}
}
