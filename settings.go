package main

//
// global settings
//

var SHUTDOWN = false

var LISTEN_PORT = "8080"
var GETH_IP = "127.0.0.1"
var GETH_PORT = "8545"
var CONFIRM_ADDR = "http://127.0.0.1:8081"

var BACKEND_IP = "oneether.com"
var BACKEND_PORT = "9999"

const MACHINE_TIMEOUT = 300.0
const CLIENT_TIMEOUT = 25.0
const CLIENT_DB_WRITEBACK = 151.0
const SERVER_UPDATETIME = 9.0
const BALANCE_POLL_TIME = 5.0
const POOL_POLL_TIME = 3.0

const DEFAULT_HASHRATE_ESTIMATE = 80000
const MIN_PROCESSED_BLOCK = 0

const SHARE_TIME = 53.0

const HOUSE_RAKE = 0.02

// DEBUG
var debugRequest = false
var debugRPC = false
var debugDivvy = false
var debugServer = false
var debugVerify = false

// CONSTANTS
var weiToFinney = "1000000000000000"
var weiToEth = "1000000000000000000"

// BLOCK
var BLOCK_PERSIST_FILENAME = "block.last"

// PAY
var PAY_PERSIST_FILENAME = "pending.persist" // i think we use mongo now
var PAY_COMPLETE_FILENAME = "complete.persist"
var PAY_WAIT = 10.0
var PAY_RPC_PORT = "9090"

// MONGO
var MONGO_DB_ID = "one"
