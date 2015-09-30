package main

import "testing"
import "time"
import "math/big"

func TestQueue (t *testing.T) {
    queue := AverageQueue{}

    queue.push(1)
    queue.push(1)
    queue.push(1)
    queue.push(1)

    if queue.getAverage() - 1 > 0.05 {
        t.Error("expected average of 1, found ", queue.getAverage())
    }

    queue.push(3)
    queue.push(3)
    queue.push(3)
    queue.push(3)

    if queue.getAverage() - 2 > 0.05 {
        t.Error("expected average of 2, found ", queue.getAverage())
    }

    queue.push(100)
    queue.push(100)
    queue.push(100)
    queue.push(100)
    queue.push(100)
    queue.push(100)
    queue.push(100)
    queue.push(100)
    queue.push(100)
    queue.push(100)
    queue.push(100)
    queue.push(100)
    queue.push(100)
    queue.push(100)
    queue.push(100)
    queue.push(100)
    queue.push(100)
    queue.push(100)
    queue.push(100)
    queue.push(100)

    if queue.getAverage() - 100 > 0.05 {
        t.Error("expected average 100, found ", queue.getAverage())
    }
}

func TestClaimedHashrate(t *testing.T) {
    miner := MinerNew(big.NewInt(1), time.Now())
    m1 := miner.getMachine(big.NewInt(1))
    m2 := miner.getMachine(big.NewInt(2))

    m1.claimedHashrate.Set(big.NewInt(10))
    m2.claimedHashrate.Set(big.NewInt(10))

    if miner.getClaimedHashrate().Cmp(big.NewInt(20)) != 0 {
        t.Error("expected 20 claimed hashrate, found ", miner.getClaimedHashrate())
    }
}

func TestTrueHashrate(t *testing.T) {
    miner := MinerNew(big.NewInt(1), time.Now())
    miner.hashrate.push(10)
    miner.hashrate.push(10)
    miner.hashrate.push(10)

    if miner.getTrueHashrate().Cmp(big.NewInt(10)) != 0 {
        t.Error("expected true hashrate of 1, found ", miner.getTrueHashrate)
    }

    miner.hashrate.push(30)
    miner.hashrate.push(30)
    miner.hashrate.push(30)

    if miner.getTrueHashrate().Cmp(big.NewInt(20)) != 0 {
        t.Error("expected true hashrate of 1, found ", miner.getTrueHashrate)
    }

    miner.hashrate.push(20)
    miner.hashrate.push(10)
    miner.hashrate.push(20)
    miner.hashrate.push(10)

    if miner.getTrueHashrate().Cmp(big.NewInt(20)) != 0 {
        t.Error("expected true hashrate of 1, found ", miner.getTrueHashrate)
    }
}

func TestGetNewDifficulty(t *testing.T) {
    defaultDifficulty := big.NewInt(DEFAULT_HASHRATE_ESTIMATE)
    defaultDifficulty.Mul(defaultDifficulty, big.NewInt(SHARE_TIME))

    miner := MinerNew(big.NewInt(1), time.Now())
    d1 := miner.GetNewDifficulty()

    if d1.Cmp(defaultDifficulty) != 0 {
        t.Error("expected default difficulty, found ", d1.String())
    }
}
