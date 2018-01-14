package transaction

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"time"

	"github.com/google/btree"

	"github.com/lnsp/txledger/ledger/account"
	"github.com/lnsp/txledger/ledger/hash"
)

const (
	// BaseFee is the minimum pee paid for each transaction
	BaseFee = 2 << 8
	// FeeSizeScalar that scales up with transaction size
	FeeSizeScalar = 2 << 4
	// FeeComplexityScalar that scales up with block complexity
	FeeComplexityScalar = 2 << 6
	// AddressSize is the amount of bytes reserved for an account address
	AddressSize = 32
	// KeyPairSize is the amount of bytes reserved for a key pair proof
	KeyPairSize = 64
	// FeeEpoch is the block epoch size
	FeeEpoch = 64.0
)

// CalculateFee calculates the fees required for a block of the given size and complexity.
func CalculateFee(size, complexity uint64) uint64 {
	return BaseFee + FeeSizeScalar*size + FeeComplexityScalar*uint64(math.Sqrt(float64(complexity)/FeeEpoch))
}

const (
	// TypeCoinbase announces a valid block on the network
	TypeCoinbase uint64 = iota
	// TypeAccount announces the account on the network
	TypeAccount
	// TypeTransfer transfers a specified amount of value from the sender to the receiver
	TypeTransfer
)

// TX is the transaction storage structure
type TX struct {
	Chain             uint64
	Type              uint64
	Sender, Recipient []byte
	Amount, Fee       uint64
	Timestamp         uint64
	Proof             []byte
	Data              []byte
}

func (tx TX) String() string {
	switch tx.Type {
	case TypeCoinbase:
		return fmt.Sprintf("TX Coinbase [miner = %s; reward = %d]", hex.EncodeToString(tx.Recipient), tx.Amount)
	case TypeAccount:
		return fmt.Sprintf("TX Account [address = %s]", hex.EncodeToString(tx.Sender))
	case TypeTransfer:
		return fmt.Sprintf("TX Transfer [from = %s; to = %s; amount = %d; fee = %d]", hex.EncodeToString(tx.Sender), hex.EncodeToString(tx.Recipient), tx.Amount, tx.Fee)
	}
	return "TX Unknown"
}

// VerifyProof checks that the transaction has been created with the permission of the sender.
func (tx TX) VerifyProof(addresses *btree.BTree) bool {
	switch tx.Type {
	case TypeCoinbase:
		pub := account.NewPublic(tx.Data)
		if !bytes.Equal(pub.Address(), tx.Recipient) {
			return false
		}
		if !pub.Verify(tx.PartialHash(), tx.Proof) {
			return false
		}
		return true
	case TypeAccount:
		pub := account.NewPublic(tx.Data)
		if !bytes.Equal(pub.Address(), tx.Sender) {
			return false
		}
		if !pub.Verify(tx.PartialHash(), tx.Proof) {
			return false
		}
		return true
	case TypeTransfer:
		item := addresses.Get(account.AddressTreeItem{
			Address: tx.Sender,
		})
		if item == nil {
			return false
		}
		acc := item.(account.Account)
		if !bytes.Equal(acc.Address(), tx.Sender) {
			return false
		}
		if !acc.Verify(tx.PartialHash(), tx.Proof) {
			return false
		}
		return true
	}
	return false
}

// VerifyFees checks if the fee requirements have been satisfied.
func (tx TX) VerifyFees(reward, complexity uint64) bool {
	switch tx.Type {
	case TypeCoinbase:
		return tx.Amount <= reward
	case TypeAccount:
		return true
	case TypeTransfer:
		return tx.Fee >= CalculateFee(uint64(len(tx.Data)), complexity)
	}
	return false
}

// Apply applies the transaction to the address database.
func (tx TX) Apply(addresses *btree.BTree) bool {
	var (
		item     btree.Item
		addrItem account.AddressTreeItem
	)
	switch tx.Type {
	case TypeCoinbase:
		if item = addresses.Get(account.AddressTreeItem{
			Address: tx.Recipient,
		}); item != nil {
			addrItem = item.(account.AddressTreeItem)
		} else {
			addr := account.NewPublic(tx.Data)
			addrItem = account.AddressTreeItem{
				Address: addr.Address(),
				Account: addr,
				Funds:   0,
			}
		}
		addrItem.Funds += tx.Amount
	case TypeAccount:
		if item = addresses.Get(account.AddressTreeItem{
			Address: tx.Sender,
		}); item != nil {
			addrItem = item.(account.AddressTreeItem)
		} else {
			addr := account.NewPublic(tx.Data)
			addrItem = account.AddressTreeItem{
				Address: addr.Address(),
				Account: addr,
				Funds:   0,
			}
		}
		if !bytes.Equal(tx.Sender, addrItem.Account.Address()) {
			return false
		}
		if !bytes.Equal(tx.Data, addrItem.Account.PublicKeyBytes()) {
			return false
		}
	case TypeTransfer:
		var (
			recipientItem     btree.Item
			recipientAddrItem account.AddressTreeItem
		)
		if item = addresses.Get(account.AddressTreeItem{
			Address: tx.Sender,
		}); item != nil {
			addrItem = item.(account.AddressTreeItem)
		} else {
			return false
		}
		if recipientItem = addresses.Get(account.AddressTreeItem{
			Address: tx.Recipient,
		}); recipientItem != nil {
			recipientAddrItem = item.(account.AddressTreeItem)
		} else {
			return false
		}
		if tx.Fee+tx.Amount < tx.Amount {
			return false
		}
		if addrItem.Funds < tx.Fee+tx.Amount {
			return false
		}
		addrItem.Funds -= tx.Fee + tx.Amount
		if recipientAddrItem.Funds+tx.Amount < recipientAddrItem.Funds {
			return false
		}
		recipientAddrItem.Funds += tx.Amount
		addresses.ReplaceOrInsert(recipientAddrItem)
	}
	addresses.ReplaceOrInsert(addrItem)
	return true
}

// Bytes serializes the transaction to a binary format.
func (tx TX) Bytes() []byte {
	buffer := bytes.NewBuffer([]byte{})
	binary.Write(buffer, binary.LittleEndian, tx.Chain)
	binary.Write(buffer, binary.LittleEndian, tx.Type)
	binary.Write(buffer, binary.LittleEndian, tx.Amount)
	binary.Write(buffer, binary.LittleEndian, tx.Fee)
	binary.Write(buffer, binary.LittleEndian, tx.Timestamp)

	buffer.Write(tx.Sender)
	buffer.Write(tx.Recipient)
	buffer.Write(tx.Proof)
	buffer.Write(tx.Data)
	return buffer.Bytes()
}

// SetBytes retrieves the transaction from the given binary data.
func (tx TX) SetBytes(b []byte) TX {
	buffer := bytes.NewBuffer(b)
	binary.Read(buffer, binary.LittleEndian, &tx.Chain)
	binary.Read(buffer, binary.LittleEndian, &tx.Type)
	binary.Read(buffer, binary.LittleEndian, &tx.Amount)
	binary.Read(buffer, binary.LittleEndian, &tx.Fee)
	binary.Read(buffer, binary.LittleEndian, &tx.Timestamp)

	buffer.Read(tx.Sender)
	buffer.Read(tx.Recipient)
	buffer.Read(tx.Proof)
	tx.Data = buffer.Bytes()
	return tx
}

// PartialHash generates a hash excluding the proof data.
func (tx TX) PartialHash() []byte {
	hasher := hash.New()
	binary.Write(hasher, binary.LittleEndian, tx.Chain)
	binary.Write(hasher, binary.LittleEndian, tx.Type)
	binary.Write(hasher, binary.LittleEndian, tx.Amount)
	binary.Write(hasher, binary.LittleEndian, tx.Fee)
	binary.Write(hasher, binary.LittleEndian, tx.Timestamp)

	hasher.Write(tx.Sender)
	hasher.Write(tx.Recipient)
	hasher.Write(tx.Data)
	return hasher.Sum()
}

// Hash generates a transaction hash from the partial hash and the proof data.
func (tx TX) Hash() []byte {
	hasher := hash.New()
	hasher.Write(tx.PartialHash())
	hasher.Write(tx.Proof)
	return hasher.Sum()
}

// New creates a new empty transaction.
func New() TX {
	return TX{
		Chain:     0,
		Type:      0,
		Amount:    0,
		Fee:       0,
		Timestamp: 0,
		Sender:    make([]byte, AddressSize),
		Recipient: make([]byte, AddressSize),
		Proof:     make([]byte, KeyPairSize),
		Data:      []byte{},
	}
}

// NewCoinbase creates a new coinbase on the given chain and miner.
func NewCoinbase(chain uint64, priv *account.Private, amount uint64) TX {
	tx := TX{
		Chain:     chain,
		Type:      TypeCoinbase,
		Amount:    amount,
		Fee:       0,
		Timestamp: uint64(time.Now().Unix()),
		Sender:    make([]byte, AddressSize),
		Recipient: priv.Address(),
		Data:      priv.PublicKeyBytes(),
	}
	tx.Proof = priv.Sign(tx.PartialHash())
	return tx
}

// NewAccount announces a new account on the given chain.
func NewAccount(chain uint64, priv *account.Private) TX {
	tx := TX{
		Chain:     chain,
		Type:      TypeAccount,
		Amount:    0,
		Fee:       0,
		Timestamp: uint64(time.Now().Unix()),
		Sender:    priv.Address(),
		Recipient: make([]byte, AddressSize),
		Data:      priv.PublicKeyBytes(),
	}
	tx.Proof = priv.Sign(tx.PartialHash())
	return tx
}

// NewTransfer creates a new transfer of the given amount of value.
func NewTransfer(chain, amount, fee uint64, from *account.Private, to account.Account) TX {
	tx := TX{
		Chain:     chain,
		Type:      TypeTransfer,
		Amount:    amount,
		Fee:       fee,
		Timestamp: uint64(time.Now().Unix()),
		Sender:    from.Address(),
		Recipient: to.Address(),
		Data:      []byte{},
	}
	tx.Proof = from.Sign(tx.PartialHash())
	return tx
}
