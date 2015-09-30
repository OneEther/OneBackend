# One. Backend

## The simple, yet powerful, ethereum backend

### Requirements

* Linux
* Geth
* MongoDB
* Mgo - mongo database driver for Go
* github.com/satori/go.uuid - uuid library for Go

### Building

Run:

    make

### Running

Run:
    ./run.sh

By default, this will run the block chain explorer, payment processor and pool.

### How it works

One Backend includes the following main threads:

* scanner: a block chain scanner that informs listeners on change of state (a
  new block) this periodically queries geth with the current block number and
  processes the chain until it is up to date. It then stores the last processed
  block in a persistant file.

* payments: a payment processor that takes in payments via RPC, and confirms
  they payment goes through.  this periodically checks if a transaction had
  really been sent. If it had not been sent after 8 blocks (a safe limit for
  chain reorganization), then the processor resends the transaction. This uses a
  persistence file to store pending transactions and mongo to store sent
  transactions.

* web: a thread to periodically update the web backend with miner and pool
  information.

* pool: a web service to listen to incoming miner connections and provide
  ethereum block shares for proof of work and update miner statistics.

* verify: an external RPC service that takes share information and verifies that
  the given share is valid. Currently implemented via a modified py-ethereum.

### License

All code in this repository is licensed under the MIT open source license.
