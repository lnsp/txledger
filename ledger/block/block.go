package block

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"math/bits"
	"runtime"
	"time"

	"github.com/google/btree"
	"github.com/pkg/errors"

	"github.com/lnsp/txledger/ledger/account"
	"github.com/lnsp/txledger/ledger/hash"
	"github.com/lnsp/txledger/ledger/transaction"
)

const (
	BlockEpoch = 16.0
)
const (
	RewardBase        uint64 = 2 << 4
	HashSize                 = 32
	VarianceChunkSize        = 2 << 16
)

type Block struct {
	Chain        uint64
	Index        uint64
	Complexity   uint64
	Timestamp    uint64
	Variance     uint64
	PreviousHash []byte
	Data         []transaction.TX
}

func (b Block) String() string {
	return fmt.Sprintf("Block [chain = %d; index = %d; fingerprint = %s; quality = %d]", b.Chain, b.Index, b.Fingerprint(), HashQuality(b.Complexity))
}

func (b Block) SetBytesFrom(source io.Reader) Block {
	var dataSize, txSize uint64
	binary.Read(source, binary.LittleEndian, &b.Chain)
	binary.Read(source, binary.LittleEndian, &b.Index)
	binary.Read(source, binary.LittleEndian, &b.Complexity)
	binary.Read(source, binary.LittleEndian, &b.Timestamp)
	binary.Read(source, binary.LittleEndian, &b.Variance)
	binary.Read(source, binary.LittleEndian, &dataSize)

	b.Data = make([]transaction.TX, dataSize)
	source.Read(b.PreviousHash)
	for i := range b.Data {
		binary.Read(source, binary.LittleEndian, &txSize)
		txBytes := make([]byte, txSize)
		source.Read(txBytes)
		b.Data[i] = transaction.New().SetBytes(txBytes)
	}
	return b
}

func (b Block) Bytes() []byte {
	buffer := bytes.NewBuffer([]byte{})
	binary.Write(buffer, binary.LittleEndian, b.Chain)
	binary.Write(buffer, binary.LittleEndian, b.Index)
	binary.Write(buffer, binary.LittleEndian, b.Complexity)
	binary.Write(buffer, binary.LittleEndian, b.Timestamp)
	binary.Write(buffer, binary.LittleEndian, b.Variance)
	binary.Write(buffer, binary.LittleEndian, uint64(len(b.Data)))

	buffer.Write(b.PreviousHash)
	for _, tx := range b.Data {
		txBytes := tx.Bytes()
		txSize := uint64(len(txBytes))
		binary.Write(buffer, binary.LittleEndian, txSize)
		buffer.Write(txBytes)
	}
	return buffer.Bytes()
}

func (b Block) SetBytes(data []byte) Block {
	buffer := bytes.NewBuffer(data)
	b.SetBytesFrom(buffer)
	return b
}

func (b Block) Hash() []byte {
	hasher := hash.New()
	binary.Write(hasher, binary.LittleEndian, b.Chain)
	binary.Write(hasher, binary.LittleEndian, b.Index)
	binary.Write(hasher, binary.LittleEndian, b.Complexity)
	binary.Write(hasher, binary.LittleEndian, b.Timestamp)
	binary.Write(hasher, binary.LittleEndian, b.Variance)

	hasher.Write(b.PreviousHash)
	for _, tx := range b.Data {
		hasher.Write(tx.Hash())
	}
	return hasher.Sum()
}

func (b Block) HashString() string {
	return hex.EncodeToString(b.Hash())
}

func (b Block) Fingerprint() string {
	return b.HashString()[:16]
}

func (b Block) Verify(fallback *btree.BTree) (*btree.BTree, error) {
	if !b.Compliant() {
		return fallback, errors.New("Block is not compliant")
	}
	if len(b.Data) < 1 {
		return fallback, errors.New("Block is empty")
	}
	tree := fallback.Clone()
	reward := BlockReward(b.Complexity, b.Data)
	for i, tx := range b.Data {
		if (tx.Type == transaction.TypeCoinbase && i != 0) || (tx.Type != transaction.TypeCoinbase && i == 0) {
			return fallback, errors.New("TX %d does not begin with coinbase")
		}
		if !tx.VerifyFees(reward, b.Complexity) {
			return fallback, errors.Errorf("TX %d does not use valid fees", i)
		}
		if !tx.VerifyProof(tree) {
			return fallback, errors.Errorf("TX %d does not have a valid proof", i)
		}
		if !tx.Apply(tree) {
			return fallback, errors.Errorf("TX %d can not be applied", i)
		}
	}
	return tree, nil
}

// SuccessorOf returns true if this block is the direct successor of the given block.
func (b Block) SuccessorOf(prev Block) error {
	if b.Chain != prev.Chain {
		return errors.New("Chain ID should match")
	}
	if b.Index != prev.Index+1 {
		return errors.New("Index should be larger than of prev block")
	}
	if b.Complexity != prev.Complexity+1 {
		return errors.New("Complexity should be larger than of prev block")
	}
	if b.Timestamp < prev.Timestamp {
		return errors.New("Timestamp should be newer than prev block")
	}
	if !bytes.Equal(b.PreviousHash, prev.Hash()) {
		return errors.New("Prev hash should be equal to hash")
	}
	return nil
}

// Compliant if the block is compliant to the hash quality requirements for this complexity step.
func (b Block) Compliant() bool {
	hash := b.Hash()
	requiredQuality := HashQuality(b.Complexity)
	maxLeadingZeros := uint64(bits.LeadingZeros8(0))
	for i := range hash {
		leadingZeros := uint64(bits.LeadingZeros8(hash[i]))
		if requiredQuality <= leadingZeros {
			return true
		} else if leadingZeros < maxLeadingZeros {
			return false
		}
		requiredQuality -= leadingZeros
	}
	return false
}

func (b Block) Append(tx transaction.TX) Block {
	b.Data = append(b.Data, tx)
	return b
}

func HashQuality(complexity uint64) uint64 {
	return uint64(math.Sqrt(float64(complexity) / BlockEpoch))
}

func BlockReward(complexity uint64, transactions []transaction.TX) uint64 {
	var sum uint64
	if transactions != nil {
		for _, tx := range transactions {
			if tx.Type != transaction.TypeTransfer {
				continue
			}
			sum += tx.Fee
		}
	}
	return sum + HashQuality(complexity)*RewardBase
}

func Genesis(chain, complexity uint64, creator *account.Private) Block {
	data := []transaction.TX{
		transaction.NewCoinbase(chain, creator, BlockReward(complexity, nil)),
	}
	return Block{
		Chain:        chain,
		Index:        0,
		Complexity:   complexity,
		Timestamp:    uint64(time.Now().Unix()),
		Variance:     0,
		PreviousHash: make([]byte, HashSize),
		Data:         data,
	}
}

func Next(prev Block) Block {
	return Block{
		Chain:        prev.Chain,
		Index:        prev.Index + 1,
		Complexity:   prev.Complexity + 1,
		Timestamp:    uint64(time.Now().Unix()),
		Variance:     0,
		PreviousHash: prev.Hash(),
		Data:         []transaction.TX{},
	}
}

func New() Block {
	return Block{
		Chain:        0,
		Index:        0,
		Complexity:   0,
		Timestamp:    0,
		Variance:     0,
		PreviousHash: make([]byte, HashSize),
		Data:         []transaction.TX{},
	}
}

func runVarianceWorker(init Block, chunks <-chan [2]uint64, sols chan<- uint64, quit <-chan bool) {
	for {
		select {
		case <-quit:
			return
		case c := <-chunks:
			for v := c[0]; v < c[1]; v++ {
				init.Variance = v
				if !init.Compliant() {
					continue
				}
				select {
				case sols <- v:
				default:
				}
			}
		}
	}
}

func Find(init Block) Block {
	chunks := make(chan [2]uint64)
	sols := make(chan uint64, 1)
	quit := make(chan bool)
	procs := runtime.NumCPU()
	for i := 0; i < procs; i++ {
		go runVarianceWorker(init, chunks, sols, quit)
	}
	var (
		varianceChunk uint64
		variance      uint64
	)
varianceLoop:
	for {
		select {
		case variance = <-sols:
			break varianceLoop
		case chunks <- [2]uint64{
			varianceChunk,
			varianceChunk + VarianceChunkSize}:
			varianceChunk += VarianceChunkSize
		}
	}
	for i := 0; i < procs; i++ {
		quit <- true
	}
	close(chunks)
	close(sols)
	close(quit)
	init.Variance = variance
	return init
}
