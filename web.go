package main

//
// web payment callback RPC
//

import "math/big"
import "log"
import "encoding/json"

type WebPaymentProcessor struct {
    server *Server
}

func NewWebPaymentProcessor(server *Server) *WebPaymentProcessor {
    return &WebPaymentProcessor{server: server}
}

func (self *WebPaymentProcessor) PaymentAdded(pmt *PendingTransaction) {
}

func (self *WebPaymentProcessor) PaymentSent(pmt *PendingTransaction) {
    type Message struct {
        IncomingId  string `json:"incoming_txid"`
        OutgoingId  string `json:"outgoing_txid"`
    }

    st := &Message{IncomingId: pmt.Id, OutgoingId: pmt.Transaction.Hash}

    msg, err := json.Marshal(st)

    if err != nil {
        log.Println("web: could not send payment message!", err.Error())
        return
    }

    err = self.server.SendMessage("successfullySent", msg)

    if err != nil {
        log.Println("web: could not mark pending - ", err.Error())
    }
}

func (*WebPaymentProcessor) PaymentResent(*PendingTransaction) {
}

func (self *WebPaymentProcessor) PaymentVerified(pmt *PendingTransaction) {
    type Message struct {
        IncomingId  string `json:"incoming_txid"`
        OutgoingId  string `json:"outgoing_txid"`
    }

    st := &Message{IncomingId: pmt.Id, OutgoingId: pmt.Transaction.Hash}

    msg, err := json.Marshal(st)

    if err != nil {
        log.Println("web: could not send payment message!", err.Error())
        return
    }

    err = self.server.SendMessage("SuccessfullyVerified", msg)

    if err != nil {
        log.Println("web: could not mark verified - ", err.Error())
    }
}

//////// this is no longer used. distributed 5 ETHER for PPLNS

type BalanceUpdater struct {
	eth   EthAll
	cache []*Block
}

func NewBalanceUpdater(e EthAll) *BalanceUpdater {
	return &BalanceUpdater{e, make([]*Block, 0, 20)}
}

func (*BalanceUpdater) BeginProcessing() error {
	return nil
}

func (*BalanceUpdater) EndProcessing() error {
	return nil
}

func (self *BalanceUpdater) Commit() error {
	coinbase, err := self.eth.GetCoinbase()
	if err != nil {
		log.Printf("could not get coinbase " + err.Error())
		return err
	}

	for _, block := range self.cache {
		// if we mined that shit
		if block.getMiner().Cmp(coinbase) == 0 {
			blockNumber, err := parseHex(block.Number, 0)

			if err != nil {
				return err
			}

            if server != nil {
			    server.AddBlock(blockNumber)
            }
			updateBalances(big.NewInt(0), big.NewInt(5)) // distribute 5
		}
	}

	self.cache = make([]*Block, 0, 20)

	return nil
}

func (self *BalanceUpdater) AddBlock(block *Block) {
	self.cache = append(self.cache, block)
}

type Balance struct {
	account *big.Int
	value   *big.Int
}

func updateBalances(balance, newBalance *big.Int) {
	dif := big.NewInt(0)
    if server != nil {
	    server.UpdateBalances(dif.Sub(newBalance, balance))
    }
	pool.resetHashcounts()
}
