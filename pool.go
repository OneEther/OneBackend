package main

//
// main pool functionality
//

import "math/big"
import "time"
import "sync"
import "log"
import "errors"

type BlockState struct {
    blockNumber *big.Int
    blockDifficulty *big.Int
    headerHash  *big.Int
	blockStart  time.Time
	submissions map[string]bool // check here to see if someone submitted the solution already
}

type BlockSolution struct {
    nonce       *big.Int
    blockNumber *big.Int
}

type MinerStat struct {
    Address     string          `json:"address"`
    Hashes      string          `json:"hashes"`
    Payout      string          `json:"payout"`
    OnlineTime  time.Duration   `json:"online"`
    Shares      uint64          `json:"shares"`
    Blocks      uint64          `json:"blocks"`
}

type MinerPool struct {
	miners      map[string]*Miner
	submissions map[string]bool // check here to see if someone submitted the solution already
	solutions   map[string]*BlockSolution // check here to see if someone submitted the solution already
	stateLock   *sync.Mutex
	tick        time.Time
	blockStart  time.Time

    db Database

    workingBlocks   []BlockState

	totalHashrate   *big.Int
	blockDifficulty *big.Int
	blockNumber     *big.Int
	headerHash      *big.Int
	seedHash        *big.Int
}

func newMinerPool(db Database) *MinerPool {
	blockDif := big.NewInt(0)
	blockDif.SetString("5000000000000", 10)
	ret := &MinerPool{miners: make(map[string]*Miner),
		submissions:     make(map[string]bool),
		stateLock:       &sync.Mutex{},
		tick:            time.Now(),
		blockStart:      time.Now(),

        db: db,

		totalHashrate:   big.NewInt(0),
		blockDifficulty: blockDif,
		blockNumber:     big.NewInt(0),
		headerHash:      big.NewInt(0),
		seedHash:        big.NewInt(0)}

    if db != nil {
        db.Connect()
    }

	return ret
}

func (self *MinerPool) destroy() {
    for key, mr := range(self.miners) {
        self.removeMiner(mr, key)
    }

    if self.db != nil {
        self.db.Disconnect()
    }
}

func (self *MinerPool) lock() {
	self.stateLock.Lock()
}

func (self *MinerPool) unlock() {
	self.stateLock.Unlock()
}

func (self *MinerPool) getBlockNumber() *big.Int {
	return self.blockNumber
}

func (self *MinerPool) getDifficulty() *big.Int {
	return self.blockDifficulty
}

func (self *MinerPool) getHeaderHash() *big.Int {
	return self.headerHash
}

func (self *MinerPool) getSeedHash() *big.Int {
	return self.seedHash
}

func (self *MinerPool) getTotalHashes() *big.Int {
	ret := big.NewInt(0)
	for _, mr := range self.miners {
		ret.Add(ret, mr.getHashes())
	}
	return ret
}

func (self *MinerPool) getAverageHashrate() *big.Int {
	ret := big.NewInt(0)
	for _, mr := range self.miners {
		ret.Add(ret, mr.getTrueHashrate())
	}
	return ret
}

func (self *MinerPool) addSolution(blockNum, nonce *big.Int) {
    solution := &BlockSolution{blockNumber: blockNum, nonce: nonce}
    self.solutions[nonce.String()] = solution
}

func (self *MinerPool) removeMiner(miner *Miner, key string) {
    pool.lock()
    defer pool.unlock()
    self.writeMinerStats(miner)
	delete(self.miners, key)
}

func (self *MinerPool) update() {
	now := time.Now()
	dt := now.Sub(self.tick)
	dstep := int64(dt.Seconds())

	// do not update more than once a second
	if dstep < 1 {
		return
	}

	for key, mr := range self.miners {
		mr.Update(dstep)

		if now.Sub(mr.lastPost) > CLIENT_TIMEOUT * time.Second {
            log.Println("pool: removing idle miner - ", key)
			self.removeMiner(mr, key)
		} else if now.Sub(mr.lastStat) > CLIENT_DB_WRITEBACK * time.Second {
            self.writeMinerStats(mr)
            mr.lastStat = now
        }
	}

    staleBlockNum := big.NewInt(0)
    staleBlockNum.Set(self.blockNumber)
    staleBlockNum.Sub(staleBlockNum, big.NewInt(8))
    for key, sub := range(self.solutions) {
        if sub.blockNumber.Cmp(staleBlockNum) < 0 {
            delete(self.solutions, key)
        }
    }

	self.tick = self.tick.Add(time.Duration(dstep) * time.Second)
}

func (self *MinerPool) getHashrate() *big.Int {
	ret := big.NewInt(0)
	for _, mr := range self.miners {
		ret.Add(ret, mr.getTrueHashrate())
	}
	return ret
}

func (self *MinerPool) resetHashcounts() {
	self.stateLock.Lock()
	for _, mr := range self.miners {
		mr.hashes = big.NewInt(0)
	}
	self.stateLock.Unlock()
}

func (self *MinerPool) writeMinerStats(miner *Miner) error {
    dt := time.Since(miner.lastStat)
    miner.onlineTime += dt

    minerStat := &MinerStat{Address: getHexString(miner.address, 40),
                            Hashes: miner.hashes.String(),
                            Payout: miner.payout.String(),
                            OnlineTime: miner.onlineTime,
                            Shares: miner.shares.Uint64(),
                            Blocks: miner.blocks.Uint64()}
    if self.db == nil {
        return errors.New("no database")
    }

    err := self.db.Connect()

    if err != nil {
        log.Println("could not connect to database")
        return err
    }

    log.Println("writing miner stats - ", getHexString(miner.address, 40))

    self.db.Update(minerStat)

    self.db.Disconnect()

    miner.lastStat = time.Now()

    return nil
}

func (self *MinerPool) getMinerStats(miner *Miner) *MinerStat {
    minerStat := &MinerStat{Address: getHexString(miner.address, 40),
                            Hashes: "0",
                            Payout: "0",
                            OnlineTime: 0,
                            Shares: 0,
                            Blocks: 0}

    if self.db == nil {
        return minerStat
    }

    err := self.db.Connect()

    if err != nil {
        log.Println("could not connect to database")
        return minerStat
    }

    self.db.Get(minerStat, getHexString(miner.address, 40))

    self.db.Disconnect()

    log.Println("reading miner stats: ", getHexString(miner.address, 40))

    return minerStat
}

func (self *MinerPool) getMiner(address *big.Int) *Miner {
	minerStr := getHexString(address, 40)
	ret, ok := self.miners[minerStr]

    if !ok {
        self.miners[minerStr] = MinerNew(self, address, self.tick)
        ret = self.miners[minerStr]

        minerStats := self.getMinerStats(ret)
        ret.hashes.SetString(minerStats.Hashes, 10)
        ret.shares = big.NewInt(int64(minerStats.Shares))
        ret.blocks = big.NewInt(int64(minerStats.Blocks))
        ret.payout.SetString(minerStats.Payout, 10)
        ret.onlineTime = minerStats.OnlineTime

        teraHashes := big.NewRat(1,1)
        teraHashes.SetFrac(ret.hashes, big.NewInt(1000000000000))
        fteraHashes, _ := teraHashes.Float64()
        log.Println("new miner joined: ", minerStr)
        log.Println("info: ", ret.onlineTime, " - terahashes: ", fteraHashes, " blocks: ", ret.blocks.String())
    }

	return ret
}

func (self *MinerPool) getTotalHashrate() *big.Int {
	return self.totalHashrate
}

func (self *MinerPool) start(finished chan bool) {
	for !SHUTDOWN {
		time.Sleep(time.Duration(POOL_POLL_TIME) * time.Second)
		pool.update()

		for _, mr := range pool.miners {
            if server != nil {
                pool.lock()
			    server.SubmitHashrate(mr)
                pool.unlock()
            }
		}
	}

    self.destroy()

	finished <- true
}
