package ledger

import (
	"encoding/binary"
	"io"

	"github.com/google/btree"
	"github.com/lnsp/txledger/ledger/account"
	"github.com/lnsp/txledger/ledger/block"
	"github.com/pkg/errors"
)

type Ledger struct {
	Chain     uint64
	Blocks    []block.Block
	Addresses *btree.BTree
}

func New(chain uint64) *Ledger {
	return &Ledger{
		Chain:     chain,
		Blocks:    []block.Block{},
		Addresses: btree.New(2),
	}
}

func (l *Ledger) Size() uint64 {
	return uint64(len(l.Blocks))
}

func (l *Ledger) Last() block.Block {
	size := l.Size()
	if size < 1 {
		panic(errors.New("Ledger is empty"))
	}
	return l.Blocks[size-1]
}

func (l *Ledger) Append(b block.Block) error {
	if l.Size() > 0 {
		if err := b.SuccessorOf(l.Last()); err != nil {
			return errors.Wrap(err, "Block not successor")
		}
	}
	addresses, err := b.Verify(l.Addresses)
	if err != nil {
		return errors.Wrap(err, "Block can not be verified")
	}
	l.Addresses = addresses
	l.Blocks = append(l.Blocks, b)
	return nil
}

func (l *Ledger) Init(complexity uint64, creator *account.Private) error {
	l.Blocks = []block.Block{}
	genesis := block.Genesis(l.Chain, complexity, creator)
	return l.Append(block.Find(genesis))
}

func (l *Ledger) ReadFrom(r io.Reader) error {
	var size uint64
	l.Addresses = account.NewAddressTree()
	binary.Read(r, binary.LittleEndian, &l.Chain)
	binary.Read(r, binary.LittleEndian, &size)
	l.Blocks = make([]block.Block, 0)
	for i := uint64(0); i < size; i++ {
		b := block.New().SetBytesFrom(r)
		err := l.Append(b)
		if err != nil {
			return errors.Wrapf(err, "Could not read block %d", i)
		}
	}
	return nil
}

func (l *Ledger) WriteTo(w io.Writer) {
	size := uint64(len(l.Blocks))
	binary.Write(w, binary.LittleEndian, &l.Chain)
	binary.Write(w, binary.LittleEndian, size)
	for i := range l.Blocks {
		w.Write(l.Blocks[i].Bytes())
	}
}
