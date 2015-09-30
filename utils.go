package main

import "math/big"
import "fmt"
import "strings"
import "strconv"

func parseHex(input string, maxlen int) (*big.Int, error) {
	trimmed := input

	if strings.HasPrefix(input, "0x") {
		trimmed = input[2:]
	}

	ret := big.NewInt(0)

	ret.SetString(trimmed, 16)

	return ret, nil
}

/*
func (self *big.Int) ToHexString(length int) string {
	return fmt.Sprintf("0x%."+strconv.Itoa(length)+"x", self)
}
*/

func getHexString(num *big.Int, length int) string {
	return fmt.Sprintf("0x%."+strconv.Itoa(length)+"x", num)
}
