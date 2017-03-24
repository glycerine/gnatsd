package peer

import (
	"encoding/binary"
	"fmt"
	"os"
	"time"

	"github.com/boltdb/bolt"
)

var BoltDataBucketName = []byte("data") // bucket
var BoltTsBucketName = []byte("ts")     // bucket
var BoltSizeBucketName = []byte("size") // bucket

type BoltSaver struct {
	db       *bolt.DB
	filepath string
	whoami   string
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

	// make the data and ts buckets, if need be.
	return b.db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(BoltDataBucketName)
		if err != nil {
			return fmt.Errorf("boltdb: CreateBucketIfNotExists(boltBucketName='%s') saw error: %s",
				string(BoltDataBucketName), err)
		}

		_, err = tx.CreateBucketIfNotExists(BoltTsBucketName)
		if err != nil {
			return fmt.Errorf("boltdb: CreateBucketIfNotExists(boltBucketName='%s') saw error: %s",
				string(BoltTsBucketName), err)
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
func (b *BoltSaver) LocalSet(ki *KeyInv) error {
	ki.Size = int64(len(ki.Val))

	return b.db.Update(func(tx *bolt.Tx) error {

		// store timestamp
		tsbuck := tx.Bucket(BoltTsBucketName)
		if tsbuck == nil {
			return fmt.Errorf("bucket '%s' does not exist", string(BoltTsBucketName))
		}

		var stamp [8]byte
		nano := ki.When.UnixNano()
		binary.LittleEndian.PutUint64(stamp[:8], uint64(nano))
		err := tsbuck.Put(ki.Key, stamp[:])
		if err != nil {
			return err
		}

		// store size
		sizebuck := tx.Bucket(BoltSizeBucketName)
		if sizebuck == nil {
			return fmt.Errorf("bucket '%s' does not exist", string(BoltSizeBucketName))
		}

		var size [8]byte
		binary.LittleEndian.PutUint64(size[:8], uint64(ki.Size))
		err = sizebuck.Put(ki.Key, size[:])
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

func (b *BoltSaver) LocalGet(key []byte, includeValue bool) (ki *KeyInv, err error) {

	ki = &KeyInv{
		Key: copyBytes(key),
		Who: b.whoami,
	}

	err = b.db.View(func(tx *bolt.Tx) error {

		// get the timestamp
		tsbuck := tx.Bucket(BoltTsBucketName)
		if tsbuck == nil {
			return fmt.Errorf("bucket '%s' does not exist", string(BoltTsBucketName))
		}
		stamp := tsbuck.Get(key)
		if len(stamp) == 8 {
			nano := binary.LittleEndian.Uint64(stamp)
			ki.When = time.Unix(0, int64(nano)).UTC()
		} else {
			return fmt.Errorf("key '%s' in bucket '%s' does not exist", string(key), string(BoltTsBucketName))
		}

		// get the size
		sizebuck := tx.Bucket(BoltSizeBucketName)
		if sizebuck == nil {
			return fmt.Errorf("bucket '%s' does not exist", string(BoltSizeBucketName))
		}
		size := sizebuck.Get(key)
		if len(size) == 8 {
			ki.Size = int64(binary.LittleEndian.Uint64(size))
		} else {
			return fmt.Errorf("key '%s' in bucket '%s' does not exist", string(key), string(BoltSizeBucketName))
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
