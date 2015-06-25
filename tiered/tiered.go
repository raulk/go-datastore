package tiered

import (
	"fmt"
	"sync"

	ds "github.com/jbenet/go-datastore"
	dsq "github.com/jbenet/go-datastore/query"
)

type tiered []ds.Datastore

// New returns a tiered datastore. Puts and Deletes will write-through to
// all datastores, Has and Get will try each datastore sequentially, and
// Query will always try the last one (most complete) first.
func New(dses ...ds.Datastore) ds.Datastore {
	return tiered(dses)
}

// Put stores the object `value` named by `key`.
func (d tiered) Put(key ds.Key, value interface{}) (err error) {
	errs := make(chan error, len(d))

	var wg sync.WaitGroup
	for _, cd := range d {
		wg.Add(1)
		go func(cd ds.Datastore) {
			defer wg.Done()
			if err := cd.Put(key, value); err != nil {
				errs <- err
			}
		}(cd)
	}
	wg.Wait()

	close(errs)
	for err := range errs {
		return err
	}
	return nil
}

// Get retrieves the object `value` named by `key`.
func (d tiered) Get(key ds.Key) (value interface{}, err error) {
	err = fmt.Errorf("no datastores")
	for _, cd := range d {
		value, err = cd.Get(key)
		if err == nil {
			break
		}
	}
	return
}

// Has returns whether the `key` is mapped to a `value`.
func (d tiered) Has(key ds.Key) (exists bool, err error) {
	err = fmt.Errorf("no datastores")
	for _, cd := range d {
		exists, err = cd.Has(key)
		if err == nil && exists {
			break
		}
	}
	return
}

// Delete removes the value for given `key`.
func (d tiered) Delete(key ds.Key) (err error) {
	errs := make(chan error, len(d))

	var wg sync.WaitGroup
	for _, cd := range d {
		wg.Add(1)
		go func(cd ds.Datastore) {
			defer wg.Done()
			if err := cd.Delete(key); err != nil {
				errs <- err
			}
		}(cd)
	}
	wg.Wait()

	close(errs)
	for err := range errs {
		return err
	}
	return nil
}

// Query returns a list of keys in the datastore
func (d tiered) Query(q dsq.Query) (dsq.Results, error) {
	// query always the last (most complete) one
	return d[len(d)-1].Query(q)
}

type tieredTransaction []ds.Transaction

func (d tiered) StartBatchOp() ds.Transaction {
	var out tieredTransaction
	for _, ds := range d {
		out = append(out, ds.StartBatchOp())
	}
	return out
}

func (t tieredTransaction) Put(key ds.Key, val interface{}) error {
	for _, ts := range t {
		err := ts.Put(key, val)
		if err != nil {
			return err
		}
	}
	return nil
}

func (t tieredTransaction) Delete(key ds.Key) error {
	for _, ts := range t {
		err := ts.Delete(key)
		if err != nil {
			return err
		}
	}
	return nil
}

func (t tieredTransaction) Commit() error {
	for _, ts := range t {
		err := ts.Commit()
		if err != nil {
			return err
		}
	}
	return nil
}
