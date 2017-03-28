package peer

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/boltdb/bolt"
	"github.com/glycerine/hnatsd/peer/api"
)

var BoltDataBucketName = []byte("data") // bucket
var BoltMetaBucketName = []byte("meta") // bucket
var BoltSizeBucketName = []byte("size") // bucket

type BoltSaver struct {
	db       *bolt.DB
	filepath string
	whoami   string

	// only one op at a time
	mut sync.Mutex
}

func (b *BoltSaver) Close() {
	if b != nil && b.db != nil {
		b.db.Close()
		b.db = nil
	}
}

func NewBoltSaver(filepath string, who string) (*BoltSaver, error) {

	if len(filepath) == 0 {
		return nil, fmt.Errorf("filepath must not be empty string")
	}
	if filepath[0] != '/' && filepath[0] != '.' {
		// help back-compat with old prefix style argument
		filepath = "./" + filepath
	}

	// Open the my.db data file in your current directory.
	// It will be created if it doesn't exist.
	db, err := bolt.Open(filepath, 0600, &bolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		wd, _ := os.Getwd()
		// probably already open by another process.
		return nil, fmt.Errorf("error opening boltdb,"+
			" in use by other process? error detail: '%v' "+
			"upon trying to open path '%s' in cwd '%s'", err, filepath, wd)
	}

	if err != nil {
		return nil, err
	}
	mylog.Printf("NewBoltSaver: BOLTDB opened successfully '%s'", filepath)

	b := &BoltSaver{
		db:       db,
		filepath: filepath,
		whoami:   who,
	}
	err = b.InitDbIfNeeded()
	return b, err
}

func (b *BoltSaver) InitDbIfNeeded() error {
	b.mut.Lock()
	defer b.mut.Unlock()

	// make the data and ts buckets, if need be.
	return b.db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(BoltDataBucketName)
		if err != nil {
			return fmt.Errorf("boltdb: CreateBucketIfNotExists(boltBucketName='%s') saw error: %s",
				string(BoltDataBucketName), err)
		}

		_, err = tx.CreateBucketIfNotExists(BoltMetaBucketName)
		if err != nil {
			return fmt.Errorf("boltdb: CreateBucketIfNotExists(boltBucketName='%s') saw error: %s",
				string(BoltMetaBucketName), err)
		}
		_, err = tx.CreateBucketIfNotExists(BoltSizeBucketName)
		if err != nil {
			return fmt.Errorf("boltdb: CreateBucketIfNotExists(boltBucketName='%s') saw error: %s",
				string(BoltSizeBucketName), err)
		}

		return nil
	})
}

// LocalSet will set ki.Size from len(ki.Val) before
// saving.
func (b *BoltSaver) LocalSet(ki *api.KeyInv) error {
	b.mut.Lock()
	defer b.mut.Unlock()

	ki.Size = int64(len(ki.Val))
	ki.Blake2b = blake2bOfBytes(ki.Val)
	ki.Who = b.whoami

	// meta is a api.KeyInv but without the Val
	meta := *ki
	meta.Val = nil
	metabytes, err := meta.MarshalMsg(nil)
	if err != nil {
		return err
	}

	return b.db.Update(func(tx *bolt.Tx) error {

		// store meta
		metabuck := tx.Bucket(BoltMetaBucketName)
		if metabuck == nil {
			return fmt.Errorf("bucket '%s' does not exist", string(BoltMetaBucketName))
		}

		err := metabuck.Put(ki.Key, metabytes)
		if err != nil {
			return err
		}

		// store data
		databuck := tx.Bucket(BoltDataBucketName)
		if databuck == nil {
			return fmt.Errorf("bucket '%s' does not exist", string(BoltDataBucketName))
		}

		return databuck.Put(ki.Key, ki.Val)
	})

}

func (b *BoltSaver) LocalGet(key []byte, includeValue bool) (ki *api.KeyInv, err error) {
	b.mut.Lock()
	defer b.mut.Unlock()

	ki = &api.KeyInv{
		Key: copyBytes(key),
		Who: b.whoami,
	}

	var meta api.KeyInv

	err = b.db.View(func(tx *bolt.Tx) error {

		// get the meta info
		metabuck := tx.Bucket(BoltMetaBucketName)
		if metabuck == nil {
			return fmt.Errorf("bucket '%s' does not exist", string(BoltMetaBucketName))
		}
		metabytes := metabuck.Get(key)
		if len(metabytes) > 0 {
			_, err := meta.UnmarshalMsg(metabytes)
			if err != nil {
				return err
			}
			ki.When = meta.When.UTC()
			ki.Size = meta.Size
			ki.Blake2b = meta.Blake2b
		} else {
			return fmt.Errorf("key '%s' in bucket '%s' does not exist", string(key), string(BoltMetaBucketName))
		}

		// get the data, if requested
		if includeValue {
			databuck := tx.Bucket(BoltDataBucketName)
			if databuck == nil {
				// bucket does not exist: first time/no snapshot to recover.
				return fmt.Errorf("bucket '%s' does not exist", string(BoltDataBucketName))
			}
			// get the data
			d := databuck.Get(key)
			if len(d) > 0 {
				// must copy since d is only good during
				// this txn.
				ki.Val = copyBytes(d)
			}
		}

		return nil
	})
	return
}

func copyBytes(d []byte) []byte {
	cp := make([]byte, len(d))
	copy(cp, d)
	return cp
}
