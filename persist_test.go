package main

import "testing"

func TestPersistence(t *testing.T) {
	persp := NewFilePersistence("unittest.bak")

	var in int64
	in = 10000

	persp.Write(in)
	var out int64
	persp.Read(&out)

	if in != out {
		t.Fail()
	}
}
