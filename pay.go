package main

//
// package for payment processing.
// implements a reliable payment system on top of geth's unreliable payments
//

import "math/big"
import "log"
import "time"
import "os"
import "sync"
import "net/http"

import "bufio"
import "io/ioutil"
import "encoding/json"

import "github.com/satori/go.uuid"

//var complete_persist *FilePersistence = nil

type PaymentListener interface {
    PaymentAdded(*PendingTransaction)
    PaymentSent(*PendingTransaction)
    PaymentResent(*PendingTransaction)
    PaymentVerified(*PendingTransaction)
}

type PendingTransaction struct {
	BlockSent   string       // the block in which the transaction was originally sent. '0' means it originally failed to send and we may need a new nonce
    Id          string
	Transaction *Transaction
}

func (self *PendingTransaction) getBlockSent() *big.Int {
    blockSent, err := parseHex(self.BlockSent, 0)

    if err != nil {
        panic(err)
    }

    return blockSent
}

func (self *PendingTransaction) isUnsent() bool {
    zero := big.NewInt(0)
    return self.getBlockSent().Cmp(zero) == 0
}

func (self *PendingTransaction) isStale(lastConfirmedBlock *big.Int) bool {
    return self.getBlockSent().Cmp(lastConfirmedBlock) < 0
}

// maybe not the 'best' name. 
// but we will consider a transaction invalid if it does not have a valid hash.
func (self *PendingTransaction) isInvalid() bool {
    zero := big.NewInt(0)
    return self.Transaction.getHash().Cmp(zero) == 0
}

type PaymentProcessor struct {
	eth          EthAll
	currentNonce *big.Int
	lock         *sync.Mutex
    listeners   []PaymentListener
	pending_file *FilePersistence
	blockCache   []*Block
	pending      map[string]*PendingTransaction
}

/*
 * create a new payment processor
 */
func NewPaymentProcessor(eth EthAll, pendingFilename string) *PaymentProcessor {
	self := &PaymentProcessor{
		eth:          eth,
		currentNonce: big.NewInt(0),
		lock:         &sync.Mutex{},
        listeners: make([]PaymentListener, 0, 10),
		pending_file: NewFilePersistence(pendingFilename),
		pending:      nil}

	if _, err := os.Stat(pendingFilename); os.IsNotExist(err) {
		self.pending = make(map[string]*PendingTransaction)
		self.pending_file.Write(self.pending)
	} else {
		self.pending_file.Read(&self.pending)
	}

	return self
}

func (self *PaymentProcessor) RegisterListener(l PaymentListener) {
    self.listeners = append(self.listeners, l)
}

/*
 * gets the next nonce.
 * the nonce counter then increments
 */
func (self *PaymentProcessor) getNewNonce() (*big.Int, error) {
	coinbase, err := self.eth.GetCoinbase()

	if err != nil {
		return nil, err
	}

    // *but* if current nonce in eth is greater, take that instead
	txnCount, err := self.eth.GetTransactionCount(coinbase)

    if err != nil {
        return nil, err
    }

    // increment internal nonce counter
	self.currentNonce.Add(self.currentNonce, big.NewInt(1))

	if txnCount.Cmp(self.currentNonce) > 0 {
		self.currentNonce.Set(txnCount)
	}

	return self.currentNonce, nil
	//XXX use persistence. DO NOT TRUST GETH
}

/**
 * Send a transaction
 */
func (self *PaymentProcessor) addTransaction(id string, from, to, value *big.Int) {
    newTxn := &Transaction{Hash: "0x0",
                           From: getHexString(from, 40),
                           To: getHexString(to, 40),
                           Value: getHexString(value, 0)}
                           ptxn := &PendingTransaction{BlockSent: "0x0", Id: id, Transaction: newTxn}

    for _, listener := range(self.listeners) {
        listener.PaymentAdded(ptxn)
    }

    self.addToPending(ptxn)
}

/*
 * add a pending transaction to the pending list.
 */
func (self *PaymentProcessor) addToPending(tx *PendingTransaction) {
    //we use UUIDs as map keys. there is a very small chance of collision, so we double check
    for {
        key := uuid.NewV4().String()

        // if (in the unlikely event) we already have this UUID in the map
        // grab a new one
        if _, ok := self.pending[key]; ok {
            log.Println("pay: HASH COLLISION; very unlikely")
            continue
        }

        self.pending[key] = tx
        self.pending_file.Write(self.pending)
        break
    }
}

func (self *PaymentProcessor) updatePending(key string, tx *PendingTransaction) {
    self.pending[key] = tx
	self.pending_file.Write(self.pending)
}

/*
 * update the payment state.
 * check if there are any confirmed payments or if
 * any payments need resending
 */
func (self *PaymentProcessor) update() {
	self.lock.Lock()
	defer self.lock.Unlock()

	lastConfirmedBlock, err := self.eth.GetLastConfirmedBlockNumber()

	if err != nil {
		log.Printf("pay: error getting last confirmed block - " + err.Error())
		return
	}

	currentBlock, err := self.eth.GetBlockNumber()

	if err != nil {
        log.Printf("pay: error getting block number - " + err.Error())
		return
	}

	for key, txn := range self.pending {
		txnBlockNum, err := parseHex(txn.BlockSent, 0)

        if err != nil {
            log.Println("pay: ERROR, could not get sent block of txn: ", err.Error())
            continue
        }

        if txn.isUnsent() {
            fromAddr := txn.Transaction.getFromAddr()
            toAddr := txn.Transaction.getToAddr()
            value := txn.Transaction.getValue()
            nonce, err := self.getNewNonce()

            if err != nil {
                log.Println("pay: error, could not get new nonce for unsent transaction: ", err.Error())
                continue
            }

			newTxn, err := self.eth.SendTransaction(fromAddr, toAddr, value, nil)

            if err != nil {
                log.Println("pay: could not send transaction - ", err.Error())
                txn.Transaction.Nonce = getHexString(nonce, 8)
                newTxn = txn.Transaction
            }

            log.Println("pay: found unsent txn (nonce: ", nonce.String(), "); sending now")

            txn.BlockSent = getHexString(currentBlock, 0)
            txn.Transaction = newTxn

            for _, listener := range(self.listeners) {
                listener.PaymentSent(txn)
            }

			self.updatePending(key, txn)
        } else if txn.isInvalid() {
            log.Println(txn.Transaction)
            fromAddr := txn.Transaction.getFromAddr()
            toAddr := txn.Transaction.getToAddr()
            value := txn.Transaction.getValue()
            nonce := txn.Transaction.getNonce()
            //XXX above is kinda weird; we expect that the nonce has been set even if the transaction
            // is invalid

            if txn.isStale(lastConfirmedBlock) {
                nonce, err = self.getNewNonce()

                if err != nil {
                    log.Println("could not retrieve new nonce for stale, invalid txn: ", err.Error())
                    continue
                }

                log.Println("invalid transaction is stale, grabbed new nonce (nonce:",nonce.String(),")")
            }

            log.Println("pay: found invalid txn (nonce: ", nonce.String(), "); resending")

			newTxn, err := self.eth.SendTransaction(fromAddr, toAddr, value, nil)

            if err != nil {
                log.Println("pay: could not send invalid transaction - ", err.Error())
                continue
            }

            txn.BlockSent = getHexString(currentBlock, 0)
            txn.Transaction = newTxn

            for _, listener := range(self.listeners) {
                listener.PaymentResent(txn)
            }

			self.updatePending(key, txn)
        } else if txn.isStale(lastConfirmedBlock) {
            // get transaction from geth, and check the transaction is still 'pending' (has not been mined)
            // if it has, then move it to the db for record. else try resending it with the same nonce

            txnHash := txn.Transaction.getHash()

			// check again with geth to see if the txn is in the blockchain yet.
			// if it isn't (it's still pending), resend it
            gethTxn := self.eth.GetTransactionByHash(txnHash)

			if gethTxn == nil || gethTxn.isPending() {
                // we couldn't find a transaction with that hash, or it hasn't been confirmed after 8 blocks

				fromAddr := txn.Transaction.getFromAddr()
				toAddr := txn.Transaction.getToAddr()
				value := txn.Transaction.getValue()
				nonce, err := self.getNewNonce()

                if err != nil {
                    log.Println("could not get a fresh nonce")
                    continue
                }

                log.Println("pay: found stale txn (nonce: ", nonce.String(), "); resending")

				newTxn, err := self.eth.SendTransaction(fromAddr, toAddr, value, nil)

				if err != nil {
					log.Println("could not resend transaction: " + err.Error())
					continue
				}

                txn.BlockSent = getHexString(currentBlock, 0)
                txn.Transaction = newTxn
                self.updatePending(key, txn)
			} else {
                txn.Transaction = gethTxn

                for _, listener := range(self.listeners) {
                    listener.PaymentVerified(txn)
                }

                delete(self.pending, key)
                self.pending_file.Write(self.pending)

				nonce := txn.Transaction.getNonce()
                log.Println("pay: found complete txn (nonce: ", nonce.String(), ") adding to db")
			}
		} else {
            waitBlock := big.NewInt(8)
            waitBlock.Add(waitBlock, txnBlockNum)
			nonce := txn.Transaction.getNonce()
            log.Println("pay: waiting for hardened block before confirmation: (block: ", currentBlock.String(), "wait block: ", waitBlock.String(), ", nonce: ", nonce.String(), ")")
		}
	}
}

func (self *PaymentProcessor) handle_addPayment(rpcRequest *RPCRequest) *RPCResponse {
	var from, to, value *big.Int = nil, nil, nil
	var err error = nil

    id, err := rpcRequest.GetParam(0)

	if err != nil {
	    return NewRPCError(rpcRequest.Id, -1, "invalid rpc parameter (0)", nil)
	}

	from, err = rpcRequest.GetBigIntParam(1, 40)

	if err != nil {
	    return NewRPCError(rpcRequest.Id, -1, "invalid rpc parameter (1)", nil)
	}

	to, err = rpcRequest.GetBigIntParam(2, 40)

	if err != nil {
	    return NewRPCError(rpcRequest.Id, -1, "invalid rpc parameter (2)", nil)
	}

	value, err = rpcRequest.GetBigIntParam(3, 0)

	if err != nil {
	    return NewRPCError(rpcRequest.Id, -1, "invalid rpc parameter (3)", nil)
	}

    self.lock.Lock()
    defer self.lock.Unlock()
    self.addTransaction(id, from, to, value)

	return NewRPCResult(rpcRequest.Id, true)
}

func (self *PaymentProcessor) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	bodyReader := bufio.NewReader(request.Body)
	bytes, _ := ioutil.ReadAll(bodyReader)
	rpcRequest := RPCRequest{}
	json.Unmarshal(bytes, &rpcRequest)

	var rpcResponse *RPCResponse = nil

	if rpcRequest.Method == "echo_addPayment" {
		rpcResponse = self.handle_addPayment(&rpcRequest)
	} else {
        rpcResponse = NewRPCError(rpcRequest.Id, -1, "unsupported rpc request: " + rpcRequest.Method, nil)
	}

	jresponse, err := json.Marshal(rpcResponse)

	if err != nil {
		log.Println("invalid json marshalling!!!")
		return
	}

	responseWriter.Write([]byte(jresponse))
}

/*
 * starts the main payment processing thread
 */
func (self *PaymentProcessor) Start(finished chan bool) {
	go http.ListenAndServe(":" + PAY_RPC_PORT, self)

	for !SHUTDOWN {
		self.update()
		time.Sleep(time.Duration(PAY_WAIT) * time.Second)
	}
	log.Printf("pay server is down")

	if finished != nil {
		finished <- true
	}
}

func pay_main() {
	geth := NewGeth(GETH_IP, GETH_PORT)
    //XXX add db listener
	pay := NewPaymentProcessor(geth, PAY_PERSIST_FILENAME)

	//http.ListenAndServe(":" + PAY_RPC_PORT, pay)
	pay.Start(nil)
}
