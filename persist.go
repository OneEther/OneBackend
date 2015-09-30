package main

//
// persistence using a simple file
// used to keep track of last processed block
//

import "encoding/json"
import "io/ioutil"
import "sync"

type FilePersistence struct {
	filename string
	lock     *sync.Mutex
}

type Persistence interface {
	Write(data interface{}) error
	Read(data interface{}) error
}

func NewFilePersistence(filename string) *FilePersistence {
	return &FilePersistence{filename, &sync.Mutex{}}
}

func (self *FilePersistence) Write(data interface{}) {
	bytes, err := json.Marshal(data)

	if err != nil {
		panic(err)
	}

	self.lock.Lock()
	ioutil.WriteFile(self.filename, bytes, 0644)
	self.lock.Unlock()
}

func (self *FilePersistence) Read(out interface{}) error {
	self.lock.Lock()
	bytes, err := ioutil.ReadFile(self.filename)
	self.lock.Unlock()

	json.Unmarshal(bytes, out)
	return err
}
