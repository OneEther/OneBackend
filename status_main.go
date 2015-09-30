package main

import "fmt"

func status_main() {
	geth := NewGeth(GETH_IP, GETH_PORT)
	statusPoll := NewStatusPoll(geth, BLOCK_PERSIST_FILENAME)
	statusPoll.Start(nil)
}

func main() {
	fmt.Println("starting chain crawling")
	status_main()
}
