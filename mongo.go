package main

//
// mongo specific database functionality
//

import "reflect"
import "fmt"
import "gopkg.in/mgo.v2"
import "gopkg.in/mgo.v2/bson"
import "errors"
import "sync"
import "strings"

type SimpleStorage interface {
	Add(interface{}) error
	Remove(interface{}) error
	Update(interface{}) error
	Exists(interface{}) bool
	Get(interface{}, string) error
	DropTable(string) error
}

type TableStorage interface {
	AddTo(string, interface{}) error
	RemoveFrom(string, interface{}) error
	UpdateTo(string, interface{}) error
	ExistsIn(string, interface{}) bool
	GetFrom(string, interface{}, string) error
}

type Database interface {
	Connect() error
	Disconnect() error
	SimpleStorage
	TableStorage
	//UpdateArray(string, interface{}) error
}

type Mongo struct {
	session  *mgo.Session
	lock     *sync.Mutex
	refcount int
	ip       string
	db_id    string
}

func NewMongoDB(db string) *Mongo {
	return &Mongo{session: nil, lock: &sync.Mutex{}, refcount: 0, ip: "localhost", db_id: db}
}

func (self *Mongo) Connect() error {
	var err error

	self.lock.Lock()
	defer self.lock.Unlock()

	if self.refcount <= 0 {
		self.session, err = mgo.Dial(self.ip)

		if err != nil {
			return err
		}

		if self.session == nil {
			return errors.New("could not connect to mongo database")
		}

		// Optional. Switch the mongo to a monotonic behavior.
		self.session.SetMode(mgo.Monotonic, true)
	}

	self.refcount++

	return nil
}

func (self *Mongo) Disconnect() error {
	self.lock.Lock()
	defer self.lock.Unlock()

	self.refcount--

	if self.refcount <= 0 {
		self.session.Close()
		self.session = nil
	}

	return nil
}

func (self *Mongo) DropTable(table string) error {
	c := self.getCollection(table)
	return c.DropCollection()
}

func (self *Mongo) getCollection(table string) *mgo.Collection {
	c := self.session.DB(self.db_id).C(table)

	switch table {
	case "transactions":
		idx := mgo.Index{Key: []string{"$text:hash"}}
		c.EnsureIndex(idx)
	case "blocks":
		idx := mgo.Index{Key: []string{"$text:hash"}}
		c.EnsureIndex(idx)
	case "accounts":
		idx := mgo.Index{Key: []string{"$text:address"}}
		c.EnsureIndex(idx)
	}

	return c
}

func (self *Mongo) getTypeBaseName(item interface{}) string {
    rv := reflect.ValueOf(item)

    switch(rv.Kind()) {
    case reflect.Ptr:
        rv = rv.Elem()
    }

    return strings.ToLower(rv.Type().Name())
}

func (self *Mongo) getCollectionForType(item interface{}) (*mgo.Collection, error) {
	var c *mgo.Collection
	switch item.(type) {
	case *Transaction:
		c = self.session.DB(self.db_id).C("transactions")
		idx := mgo.Index{Key: []string{"$text:hash"}}
		c.EnsureIndex(idx)
	case *Block:
		c = self.session.DB(MONGO_DB_ID).C("blocks")
		idx := mgo.Index{Key: []string{"$text:hash"}}
		c.EnsureIndex(idx)
	case *Account:
		c = self.session.DB(MONGO_DB_ID).C("accounts")
		idx := mgo.Index{Key: []string{"$text:address"}}
		c.EnsureIndex(idx)
	default:
        // default, create a table with the type name + s
        // example struct MyStruct -> mystructs
        typename := self.getTypeBaseName(item) + "s"
		c = self.session.DB(MONGO_DB_ID).C(typename)
	}
	return c, nil
}

func (self *Mongo) AddTo(table string, item interface{}) error {
	c := self.getCollection(table)
	return c.Insert(item)
}

func (self *Mongo) Add(item interface{}) error {
	c, err := self.getCollectionForType(item)

	if err != nil {
		return err
	}

	err = c.Insert(item)

	return err
}

func (self *Mongo) RemoveFrom(table string, item interface{}) error {
	var err error = nil

	c := self.getCollection(table)

	switch item.(type) {
	case *Transaction:
		err = c.Remove(bson.M{"hash": item.(*Transaction).Hash})
	case *Block:
		err = c.Remove(bson.M{"hash": item.(*Block).Hash})
	case *Account:
		err = c.Remove(bson.M{"address": item.(*Account).Address})
	case *MinerStat:
		err = c.Remove(bson.M{"address": item.(*MinerStat).Address})
	}

	return err
}

func (self *Mongo) Remove(item interface{}) error {
	c, err := self.getCollectionForType(item)

	switch item.(type) {
	case *Transaction:
		err = c.Remove(bson.M{"hash": item.(*Transaction).Hash})
	case *Block:
		err = c.Remove(bson.M{"hash": item.(*Block).Hash})
	case *Account:
		err = c.Remove(bson.M{"address": item.(*Account).Address})
	case *MinerStat:
		err = c.Remove(bson.M{"address": item.(*MinerStat).Address})
	}

	return err
}

func (self *Mongo) GetFrom(table string, result interface{}, key string) error {
	rv := reflect.ValueOf(result)

	if rv.Kind() != reflect.Ptr {
		return errors.New("expected pointer")
	}

	c := self.getCollection(table)

	var query *mgo.Query = nil

	switch result.(type) {
	case *Transaction:
		query = c.Find(bson.M{"hash": key})
	case *Block:
		query = c.Find(bson.M{"hash": key})
	case *Account:
		query = c.Find(bson.M{"address": key})
    case *MinerStat:
		query = c.Find(bson.M{"address": key})
	}

	n, err := query.Count()

	if n <= 0 || err != nil {
		return errors.New("item not found")
	}

	query.One(result)

	return nil
}

func (self *Mongo) Get(result interface{}, key string) error {
	rv := reflect.ValueOf(result)

	if rv.Kind() != reflect.Ptr {
		return errors.New("expected pointer")
	}

	c, err := self.getCollectionForType(result)

	if err != nil {
		return err
	}

	var query *mgo.Query = nil

	switch result.(type) {
	case *Transaction:
		query = c.Find(bson.M{"hash": key})
	case *Block:
		query = c.Find(bson.M{"hash": key})
	case *Account:
		query = c.Find(bson.M{"address": key})
    case *MinerStat:
		query = c.Find(bson.M{"address": key})
	}

	n, err := query.Count()

	if n <= 0 || err != nil {
		return errors.New("item not found")
	}

	err = query.One(result)

	return err
}

func (self *Mongo) ExistsIn(table string, item interface{}) bool {
	var err error = nil
	c := self.getCollection(table)

	if err != nil {
		return false
	}

	var query *mgo.Query

	switch item.(type) {
	case *Transaction:
		query = c.Find(bson.M{"hash": item.(*Transaction).Hash})
	case *Block:
		query = c.Find(bson.M{"hash": item.(*Block).Hash})
	case *Account:
		query = c.Find(bson.M{"address": item.(*Account).Address})
    case *MinerStat:
		query = c.Find(bson.M{"address": item.(*MinerStat).Address})
	}

	num, err := query.Count()

	return err == nil && num > 0
}

func (self *Mongo) Exists(item interface{}) bool {
	c, err := self.getCollectionForType(item)

	if err != nil {
		return false
	}

	var query *mgo.Query

	switch item.(type) {
	case *Transaction:
		query = c.Find(bson.M{"hash": item.(*Transaction).Hash})
	case *Block:
		query = c.Find(bson.M{"hash": item.(*Block).Hash})
	case *Account:
		query = c.Find(bson.M{"address": item.(*Account).Address})
	case *MinerStat:
		query = c.Find(bson.M{"address": item.(*MinerStat).Address})
	}

	num, err := query.Count()

	return err == nil && num > 0
}

func (self *Mongo) UpdateTo(table string, item interface{}) error {
	var err error = nil
	c := self.getCollection(table)

	switch item.(type) {
	case *Transaction:
		_, err = c.Upsert(bson.M{"hash": item.(*Transaction).Hash}, bson.M{"$set": item})
	case *Block:
		_, err = c.Upsert(bson.M{"hash": item.(*Block).Hash}, bson.M{"$set": item})
	case *Account:
		_, err = c.Upsert(bson.M{"address": item.(*Account).Address}, bson.M{"$set": item})
	case *MinerStat:
		_, err = c.Upsert(bson.M{"address": item.(*MinerStat).Address}, bson.M{"$set": item})
	}

	if err != nil {
		fmt.Println(err)
	}

	return err
}

func (self *Mongo) Update(item interface{}) error {
	c, err := self.getCollectionForType(item)

	if err != nil {
		return err
	}

	switch item.(type) {
	case *Transaction:
		_, err = c.Upsert(bson.M{"hash": item.(*Transaction).Hash}, bson.M{"$set": item})
	case *Block:
		_, err = c.Upsert(bson.M{"hash": item.(*Block).Hash}, bson.M{"$set": item})
	case *Account:
		_, err = c.Upsert(bson.M{"address": item.(*Account).Address}, bson.M{"$set": item})
	case *MinerStat:
		_, err = c.Upsert(bson.M{"address": item.(*MinerStat).Address}, bson.M{"$set": item})
	}

	if err != nil {
		fmt.Println(err)
	}

	return err
}
