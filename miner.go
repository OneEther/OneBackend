package main

//
// manages miner information
//

import "log"
import "math/big"
import "time"


// the queues are no longer used.
// it turns out that it is more accurate to estimate
// hashrate using the current easing technique
const QUEUE_LEN = 8

type AverageQueue struct {
	entries []float64
}

func (self *AverageQueue) push(val float64) {
	if len(self.entries) >= QUEUE_LEN {
		copy(self.entries[:len(self.entries)-1], self.entries[1:len(self.entries)])
		self.entries[QUEUE_LEN-1] = val
	} else {
		self.entries = append(self.entries, val)
	}
}

func (self *AverageQueue) getAverage() float64 {
	var sum float64 = 0
	for i := 0; i < len(self.entries); i++ {
		sum += self.entries[i]
	}

	if len(self.entries) > 0 {
		sum /= float64(len(self.entries))
	}

	return sum
}

type MinerMachine struct {
	id              *big.Int  // id of the machine
	claimedHashrate *big.Int  // hashrate the client claims to have
	lastUpdate      time.Time // last time since hash count update
}

func NewMinerMachine(id, claimedHashrate *big.Int, now time.Time) *MinerMachine {
	return &MinerMachine{id: id, claimedHashrate: claimedHashrate, lastUpdate: now}
}

func (self *MinerMachine) update(dt int64) {
	//hashes := big.NewInt(dt)
	//hashes.Mul(hashes, self.trueHashrate)
	//self.hashes.Add(self.hashes, hashes)
}

type Miner struct {
    owner      *MinerPool
	machines   map[string]*MinerMachine
	address    *big.Int  // Miner address
    payout     *big.Int  // amount we have paid this miner (in wei)
	hashes     *big.Int  // hashes since last commit
	shares     *big.Int  // shares submitted since join (or reset)
    blocks     *big.Int  // mined blocks
	difficulty *big.Int  // current miner difficulty; updates on 'submit'
    onlineTime time.Duration // time this miner has been online (updated each mongo write)
	joinTime   time.Time // time the miner is first seen
    lastStat   time.Time // last time we updated the stats in the database
	lastPost   time.Time // last time since we sent status update to server
	lastSubmit time.Time // time since last valid submit
	hashrate   AverageQueue
    ehashrate float64
}

func MinerNew(owner *MinerPool, address *big.Int, now time.Time) *Miner {
	defaultDifficulty := big.NewInt(DEFAULT_HASHRATE_ESTIMATE)
	defaultDifficulty.Mul(defaultDifficulty, big.NewInt(int64(SHARE_TIME)))

	return &Miner{machines: make(map[string]*MinerMachine),
		address:    address,
        payout:     big.NewInt(0),
		hashes:     big.NewInt(0),
		shares:     big.NewInt(0),
        blocks:     big.NewInt(0),
		difficulty: defaultDifficulty,
        onlineTime: 0,
		joinTime:   now,
        lastStat:   now,
		lastPost:   now,
		lastSubmit: now,
        ehashrate: DEFAULT_HASHRATE_ESTIMATE,
	}
}

func (miner *Miner) GetNewDifficulty() *big.Int {
	defaultHashrate := big.NewInt(DEFAULT_HASHRATE_ESTIMATE)
	newDifficulty := big.NewInt(0)

    claimHashrate := miner.getClaimedHashrate()
	trueHashrate := miner.getTrueHashrate()

    if claimHashrate.Cmp(defaultHashrate) > 0 {
		newDifficulty.Set(claimHashrate)
    } else if trueHashrate.Cmp(defaultHashrate) > 0 {
		newDifficulty.Set(trueHashrate)
	} else {
		newDifficulty.Set(defaultHashrate)
	}
	newDifficulty.Mul(newDifficulty, big.NewInt(int64(SHARE_TIME)))
	return newDifficulty
}

func (m *Miner) removeMachine(key string) {
    delete(m.machines, key)
}

func (m *Miner) Update(dt int64) {
	for key, machine := range m.machines {
		machine.update(dt)

		if pool.tick.Sub(machine.lastUpdate) > time.Second * MACHINE_TIMEOUT {
            log.Println("removing idle machine - ", key)
			m.removeMachine(key)
		}
	}
}

/*
 * used for hashrate calculation and statistics; not payments
 */
func (m *Miner) claimShare(diff, dt, payout float64) {
    clamp := func(a, b, val float64) float64 {
        if val < a { return a }
        if val > b { return b }
        return val
    }

    MIN_TIME := SHARE_TIME * 0.2
    MAX_TIME := SHARE_TIME * 2.0
    hashrate := diff / (2.0 * clamp(MIN_TIME, MAX_TIME, dt))


    DAMPING_FACTOR := 0.3
    m.ehashrate += ((hashrate - m.ehashrate) * DAMPING_FACTOR)

	m.shares.Add(m.shares, big.NewInt(1))
    m.hashes.Add(m.hashes, big.NewInt(int64(diff)))
    m.payout.Add(m.payout, big.NewInt(int64(payout)))
}

func (m *Miner) getMachine(id *big.Int) *MinerMachine {
	idStr := getHexString(id, 32)
    machine, ok := m.machines[idStr]

    if !ok {
        log.Println("new machine joined: ", idStr)
        machine = NewMinerMachine(id, big.NewInt(0), m.lastSubmit)
        m.machines[idStr] = machine
    }

    return machine
}

func (self *Miner) getTotalHashes() *big.Int {
	return self.hashes
}

func (m *Miner) updateMachine(id *big.Int, machine *MinerMachine) {
	idStr := getHexString(id, 32)
	m.machines[idStr] = machine
}

func (m *Miner) getClaimedHashrate() *big.Int {
	ret := big.NewInt(0)
	for _, machine := range m.machines {
		ret.Add(ret, machine.claimedHashrate)
	}
	return ret
}

func (m *Miner) getTrueHashrate() *big.Int {
	// ret := big.NewInt(int64(m.hashrate.getAverage()))
    ret := big.NewInt(int64(m.ehashrate))
	return ret
}

func (m *Miner) getHashes() *big.Int {
	return m.hashes
}

func (self *Miner) getDifficulty() *big.Int {
	return self.difficulty
}
