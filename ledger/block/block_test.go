package block

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"github.com/lnsp/txledger/ledger/account"
)

func TestBlock(t *testing.T) {
	p := account.NewPrivate()
	g := Genesis(0, 0, p)
	g2 := New().SetBytes(g.Bytes())

	if !reflect.DeepEqual(g, g2) {
		fmt.Println("Bytes not reversible")
	}

	buf := bytes.NewBuffer(g.Bytes())
	g2 = New().SetBytesFrom(buf)
	if !reflect.DeepEqual(g, g2) {
		fmt.Println("BytesFrom not reversible")
	}
}
