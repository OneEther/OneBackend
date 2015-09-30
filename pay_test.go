package main

import "fmt"
import "os"
import "testing"
import "math/big"

/*
func makePendingTxn(e EthAll, from, to, val *big.Int) *PendingTransaction {
	tc, _ := e.GetTransactionCount(big.NewInt(0))
	tx, _ := e.SendTransaction(big.NewInt(0x123), big.NewInt(0x321), big.NewInt(5), tc)
	return &PendingTransaction{"1", tx}
}*/

func TestAddToPending(t *testing.T) {
	if _, err := os.Stat("test.pending"); !os.IsNotExist(err) {
		os.Remove("test.pending")
	}

	geth := &MockGeth{}
	pay := NewPaymentProcessor(geth, "test.pending")

    pay.addTransaction("1", big.NewInt(0x124), big.NewInt(0x421), big.NewInt(6))
    pay.addTransaction("2", big.NewInt(0x125), big.NewInt(0x521), big.NewInt(5))
    pay.addTransaction("3", big.NewInt(0x126), big.NewInt(0x621), big.NewInt(4))
    pay.addTransaction("4", big.NewInt(0x127), big.NewInt(0x721), big.NewInt(3))

	pay = nil
	pay = NewPaymentProcessor(&MockGeth{}, "test.pending")

	if len(pay.pending) != 4 {
		t.Error("expected 4 pending transaction, found ", len(pay.pending))
	}

	os.Remove("test.pending")
}

func TestUpdate(t *testing.T) {
	if _, err := os.Stat("test.pending"); !os.IsNotExist(err) {
		os.Remove("test.pending")
	}

    fmt.Println("pay: update test")

	geth := &MockGeth{blockNumber: 0x01}
	geth.transactionsConfirmed = true
	pay := NewPaymentProcessor(geth, "test.pending")
    pay.addTransaction("1", big.NewInt(0x127), big.NewInt(0x721), big.NewInt(3))
    pay.addTransaction("2", big.NewInt(0x127), big.NewInt(0x721), big.NewInt(3))
	pay.update()
	geth.blockNumber = 0x10
	pay.update()

	if len(pay.pending) != 0 {
		t.Error("expected 0 pending transaction, found ", len(pay.pending))
	}

    pay.addTransaction("3", big.NewInt(0x127), big.NewInt(0x721), big.NewInt(3))
    pay.addTransaction("4", big.NewInt(0x127), big.NewInt(0x721), big.NewInt(3))
	geth.transactionsConfirmed = false
	pay.update()
	pay.update()

	if geth.transactionCount != 4 {
		t.Error("expected 4 geth transactions sent, found ", geth.transactionCount)
	}

	if len(pay.pending) != 2 {
		t.Error("expected 2 pending transaction, found ", len(pay.pending))
	}

	geth.blockNumber = 0x20
	geth.transactionsConfirmed = true
	pay.update()

	if len(pay.pending) != 0 {
		t.Error("expected 0 pending transaction, found ", len(pay.pending))
	}

	if _, err := os.Stat("test.pending"); !os.IsNotExist(err) {
		os.Remove("test.pending")
	}
}
