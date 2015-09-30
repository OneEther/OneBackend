package main

//
// chain explorer and callback system
//

import "log"
import "time"
import "math/big"
import "os"

type StatusPoll struct {
	eth                EthAll
	lastProcessedBlock int64
	persist            *FilePersistence
	blockProcessors    []BlockProcessor
}

type BlockProcessor interface {
	BeginProcessing() error
	AddBlock(*Block)
	Commit() error
	EndProcessing() error
}

type PendingBlockProcessor interface {
	AddPendingBlock(*Block) error
}

func NewStatusPoll(eth EthAll, persistFilename string) *StatusPoll {
	self := &StatusPoll{eth: eth}

	self.persist = NewFilePersistence(persistFilename)
	if _, err := os.Stat(persistFilename); os.IsNotExist(err) {
		self.lastProcessedBlock = int64(MIN_PROCESSED_BLOCK)
		self.persist.Write(self.lastProcessedBlock)
	} else {
		self.persist.Read(&self.lastProcessedBlock)
		log.Printf("loaded block persistence: %d\n", self.lastProcessedBlock)
	}

	return self
}

/*
 * like get balance, but should not return unless it succeeds
 */
func GetInitialBalance(eth EthWallet) *big.Int {
	err := error(nil)

RETRY:
	ret, err := eth.GetBalance()

	if err != nil {
		log.Printf("ERROR unable to get initial balance: " + err.Error() + "\n")
		time.Sleep(2 * time.Second)
		goto RETRY
	}

	return ret
}

func (self *StatusPoll) RegisterBlockProcessor(p BlockProcessor) {
	self.blockProcessors = append(self.blockProcessors, p)
}

func (self *StatusPoll) Commit() error {
	for _, proc := range self.blockProcessors {
		err := proc.Commit()

		if err != nil {
            log.Println(err)
			//panic(err)
		}
	}

	self.persist.Write(&self.lastProcessedBlock)

	return nil
}

func (self *StatusPoll) Start(finished chan bool) {
	balance := GetInitialBalance(self.eth)

	for !SHUTDOWN {
		self.updateNewBlocks()

		if !SHUTDOWN {
			balance, _ = self.eth.GetBalance()
			log.Println("BALANCE: " + balance.String() + "\n")
			time.Sleep(time.Duration(BALANCE_POLL_TIME) * time.Second)
		}
	}

	log.Println("closed status poll")
	if finished != nil {
		finished <- true
	}
}

func (self *StatusPoll) updateNewBlocks() {
	num, err := self.eth.GetBlockNumber()

	if err != nil {
		log.Printf("could not get current block number: " + err.Error())
		return
	}

	blockNumber := num.Int64()
	confirmedBlockNumber := blockNumber - 8
	pendingBlockNumber := blockNumber
	// Only process up to the last 8 blocks (to avoid a mess with uncles)

	for _, proc := range self.blockProcessors {
		err := proc.BeginProcessing()

		if err != nil {
			log.Printf("error beginning block processing: " + err.Error())
			log.Printf("skipping block update")
			return
		}
	}

	for self.lastProcessedBlock < confirmedBlockNumber && !SHUTDOWN {
		block := self.eth.GetBlockByNumber(big.NewInt(self.lastProcessedBlock), true)

		if confirmedBlockNumber-self.lastProcessedBlock > 10 {
			if self.lastProcessedBlock%500 == 0 {
				log.Printf("processing block: %d\n", self.lastProcessedBlock)
			}
		} else {
			log.Printf("processing block: %d\n", self.lastProcessedBlock)
		}

		for _, proc := range self.blockProcessors {
			proc.AddBlock(block)
		}

		if self.lastProcessedBlock%1000 == 0 {
			self.Commit()
		}

		self.lastProcessedBlock++
	}

	for pendingIt := self.lastProcessedBlock; pendingIt < pendingBlockNumber; pendingIt++ {
		block := self.eth.GetBlockByNumber(big.NewInt(pendingIt), true)

		for _, proc := range self.blockProcessors {
			if pproc, ok := proc.(PendingBlockProcessor); ok {
				pproc.AddPendingBlock(block)
			}
		}
	}

	self.Commit()
	for _, proc := range self.blockProcessors {

		err = proc.EndProcessing()
	}
}
