package peer

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/boltdb/bolt"
)

// Compact does these steps to reduce the
// space fragmentation that happens in a bolt database.
//
// 0) open a fresh .compressed bolt db file.
// 1) read each object from the bolt db and write it to the fresh -> .compressed bolt db file.
// 2) close the both files.
// 3) rename the .compressed file to be the original db file name.
//    os.Rename is atomic.
// 4) re-open the newly compact-ed db file
//
// The routines below were adapted from the compaction code in
// https://github.com/boltdb/bolt/blob/master/cmd/bolt/main.go
// which is used under the following MIT license.
/*
The MIT License (MIT)

Copyright (c) 2013 Ben Johnson

Permission is hereby granted, free of charge, to any person obtaining a copy of
this software and associated documentation files (the "Software"), to deal in
the Software without restriction, including without limitation the rights to
use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
the Software, and to permit persons to whom the Software is furnished to do so,
subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
*/
// INVAR: b.db must be already open.
func (b *BoltSaver) Compact(lockNeeded bool) error {

	if lockNeeded {
		b.mut.Lock()
		defer b.mut.Unlock()
	}

	// Ensure source file exists.
	fi, err := os.Stat(b.filepath)
	if os.IsNotExist(err) {
		return fmt.Errorf("file not found '%s'", b.filepath)
	} else if err != nil {
		return err
	}

	dstPath := b.filepath + ".compressed"
	os.Remove(dstPath)

	// Open destination database.
	dst, err := bolt.Open(dstPath, fi.Mode(), nil)
	if err != nil {
		return err
	}
	defer dst.Close()

	// Run compaction.
	if err := b.compactInBatches(dst, b.db); err != nil {
		return err
	}

	// Report stats on new size.
	fi2, err := os.Stat(dstPath)
	if err != nil {
		return err
	} else if fi2.Size() == 0 {
		return fmt.Errorf("zero db size")
	}
	log.Printf("BoltSaver.Compact() did: %d -> %d bytes (gain=%.2fx)\n", fi.Size(), fi2.Size(), float64(fi.Size())/float64(fi2.Size()))

	dst.Close()
	b.Close()

	// now move into place the compacted file.
	err = os.Rename(dstPath, b.filepath)
	if err != nil {
		err2 := b.reOpen()
		err = fmt.Errorf("error in BoltSaver.Compact() on os.Rename from '%s' to '%s' got err: '%v'", dstPath, b.filepath, err)
		if err2 != nil {
			err = fmt.Errorf("%s. And: %s", err.Error(), err2.Error())
		}
		return err
	}
	return b.reOpen()
}

func (b *BoltSaver) reOpen() error {
	db, err := bolt.Open(b.filepath, 0600, &bolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		wd, _ := os.Getwd()
		// probably already open by another process.
		return fmt.Errorf("error opening boltdb,"+
			" in use by other process? error detail: '%v' "+
			"upon trying to open path '%s' in cwd '%s'", err, b.filepath, wd)
	}
	b.db = db
	return nil
}

func (b *BoltSaver) compactInBatches(dst, src *bolt.DB) error {
	// commit regularly, or we'll run out of memory for large datasets if using one transaction.
	var size int64
	tx, err := dst.Begin(true)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := b.walk(src, func(keys [][]byte, k, v []byte, seq uint64) error {
		// On each key/value, check if we have exceeded tx size.
		sz := int64(len(k) + len(v))
		if size+sz > b.compactTxMaxSizeBytes && b.compactTxMaxSizeBytes != 0 {
			// Commit previous transaction.
			if err := tx.Commit(); err != nil {
				return err
			}

			// Start new transaction.
			tx, err = dst.Begin(true)
			if err != nil {
				return err
			}
			size = 0
		}
		size += sz

		// Create bucket on the root transaction if this is the first level.
		nk := len(keys)
		if nk == 0 {
			bkt, err := tx.CreateBucket(k)
			if err != nil {
				return err
			}
			if err := bkt.SetSequence(seq); err != nil {
				return err
			}
			return nil
		}

		// Create buckets on subsequent levels, if necessary.
		b := tx.Bucket(keys[0])
		if nk > 1 {
			for _, k := range keys[1:] {
				b = b.Bucket(k)
			}
		}

		// If there is no value then this is a bucket call.
		if v == nil {
			bkt, err := b.CreateBucket(k)
			if err != nil {
				return err
			}
			if err := bkt.SetSequence(seq); err != nil {
				return err
			}
			return nil
		}

		// Otherwise treat it as a key/value pair.
		return b.Put(k, v)
	}); err != nil {
		return err
	}

	return tx.Commit()
}

// walkFunc is the type of the function called for keys (buckets and "normal"
// values) discovered by Walk. keys is the list of keys to descend to the bucket
// owning the discovered key/value pair k/v.
type walkFunc func(keys [][]byte, k, v []byte, seq uint64) error

// walk walks recursively the bolt database db, calling walkFn for each key it finds.
func (bs *BoltSaver) walk(db *bolt.DB, walkFn walkFunc) error {
	return db.View(func(tx *bolt.Tx) error {
		return tx.ForEach(func(name []byte, buck *bolt.Bucket) error {
			return bs.walkBucket(buck, nil, name, nil, buck.Sequence(), walkFn)
		})
	})
}

func (bs *BoltSaver) walkBucket(buck *bolt.Bucket, keypath [][]byte, k, v []byte, seq uint64, fn walkFunc) error {
	// Execute callback.
	if err := fn(keypath, k, v, seq); err != nil {
		return err
	}

	// If this is not a bucket then stop.
	if v != nil {
		return nil
	}

	// Iterate over each child key/value.
	keypath = append(keypath, k)
	return buck.ForEach(func(k, v []byte) error {
		if v == nil {
			bkt := buck.Bucket(k)
			return bs.walkBucket(bkt, keypath, k, nil, bkt.Sequence(), fn)
		}
		return bs.walkBucket(buck, keypath, k, v, buck.Sequence(), fn)
	})
}
