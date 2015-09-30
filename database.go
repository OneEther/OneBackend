package main

//
// package for database callback on eth block events
// (new block)
// you might also want to view 'mongo.go', which has *actual* database stuff
//

import "log"

type DatabaseBlockProcessor struct {
	db    Database
	cache map[string]*Account
}

func NewDatabaseBlockProcessor(db Database) *DatabaseBlockProcessor {
	return &DatabaseBlockProcessor{db: db, cache: make(map[string]*Account)}
}

func (self *DatabaseBlockProcessor) BeginProcessing() error {
	err := self.db.Connect()

	if err == nil {
		self.db.DropTable("pending_blocks")
		self.db.DropTable("pending_transactions")
		self.db.DropTable("pending_accounts")
	}

	return err
}

func (self *DatabaseBlockProcessor) EndProcessing() error {
	self.dumpCache()
	return self.db.Disconnect()
}

func (self *DatabaseBlockProcessor) Commit() error {
	return self.dumpCache()
}

func (self *DatabaseBlockProcessor) retrieveAccount(address string) *Account {
	account := &Account{}
	err := self.db.GetFrom("pending_accounts", account, address)

	if err != nil {
		account = NewAccount(address)
	}

	return account
}

func (self *DatabaseBlockProcessor) getAccount(address string) *Account {
	account, ok := self.cache[address]

	if ok && account != nil {
		return account
	}

	account = &Account{}
	err := self.db.Get(account, address)

	if err != nil {
		account = NewAccount(address)
	}

	self.cache[address] = account
	return account
}

func (self *DatabaseBlockProcessor) dumpCache() error {
	for _, account := range self.cache {
        if len(account.Mined) > 200 {
            account.Mined = account.Mined[len(account.Mined)-200:len(account.Mined)]
        }

        if len(account.Incoming) > 200 {
            account.Incoming = account.Incoming[len(account.Incoming)-200:len(account.Incoming)]
        }

        if len(account.Outgoing) > 200 {
            account.Outgoing = account.Outgoing[len(account.Outgoing)-200:len(account.Outgoing)]
        }

		err := self.db.Update(account)

		if err != nil {
			log.Printf("could not update miner in db: " + err.Error())
            log.Println(len(account.Incoming), len(account.Outgoing), len(account.Mined))
			panic(err)
		}
	}
	self.cache = make(map[string]*Account)
	return nil
}

func (self *DatabaseBlockProcessor) updateBlockMiner(block *Block) {
	minerAccount := self.getAccount(block.Miner)
	minerAccount.Mined = append(minerAccount.Mined, block.Number)
}

func (self *DatabaseBlockProcessor) updateTransaction(txn *Transaction) {
	err := self.db.Add(txn)

	if err != nil {
		log.Printf("error adding transaction to db: " + err.Error())
	}

	fromAccount := self.getAccount(txn.From)
	fromAccount.Outgoing = append(fromAccount.Outgoing, txn)

	toAccount := self.getAccount(txn.To)
	toAccount.Incoming = append(toAccount.Incoming, txn)
}

func (self *DatabaseBlockProcessor) AddPendingBlock(block *Block) error {
	self.db.AddTo("pending_blocks", block)

	minerAccount := self.retrieveAccount(block.Miner)
	minerAccount.Mined = append(minerAccount.Mined, block.Number)
	self.db.UpdateTo("pending_accounts", minerAccount)

	for _, txn := range block.Transactions {
		err := self.db.AddTo("pending_transactions", txn)

		if err != nil {
			log.Printf("error adding transaction to db: " + err.Error())
		}

		fromAccount := self.retrieveAccount(txn.From)
		fromAccount.Outgoing = append(fromAccount.Outgoing, txn)
		err = self.db.UpdateTo("pending_accounts", fromAccount)

		if err != nil {
			log.Printf("error adding 'from' account to db: " + err.Error())
		}

		toAccount := self.retrieveAccount(txn.To)
		toAccount.Incoming = append(toAccount.Incoming, txn)
		err = self.db.UpdateTo("pending_accounts", toAccount)

		if err != nil {
			log.Printf("error adding 'to' transaction to db: " + err.Error())
		}
	}
	return nil
}

func (self *DatabaseBlockProcessor) AddBlock(block *Block) {
	self.db.Add(block)

	self.updateBlockMiner(block)

	for _, txn := range block.Transactions {
		self.updateTransaction(txn)
	}
}

/**
 *
 */

type DatabasePaymentProcessor struct {
    db Database
}

func NewDatabasePaymentProcessor(db Database) *DatabasePaymentProcessor {
    return &DatabasePaymentProcessor{db}
}

func (*DatabasePaymentProcessor) PaymentAdded(*PendingTransaction) {
}

func (*DatabasePaymentProcessor) PaymentSent(*PendingTransaction) {
}

func (*DatabasePaymentProcessor) PaymentResent(*PendingTransaction) {
}

func (self *DatabasePaymentProcessor) PaymentVerified(txn *PendingTransaction) {
    self.db.Connect()
    self.db.AddTo("verified_payments", txn.Transaction)
    self.db.Disconnect()
}
