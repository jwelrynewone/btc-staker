package staker

import (
	"fmt"
	"sync"

	"github.com/btcsuite/btcd/wire"
)

type TxState uint8

const (
	Send TxState = iota
	Confirmed
)

type TrackedTransaction struct {
	tx *wire.MsgTx
	// We need to track script also, as it is needed for slashing tx buidling
	txscript []byte

	state TxState
}

// TODO Add version with db!
// Safe
type StakingTxTracker struct {
	mutex        sync.RWMutex
	transactions map[string]*TrackedTransaction
}

func NewStakingTxTracker() *StakingTxTracker {
	return &StakingTxTracker{
		transactions: make(map[string]*TrackedTransaction),
	}
}

func (t *StakingTxTracker) Add(tx *wire.MsgTx, txscript []byte) error {
	txHash := tx.TxHash().String()

	t.mutex.Lock()
	defer t.mutex.Unlock()
	_, ok := t.transactions[txHash]

	if ok {
		return fmt.Errorf("tx with hash %s already added", txHash)
	}

	t.transactions[txHash] = &TrackedTransaction{
		tx:       tx,
		txscript: txscript,
		state:    Send,
	}

	return nil
}

func (t *StakingTxTracker) SetState(txHash string, state TxState) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	entry, ok := t.transactions[txHash]

	if !ok {
		return fmt.Errorf("tx with hash %s not found", txHash)
	}

	entry.state = state

	return nil
}

// returns nil only if tx is not found
func (t *StakingTxTracker) Get(txHash string) *TrackedTransaction {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	entry, ok := t.transactions[txHash]

	if !ok {
		return nil
	}

	return entry
}

func (t *StakingTxTracker) Remove(txHash string) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	delete(t.transactions, txHash)
}


func (t *StakingTxTracker) GetAll() []*TrackedTransaction {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	transactions := make([]*TrackedTransaction, 0, len(t.transactions))

	for _, tx := range t.transactions {
		transactions = append(transactions, tx)
	}

	return transactions
}