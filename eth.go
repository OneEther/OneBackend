package main

import "strings"
import "log"
import "fmt"
import "math/big"
import "errors"
import "time"
import "encoding/json"

/**
 *
 */
type Account struct {
	Address  string         `json:"address"`
	Incoming []*Transaction `json:"incoming"`
	Outgoing []*Transaction `json:"outgoing"`
	Mined    []string       `json:"mined"` // array of mined block numbers
}

func NewAccount(addr string) *Account {
	return &Account{Address: addr,
		Incoming: make([]*Transaction, 0, 16),
		Outgoing: make([]*Transaction, 0, 16),
		Mined:    make([]string, 0, 10)}
}

/**
 *
 */
type Transaction struct {
	Hash        string `json:"hash"`
	From        string `json:"from"`
	To          string `json:"to"`
	Value       string `json:"value"`
	Nonce       string `json:"nonce"`
	BlockNumber string `json:"blockNumber"`
	Timestamp   string `json:"timestamp"`
}

func (self *Transaction) isPending() bool {
	return len(self.BlockNumber) <= 0
}

func (self *Transaction) getHash() *big.Int {
	ret, err := parseHex(self.Hash, 32)
	if err != nil {
		panic(err)
	}
	return ret
}

func (self *Transaction) getToAddr() *big.Int {
	ret, err := parseHex(self.To, 40)
	if err != nil {
		panic(err)
	}
	return ret
}

func (self *Transaction) getFromAddr() *big.Int {
	ret, err := parseHex(self.From, 40)
	if err != nil {
		panic(err)
	}
	return ret
}

func (self *Transaction) getValue() *big.Int {
    if strings.HasPrefix(self.Value, "0x") {
        ret, err := parseHex(self.Value, 0)
        if err != nil {
            panic(err)
        }
	    return ret
    } else {
        ret := big.NewInt(0)
        ret.SetString(self.Value, 10)
        return ret
    }
}

func (self *Transaction) getNonce() *big.Int {
	ret, err := parseHex(self.Nonce, 8)
	if err != nil {
		panic(err)
	}
	return ret
}

/**
 *
 */
type Block struct {
	Number     string `json:"number"`
	Hash       string `json:"hash"`
	ParentHash string `json:"parentHash"`
	Nonce      string `json:"nonce"`

	Miner      string `json:"miner"`
	Difficulty string `json:"difficulty"`
	Timestamp  string `json:"timestamp"`

	Transactions []*Transaction `json:"transactions"`

	Uncles []string `json:"uncles"`
}

func (self *Block) timeFromBlock(oth *Block) time.Duration {
	return self.getTimestamp().Sub(oth.getTimestamp())
}

func (self *Block) getTimestamp() time.Time {
	num, err := parseHex(self.Timestamp, 0)

	if err != nil {
		fmt.Printf("could not parse timestamp\n")
		return time.Time{}
	}

	return time.Unix(num.Int64(), 0)
}

func (self *Block) getNumber() *big.Int {
	num, err := parseHex(self.Number, 0)

	if err != nil {
		panic(err)
	}

	return num
}

func (self *Block) getHash() *big.Int {
	num, err := parseHex(self.Hash, 40)

	if err != nil {
		panic(err)
	}

	return num
}

func (self *Block) getMiner() *big.Int {
	num, err := parseHex(self.Miner, 40)

	if err != nil {
		panic(err)
	}

	return num
}

func (self *Block) getDifficulty() *big.Int {
	num, err := parseHex(self.Difficulty, 0)

	if err != nil {
		panic(err)
	}

	return num
}

type EthWallet interface {
	SendTransaction(from, to, value, nonce *big.Int) (*Transaction, error)
	GetCoinbase() (*big.Int, error)
	GetBalance() (*big.Int, error)
	GetTransactionCount(*big.Int) (*big.Int, error)
	GetBalanceFromCoinbase(coinbase *big.Int) (*big.Int, error)
}

type EthChain interface {
	GetBlockByNumber(num *big.Int, full bool) *Block
	GetTransactionByHash(num *big.Int) *Transaction
	GetBlockNumber() (*big.Int, error)
	GetLastConfirmedBlockNumber() (*big.Int, error)
}

type EthAll interface {
	EthWallet
	EthChain
}

type Geth struct {
	ip      string
	port    string
	address string
}

func (self *Geth) SendRPCRequest(request *RPCRequest) (*RPCResponse, error) {
	return sendRPCRequest(request, self.address)
}

func (self *Geth) SendRPCRequestRaw(request *RPCRequest) ([]byte, error) {
	return sendRPCRequestRaw(request, self.address)
}

func NewGeth(ip, port string) *Geth {
	return &Geth{ip: ip, port: port, address: "http://" + ip + ":" + port}
}

func (self *Geth) SendTransaction(from, to, value, nonce *big.Int) (*Transaction, error) {
	params := make([]interface{}, 1)

	type TransactionParameters struct {
		From  string `json:"from"`
		To    string `json:"to"`
		Value string `json:"value"`
        Nonce string `json:"nonce,omitempty"`
	}

	fromStr := getHexString(from, 40)
	toStr := getHexString(to, 40)
	valueStr := value.String()
	nonceStr := ""

    if nonce != nil {
	    nonceStr = getHexString(nonce, 8)
    }

    params[0] = &TransactionParameters{From: fromStr, To: toStr, Value: valueStr, Nonce: nonceStr}
	request := NewRPCRequest(1, "eth_sendTransaction", params)
	response, err := self.SendRPCRequest(request)

	if err != nil {
		fmt.Printf("could not send transaction: " + err.Error() + "\n")
		return nil, err
	}

	if response.Result == nil {
		if response.Error != nil {
			return nil, errors.New("error response: " + response.Error.Message)
		}
		return nil, errors.New("could not send transaction: nil result")
	}

	txHash, ok := (*response.Result).(string)

	if !ok {
		fmt.Printf("could not get transaction result; likely errored\n")
		return nil, errors.New("could not get transaction hash")
	}

	ret := &Transaction{Hash: txHash, From: fromStr, To: toStr, Value: valueStr, Nonce: nonceStr, BlockNumber: "", Timestamp: ""}

	return ret, nil
}

func (self *Geth) GetBlockByNumber(num *big.Int, full bool) *Block {
	params := make([]interface{}, 2)
	params[0] = getHexString(num, 0)
	params[1] = full

	request := NewRPCRequest(1, "eth_getBlockByNumber", params)
	jresponse, err := self.SendRPCRequestRaw(request)

	if err != nil {
		fmt.Printf("COULD NOT GET BLOCK " + err.Error() + "\n")
		return nil
	}

	type GetBlockResponse struct {
		Result *Block
	}

	blockResponse := GetBlockResponse{}
	json.Unmarshal(jresponse, &blockResponse)

	block := blockResponse.Result

	if full {
		for _, txn := range block.Transactions {
			txn.Timestamp = block.Timestamp
		}
	}

	return block
}

func (self *Geth) GetTransactionByHash(num *big.Int) *Transaction {
	params := make([]interface{}, 1)
	params[0] = getHexString(num, 40)

	request := NewRPCRequest(1, "eth_getTransactionByHash", params)
	jresponse, err := self.SendRPCRequestRaw(request)

	if err != nil {
		fmt.Printf("could not get transaction: " + err.Error() + "\n")
		return nil
	}

	type GetTransactionResponse struct {
		Result *Transaction
	}

	txnResponse := GetTransactionResponse{}
	json.Unmarshal(jresponse, &txnResponse)

	txn := txnResponse.Result

	if txn == nil {
		return nil
	}

	if !txn.isPending() {
		blockNumber, err := parseHex(txn.BlockNumber, 0)

		if err != nil {
			log.Printf("error getting transaction owned block number")
		} else {
			ownedBlock := self.GetBlockByNumber(blockNumber, false)
			txn.Timestamp = ownedBlock.Timestamp
		}
	}

	return txn
}

func (self *Geth) GetCoinbase() (*big.Int, error) {
	request := NewRPCRequest(1, "eth_coinbase", RPCParams{})

	response, err := self.SendRPCRequest(request)

	if err != nil {
		return nil, err
	}

	coinbase, err := response.GetStringResult()

	if err != nil {
		return nil, errors.New("invalid coinbase response; cannot read string")
	}

	ret, err := parseHex(coinbase, 40)
	return ret, err
}

func (self *Geth) GetBlockNumber() (*big.Int, error) {
	request := NewRPCRequest(1, "eth_blockNumber", RPCParams{})
	response, err := self.SendRPCRequest(request)

	if err != nil {
		return nil, errors.New("invalid RPC response")
	}

	ret, err := response.GetBigIntResult(0)

	return ret, err
}

func (self *Geth) GetLastConfirmedBlockNumber() (*big.Int, error) {
	num, err := self.GetBlockNumber()

	if err != nil {
		return nil, err
	}

	confirmWindow := big.NewInt(8)
	if num.Cmp(confirmWindow) < 0 {
		num = big.NewInt(0)
	} else {
		num.Sub(num, confirmWindow)
	}

	return num, nil
}

func (self *Geth) GetBalanceFromCoinbase(coinbase *big.Int) (*big.Int, error) {
	cbStr := getHexString(coinbase, 40)
	request := NewRPCRequest(1, "eth_getBalance", RPCParams{cbStr, "latest"})
	response, err := self.SendRPCRequest(request)

	if err != nil {
		return nil, errors.New("invalid RPC response")
	}

	ret, err := response.GetBigIntResult(0)

	return ret, err
}

func (self *Geth) GetBalance() (*big.Int, error) {
	coinbase, err := self.GetCoinbase()

	if err != nil {
		return nil, errors.New("cannot get coinbase: " + err.Error())
	}

	return self.GetBalanceFromCoinbase(coinbase)
}

func (self *Geth) GetTransactionCount(account *big.Int) (*big.Int, error) {
	cbStr := getHexString(account, 40)
	request := NewRPCRequest(1, "eth_getTransactionCount", RPCParams{cbStr, "pending"})
	response, err := self.SendRPCRequest(request)

	if err != nil {
		return nil, errors.New("invalid RPC response")
	}

    //XXX may not be hex
	ret, err := response.GetBigIntResult(0)

	return ret, err
}
