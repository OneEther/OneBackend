package main

import "testing"
import "fmt"
import "math/big"

/*
import "math/big"

import "fmt"
import "encoding/json"
*/

func TestGetBlockNumber(t *testing.T) {
	/*
	   block := getBlockByNumber(big.NewInt(1))

	   b, _ := json.Marshal(&block)
	   fmt.Printf("BLOCK: " + string(b) + "\n")
	*/
}

type MockGeth struct {
    blockNumber           int64
	transactionCount      int64
	transactionsConfirmed bool
}

func (self *MockGeth) SendTransaction(from, to, value, nonce *big.Int) (*Transaction, error) {
    if nonce == nil {
        cb, _ := self.GetCoinbase()
        nonce, _ = self.GetTransactionCount(cb)
    }

	txn := &Transaction{
        Hash:        getHexString(nonce, 40),
		From:        getHexString(from, 40),
		To:          getHexString(to, 40),
		Value:       getHexString(value, 0),
		Nonce:       getHexString(nonce, 0),
		BlockNumber: fmt.Sprintf("%d", self.blockNumber)}

	self.transactionCount++

	return txn, nil
}

func (self *MockGeth) GetTransactionCount(*big.Int) (*big.Int, error) {
	return big.NewInt(self.transactionCount+1), nil
}

func (*MockGeth) GetCoinbase() (*big.Int, error) {
	cb := big.NewInt(0x12345)
	return cb, nil
}

func (*MockGeth) GetBalance() (*big.Int, error) {
	bal := big.NewInt(88888)
	return bal, nil
}

func (*MockGeth) GetBalanceFromCoinbase(coinbase *big.Int) (*big.Int, error) {
	bal := big.NewInt(84848)
	return bal, nil
}

func (*MockGeth) GetBlockByNumber(num *big.Int, full bool) *Block {
	block := &Block{Number: getHexString(num, 0),
		Hash:         "0x1234567890123456789012345678901234567890",
		ParentHash:   "0x098765432109876543210987654210987654321",
		Nonce:        "0x8888444422221111",
		Miner:        "0x1111111111222222222333333333344444444444",
		Difficulty:   "0x442211",
		Timestamp:    "0x55e67c30",
		Transactions: make([]*Transaction, 0, 10),
		Uncles:       make([]string, 0, 10)}
	return block
}

func (self *MockGeth) GetTransactionByHash(num *big.Int) *Transaction {
	txn := &Transaction{Hash: getHexString(num, 40),
		From:  "0x1111111111222222222233333333333444444444",
		To:    "0x4444444444333333333322222222221111111111",
		Value: "0x10"}
	if self.transactionsConfirmed {
		txn.BlockNumber = "0x30"
	}
	return txn
}

func (self *MockGeth) GetBlockNumber() (*big.Int, error) {
	return big.NewInt(self.blockNumber), nil
}

func (self *MockGeth) GetLastConfirmedBlockNumber() (*big.Int, error) {
	return big.NewInt(self.blockNumber-8), nil
}
