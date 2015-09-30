package main

//
// OneEther
//

import "os/signal"
import "runtime/pprof"
import "flag"
import "os"
import "math/big"
import "time"
import "errors"
import "net/http"
import "fmt"
import "io/ioutil"
import "bufio"
import "encoding/json"
import "log"

//TODO proper miner diff

var workLog *log.Logger

var pool *MinerPool
var geth *Geth
var server *Server
var pay *PaymentProcessor

var config *Config

var sigkill chan os.Signal

const (
	PUBLIC_ERROR  = iota
	PRIVATE_ERROR = iota
)

type RequestError struct {
	when    time.Time
	message string
	kind    int
}

func (e RequestError) Error() string {
	return e.message
}

func (e RequestError) String() string {
	return fmt.Sprintf("%v: %v", e.when, e.message)
}

func NewRequestError(message string, kind int) RequestError {
	return RequestError{when: time.Now(), message: message, kind: kind}
}

func writeResponse(w http.ResponseWriter, response interface{}) {
	responseOutput, _ := json.Marshal(response)

	if debugRPC {
		log.Printf("\n\n----- RESPONSE ----->>\n")
		log.Printf(string(responseOutput))
	}

	fmt.Fprintf(w, string(responseOutput))
}

// used to check if a requested method even exists
func methodIsValid(method string) bool {
	switch method {
	case "web3_clientVersion",
		"web3_sha3",
		"net_version",
		"net_peerCount",
		"net_listening",
        "eth_alive",
		"eth_protocolVersion",
		"eth_coinbase",
		"eth_mining",
		"eth_hashrate",
		"eth_gasPrice",
		"eth_accounts",
		"eth_blockNumber",
		"eth_getBalance",
		"eth_getStorageAt",
		"eth_getTransactionCount",
		"eth_getBlockTransactionCountByHash",
		"eth_getBlockTransactionCountByNumber",
		"eth_getUncleCountByBlockHash",
		"eth_getUncleCountByBlockNumber",
		"eth_getCode",
		"eth_sign",
		"eth_sendTransaction",
		"eth_call",
		"eth_estimateGas",
		"eth_getBlockByHash",
		"eth_getBlockByNumber",
		"eth_getTransactionByHash",
		"eth_getTransactionByBlockHashAndIndex",
		"eth_getTransactionByBlockNumberAndIndex",
		"eth_getTransactionReceipt",
		"eth_getUncleByBlockHashAndIndex",
		"eth_getUncleByBlockNumberAndIndex",
		"eth_getCompilers",
		"eth_compileLLL",
		"eth_compileSolidity",
		"eth_compileSerpent",
		"eth_newFilter",
		"eth_newBlockFilter",
		"eth_newPendingTransactionFilter",
        "eth_ping",
		"eth_uninstallFilter",
		"eth_getFilterChanges",
		"eth_getFilterLogs",
		"eth_getLogs",
		"eth_getWork",
		"eth_submitWork",
		"eth_submitHashrate",
		"db_putString",
		"db_getString",
		"db_putHex",
		"db_getHex",
		"shh_post",
		"shh_version",
		"shh_newIdentity",
		"shh_hasIdentity",
		"shh_newGroup",
		"shh_addToGroup",
		"shh_newFilter",
		"shh_uninstallFilter",
		"shh_getFilterChanges",
		"shh_getMessages":
		return true
	}
	return false
}

// only allow certain methods
func methodIsAllowed(method string) bool {
	switch method {
	case "eth_getWork",
        "eth_alive",
		"eth_submitWork",
		"eth_submitHashrate",
        "eth_ping",
		"eth_protocolVersion",
		"eth_coinbase",
		"eth_mining":
		return true
	}
	return false
}

// send work to the miner
func eth_getWork(request *RPCRequest, miner *big.Int) (*RPCResponse, error) {
	mr := pool.getMiner(miner)

	if mr == nil {
		return nil, errors.New("invalid miner")
	}

	response, err := geth.SendRPCRequest(request)

	if err != nil {
		return nil, errors.New("could send rpc request to geth: " + err.Error())
	}

	headerHash, err := response.GetBigIntEntryResult(0, 32)

	if err != nil {
		return nil, errors.New("could not get result[0] - " + err.Error())
	}

	seedHash, err := response.GetBigIntEntryResult(1, 32)

	if err != nil {
		return nil, errors.New("could not get result[1] - " + err.Error())
	}

	target, err := response.GetBigIntEntryResult(2, 32)

	if err != nil {
		return nil, errors.New("could not get result[2] - " + err.Error())
	}

	// block has changed, update pool information
	if pool.headerHash.Cmp(headerHash) != 0 {
		var err error = nil

		difficulty := big.NewInt(1)
		difficulty.Lsh(difficulty, 255)
		difficulty.Div(difficulty, target)
		difficulty.Lsh(difficulty, 1)

		pool.lock()
		pool.blockStart = time.Now()
		pool.submissions = make(map[string]bool)
		pool.blockNumber, err = geth.GetBlockNumber()
		pool.blockDifficulty = difficulty
		pool.headerHash = headerHash
		pool.seedHash = seedHash
		pool.unlock()

		if err != nil {
			log.Printf("could not get block number!\n")
			return nil, err
		}
	}

	mr.lastPost = time.Now()

	response.ReplaceResult(2, getHexString(getBoundaryCondition(mr.getDifficulty()), 32))
	return response, nil
}

// sends an RPC to a verify process. Currently this is a modified version of the py-ethereum library (for historical reasons)
func verifyWork(blockNumber, headerHash, mixHash, nonce, difficulty *big.Int) bool {
	if debugVerify {
		log.Printf("sending verify #1: " + difficulty.String() + " : " + nonce.String() + "\n")
		log.Printf("sending verify #2: " + blockNumber.String() + " : " + headerHash.String() + "\n")
	}

	request := NewRPCRequest(1, "verify", RPCParams{
		blockNumber.String(),
		getHexString(headerHash, 32),
		getHexString(mixHash, 32),
		getHexString(nonce, 8),
		difficulty.String(),
	})

	response, err := sendRPCRequest(request, CONFIRM_ADDR)

	if err != nil {
		log.Printf("error while running verify!\n")
		return false
	}

	ret, err := response.GetBoolResult()
	return err == nil && ret
}

// assumes 5Eth block payout, calculates the worth of a share.
func calculateSharePayout(difficulty, poolDifficulty *big.Int) float64 {
    fiveEther := big.NewRat(1, 1)
    fiveEther.SetFrac(big.NewInt(5000000000000000000), big.NewInt(1))
    sharePrice := big.NewRat(1, 1)
    sharePrice.SetFrac(difficulty, poolDifficulty)
    sharePrice.Mul(sharePrice, fiveEther)
    shareFloat, _ := sharePrice.Float64()

    shareFloat *= (1.0 - HOUSE_RAKE)
    return shareFloat
}

// figure out if the submitted share is valid
func eth_submitWork(request *RPCRequest, minerAddr *big.Int) (*RPCResponse, error) {
	nonce, err := request.GetBigIntParam(0, 8)

	if err != nil {
		return nil, NewRequestError("invalid RPC parameters(0) - Nonce", PUBLIC_ERROR)
	}

	_, err = request.GetBigIntParam(1, 32)

	if err != nil {
		return nil, NewRequestError("invalid RPC parameters(1) - POW Hash", PUBLIC_ERROR)
	}

	mixHash, err := request.GetBigIntParam(2, 32)

	if err != nil {
		return nil, NewRequestError("invalid RPC parameters(2) - digest", PUBLIC_ERROR)
	}

	miner := pool.getMiner(minerAddr)

	dt := time.Since(miner.lastSubmit).Seconds() + 0.1
    difficulty := big.NewInt(0)
	difficulty.Set(miner.getDifficulty())
	diff := float64(difficulty.Int64())

    miner.lastPost = time.Now()
	blockNumber := pool.getBlockNumber()
	headerHash := pool.getHeaderHash()

	// block#, header hash, mix hash, nonce, difficulty
	if verifyWork(blockNumber, headerHash, mixHash, nonce, difficulty) {
		poolDifficulty := pool.getDifficulty()

		hashrate := diff / dt

		log.Println("MINER: ", miner.shares.String(), " ", time.Since(miner.joinTime), "::", float64(miner.shares.Uint64())/time.Since(miner.joinTime).Seconds())
        log.Println("HRATE: DT-", dt, " C-", miner.getClaimedHashrate().String(), " T-", miner.getTrueHashrate().String())

		pool.lock()
		defer pool.unlock()

		// avoid duplicates by storing nonce in global map; if entry is not in map, then this is a new solution
		_, exists := pool.submissions[nonce.String()]
		if !exists {
            payout := calculateSharePayout(difficulty, poolDifficulty)
		    miner.claimShare(diff, dt, payout)
			miner.hashrate.push(hashrate)
	        miner.lastSubmit = time.Now()
			pool.submissions[nonce.String()] = true

            if server != nil {
                server.submitShare(miner, big.NewInt(int64(payout)))
            }
		} else {
			return NewRPCResult(request.Id, false), nil
		}

		miner.difficulty = miner.GetNewDifficulty()

		//FOUND A BLOCK, DAWG
		log.Printf("POOL DIFFICULTY: " + poolDifficulty.String())
		if verifyWork(blockNumber, headerHash, mixHash, nonce, poolDifficulty) {
			log.Printf("BLOCK FOUND!!!!")
            miner.blocks.Add(miner.blocks, big.NewInt(1))
			return geth.SendRPCRequest(request)
		}

		workLog.Printf("DT: %f\n", dt)
		workLog.Printf("DIF: %f\n", diff)
		workLog.Printf("HASHRATE: %f\n", miner.hashrate.getAverage())
	} else {
		log.Println("FAIL SUBMIT! ", getHexString(miner.address, 40))
		return NewRPCResult(request.Id, false), nil
	}

	return NewRPCResult(request.Id, true), nil
}

/*
 */

// process miner's submitted hashrate. helpful for estimating share difficulty
func eth_submitHashrate(request *RPCRequest, minerAddr *big.Int) (*RPCResponse, error) {
	miner := pool.getMiner(minerAddr)

	claimedHashrate, err := request.GetBigIntParam(0, 32)

	if miner != nil {
		request.Params[0] = getHexString(miner.getTrueHashrate(), 32)
	}

	if err != nil {
		return nil, errors.New("invalid RPC parameter(0) - " + err.Error())
	}

	id, err := request.GetBigIntParam(1, 32)

	if err != nil {
		return nil, errors.New("invalid RPC parameters(1) - " + err.Error())
	}

	response, err := geth.SendRPCRequest(request)

	if err != nil {
		return nil, errors.New("could not contact geth" + err.Error())
	}

	result, err := response.GetBoolResult()
	if !result || err != nil {
		return nil, NewRequestError("hashrate submitted is invalid", PUBLIC_ERROR)
	}

    miner.lastPost = time.Now()
	machine := miner.getMachine(id)
	machine.claimedHashrate.Set(claimedHashrate)
	machine.lastUpdate = pool.tick


	return response, nil
}

// figure out how to handle the request, send it to geth if needed
func proxyRequest(request *RPCRequest, minerAddr *big.Int) (*RPCResponse, error) {
	if !methodIsValid(request.Method) {
		return nil, NewRequestError("invalid RPC method: "+request.Method, PUBLIC_ERROR)
	}

	if !methodIsAllowed(request.Method) {
		return nil, NewRequestError("Restricted request method: "+request.Method, PUBLIC_ERROR)
	}

	switch request.Method {
    case "eth_ping":
        return NewRPCResult(request.Id, true), nil
	case "eth_getWork":
		return eth_getWork(request, minerAddr)
	case "eth_submitWork":
		return eth_submitWork(request, minerAddr)
	case "eth_submitHashrate":
		return eth_submitHashrate(request, minerAddr)
    case "eth_alive":
        return NewRPCResult(request.Id, true), nil
	default:
		return geth.SendRPCRequest(request) //forward other allowed requests to GETH
	}
}

// main HTTP entry point
func httpHandler(w http.ResponseWriter, r *http.Request) {
    if SHUTDOWN {
        return
    }

	err := r.ParseForm()

	if err != nil {
		log.Printf("ERROR: invalid http request form - " + err.Error() + "\n")
		rpcerr := NewRPCError(1, -32602, "invalid http request", nil)
		writeResponse(w, rpcerr)
		return
	}

	minerAddrStr := r.FormValue("miner")

	if len(minerAddrStr) <= 0 {
		log.Printf("WARNING attempt to mine to NULL - " + r.URL.String() + "\n")
		rpcerr := NewRPCError(1, -32602, "invalid or missing miner id", nil)
		writeResponse(w, rpcerr)
		return
	}

	minerAddr, err := parseHex(minerAddrStr, 0)

	if err != nil {
		log.Printf("ERROR getting miner form value - " + err.Error() + "\n")
		rpcerr := NewRPCError(1, -32602, "could not retrieve miner id from request", nil)
		writeResponse(w, rpcerr)
		return
	}

	bodyReader := bufio.NewReader(r.Body)
	bytes, _ := ioutil.ReadAll(bodyReader)
	request := RPCRequest{}
	json.Unmarshal(bytes, &request)

	if debugRPC {
		log.Printf("\n\n<<----- REQUEST -----\n")
		log.Printf(string(bytes))
	}

	if debugRequest {
		workLog.Printf("REQ FROM " + minerAddrStr + "\n")
		workLog.Printf(string(bytes))
	}

	if len(minerAddrStr) <= 0 {
		log.Printf("NULLMSG: " + string(bytes) + "\n")
	}

	response, err := proxyRequest(&request, minerAddr)

	if err != nil {
		log.Printf("ERROR proxying request - " + err.Error() + "\n")
		rpcerr := NewRPCError(1, -32602, "could not process request - server side error", nil)
		writeResponse(w, rpcerr)
		return
	}

	if response == nil {
		panic("expected response!")
	}

	writeResponse(w, response)
}


func getBoundaryCondition(minerDifficulty *big.Int) *big.Int {
	zero := big.NewInt(0)

	difficulty := minerDifficulty

	if difficulty.Cmp(zero) == 0 {
        difficulty = big.NewInt(DEFAULT_HASHRATE_ESTIMATE * SHARE_TIME)
	}

	ret := big.NewInt(1)
	ret.Lsh(ret, 255)
	ret.Div(ret, difficulty)
	ret.Lsh(ret, 1)
	return ret
}


// secretly give everyone ether
type secretCommand struct {
	Magic float64 `json:"magic"`
}

func (secretCommand) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	zero := big.NewInt(0)

	bytes, _ := ioutil.ReadAll(bufio.NewReader(request.Body))
	var cmd secretCommand
	json.Unmarshal(bytes, &cmd)

	eth := big.NewInt(int64(cmd.Magic))
	mul := big.NewInt(1)

	fmt.Printf("MAGIC! %f\n", cmd.Magic)
	mul.SetString(weiToEth, 10)
	eth.Mul(eth, mul)

	updateBalances(zero, eth)
}

func main() {
	flag_pool := flag.Bool("pool", false, "Enable pool")
	flag_scanner := flag.Bool("scanner", false, "Enable block chain scanner")
	flag_pay := flag.Bool("pay", false, "Enable payment component")
	flag_web := flag.Bool("web", false, "Enable web backend communication")
	flag_all := flag.Bool("all", false, "Enable all features")
	flag_cpuprofile := flag.String("cpuprofile", "", "write cpu profile to file")
	flag.Parse()

	wait := make(chan bool)
	sigkill = make(chan os.Signal)
	signal.Notify(sigkill, os.Interrupt)

	config = NewConfig(*flag_scanner, *flag_pool, *flag_pay, *flag_web, *flag_all)

	if *flag_cpuprofile != "" {
		f, err := os.Create(*flag_cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	db := NewMongoDB("one")
	pool = newMinerPool(db)
	geth = NewGeth(GETH_IP, GETH_PORT)

	workFile, _ := os.Create("work.log")
	workLog = log.New(workFile, "", log.Ldate|log.Ltime)

	statusPoll := NewStatusPoll(geth, BLOCK_PERSIST_FILENAME)

	log.Println("starting...")

    // launches block chain scanner thread
	if config.scanner {
		bp := NewDatabaseBlockProcessor(db)
		statusPoll.RegisterBlockProcessor(bp)
        log.Println("registered block processor")
	}

    // launches payment thread
	if config.pay {
		bu := NewBalanceUpdater(geth)

		statusPoll.RegisterBlockProcessor(bu)

		pay = NewPaymentProcessor(geth, PAY_PERSIST_FILENAME)
        dbproc := NewDatabasePaymentProcessor(db)
        pay.RegisterListener(dbproc)
        log.Println("registered db payment listener")
		go pay.Start(wait)
		defer func() { <-wait }()
	}

	go statusPoll.Start(wait)
	defer func() { <-wait }()

    // launches pool thread
	if config.pool {
		go pool.start(wait)
		defer func() { <-wait }()
		http.HandleFunc("/", httpHandler)
		go http.ListenAndServe(":7777", secretCommand{})
		go http.ListenAndServe(":"+LISTEN_PORT, nil)
	}

    // launches web payment listener thread
    if config.web {
        server = NewServer()

        if pay != nil {
            webproc := NewWebPaymentProcessor(server)
            pay.RegisterListener(webproc)
            log.Println("registered web payment listener")
        }
    }

	<-sigkill
	SHUTDOWN = true
	log.Println("exiting")
}
