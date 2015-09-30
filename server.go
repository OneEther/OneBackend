package main

//
// RPC functionality to the web server
//

import "math/big"
import "math/rand"
import "log"
import "net/http"
import "bytes"
import "errors"
import "fmt"
import "encoding/json"
import "time"
import "io"
import "io/ioutil"

/*
func getJsonTransactionMessage(coinbase, miner, amount *big.Int) (string, error) {
	type ethRPCTransaction struct {
		From  string `json:"from"`
		To    string `json:"to"`
		Value string `json:"value"`
		//Data    string `json:"data"`
	}

	type ethTransactionRPCRequest struct {
		Id      interface{}          `json:"id"`
		Jsonrpc string               `json:"jsonrpc"`
		Method  string               `json:"method"`
		Params  [1]ethRPCTransaction `json:"params"`
	}

	transact := ethRPCTransaction{From: getHexString(coinbase, 40), To: getHexString(miner, 40), Value: amount.String()}

	request := ethTransactionRPCRequest{Id: 1, Jsonrpc: "2.0", Method: "eth_sendTransaction"}
	request.Params[0] = transact

	msg, err := json.Marshal(request)

	if err != nil {
		return "", errors.New("cannot get transaction message json - " + err.Error())
	}

	return string(msg), nil
}*/

type Server struct {
}

func NewServer() (*Server) {
    return &Server{}
}

// for PPLNS, not used
func getMinersDivvy(value *big.Int) []*Balance {
	pool.stateLock.Lock()
	defer pool.stateLock.Unlock()

	balanceList := make([]*Balance, 0, len(pool.miners))

	if debugDivvy {
		log.Printf("DIVVY: " + value.String() + "\n")
	}

	for _, v := range pool.miners {
		share := getAccountShare(value, v.getHashes(), pool.getTotalHashes())

		if debugDivvy {
			log.Printf("ADDR: " + getHexString(v.address, 40) + "\n")
		}

		balanceList = append(balanceList, &Balance{account: v.address, value: share})
	}
	return balanceList
}

// for PPLNS, not used
func getAccountShare(divvy, hashes, totalHashes *big.Int) *big.Int {
    zero := big.NewInt(0)

	rat := big.NewRat(1.0, 1.0)
	divvyRat := big.NewRat(1.0, 1.0)
	share := big.NewRat(1.0, 1.0)
	intShare := big.NewInt(0)
	cut := big.NewRat(1.0, 1.0)
	cut.SetFloat64(1.0 - HOUSE_RAKE)

    if hashes.Cmp(zero) == 0 {
        return zero
    }

	if totalHashes.Cmp(big.NewInt(0)) >= 0 {
		rat.SetFrac(hashes, totalHashes)
		rat.Mul(rat, cut)
		divvyRat.SetInt(divvy)
		share.Mul(rat, divvyRat)

		intShare.Set(share.Num())
		intShare.Div(intShare, share.Denom())
	}

	return intShare
}

func getJsonBalances(divvy *big.Int) []byte {
	type jsonBalanceEntry struct {
		Address string `json:"address"`
		Balance string `json:"balance"`
	}

	type jsonBalanceList struct {
		Updatelist []jsonBalanceEntry `json:"updatelist"`
	}

	bList := jsonBalanceList{}

	balances := getMinersDivvy(divvy)

	for _, balance := range balances {
		bList.Updatelist = append(bList.Updatelist, jsonBalanceEntry{Address: getHexString(balance.account, 40), Balance: balance.value.String()})
	}

	ret, err := json.Marshal(bList)

	if err != nil {
		log.Printf("bad news bears, we cant turn the balances to json")
	}

	return ret
}

func get_jsonBlock(blockNumber *big.Int) []byte {
	return []byte(blockNumber.String())
}

func (*Server) SendMessage(target string, message []byte) error {
	addr := "http://" + BACKEND_IP + ":" + BACKEND_PORT + "/" + target

	if debugServer {
		log.Printf("SERVER: Sending message to " + addr + "\n")
		log.Printf(string(message) + "\n")
	}

	req, err := http.NewRequest("POST", addr, bytes.NewBuffer(message))

	if err != nil {
		return errors.New("cannot create request to server: " + err.Error())
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connection", "close")
	req.Close = true
	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return errors.New("could not send request to server" + err.Error())
	}

    io.Copy(ioutil.Discard, resp.Body)
	resp.Body.Close()
	return error(nil)
}

func (self *Server) AddBlock(blockNumber *big.Int) {
    // no longer used
}

// {"address": ..., "difficulty": ..., "value": ..., "pw": ...}
func (self *Server) submitShare(mr *Miner, sharePrice *big.Int) {
    type shareJson struct {
        Address     string `json:"address"`
        Difficulty  string `json:"difficulty"`
        Value       string `json:"value"`
        Pw          string `json:"pw"`
    }

    st := &shareJson{Address: getHexString(mr.address, 40),
              Difficulty: mr.getDifficulty().String(),
              Value: sharePrice.String(),
              Pw: "super_secret_password"}

    msg, err := json.Marshal(st)

    if err != nil {
        panic(err)
    }

	self.SendMessage("addShares", msg)
}

func (self *Server) SubmitHashrate(mr *Miner) error {
	claim := mr.getClaimedHashrate()
	actual := mr.getTrueHashrate()

	uiHashrate := big.NewRat(1, 1)
	uiHashrate.SetFrac(claim, big.NewInt(1))

	upperBound := big.NewRat(2, 1)
	lowerBound := big.NewRat(1, 2)

	actualRat := big.NewRat(1, 1)
	actualRat.SetFrac(actual, big.NewInt(1))
	upperBound.Mul(upperBound, actualRat)
	lowerBound.Mul(lowerBound, actualRat)

	if uiHashrate.Cmp(upperBound) > 0 {
		uiHashrate.Set(upperBound)
	}

	if uiHashrate.Cmp(lowerBound) < 0 {
		uiHashrate.Set(lowerBound)
	}

	fhashrate, _ := uiHashrate.Float64()

	msg := get_jsonHashrate(mr.address, big.NewInt(int64(fhashrate)))

	self.SendMessage("addHashes", msg)

	return error(nil)
}

func get_jsonHashrate(miner *big.Int, hashrate *big.Int) []byte {
	type hashrateJson struct {
		Address  string `json:"address"`
		Hashrate string `json:"hashrate"`
	}

	hashrateStruct := hashrateJson{Address: getHexString(miner, 40), Hashrate: fmt.Sprintf("%d", hashrate)}

	if debugServer {
		log.Printf("HASHRATE: " + hashrateStruct.Address + " " + hashrateStruct.Hashrate + "\n")
	}

	ret, err := json.Marshal(hashrateStruct)

	if err != nil {
		log.Printf("ERROR: cannot get updatelist: this is bad... really bad!")
	}

	return ret
}

func (self *Server) UpdateBalances(divvy *big.Int) {
	msg := getJsonBalances(divvy)

	log.Printf("BALANCEMSG: Sending balance update to server: " + string(msg) + "\n")

	self.SendMessage("addEther", msg)
}
