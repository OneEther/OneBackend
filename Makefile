sharefiles=config.go eth.go mongo.go pay.go persist.go rpc.go settings.go status.go utils.go database.go web.go server.go
poolfiles=miner.go pool.go
testfiles=pay_test.go status_test.go eth_test.go miner_test.go

#go build -o echoPay $(sharefiles) pay_main.go
#go build -o echoChain $(sharefiles) status_main.go

all:
	go build -o echo $(sharefiles) $(poolfiles) main.go

test:
	go test $(sharefiles) $(testfiles) $(poolfiles) main.go

clean:
	rm echo
	rm echoPay
	rm echoChain
