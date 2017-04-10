package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/boltdb/bolt"
	"github.com/glycerine/hnatsd/peer"
	"github.com/glycerine/hnatsd/peer/api"
)

func main() {
	fl := flag.NewFlagSet("viewdb", flag.ExitOnError)
	var cfg ViewBoltConfig
	cfg.DefineFlags(fl)
	err := fl.Parse(os.Args[1:])
	panicOn(err)
	err = cfg.ValidateConfig()
	panicOn(err)

	cfg.Bopt = &bolt.Options{Timeout: 1 * time.Second, ReadOnly: true}
	cfg.Db, err = bolt.Open(cfg.DbPath, 0600, cfg.Bopt)
	if err != nil {
		if err.Error() == "timeout" {
			fmt.Fprintf(os.Stderr, "file open timedout, probably locked by another process.\n")
			os.Exit(1)
		} else {
			panic(err)
		}
	}
	cfg.Dump(cfg.Db, os.Stdout)
}

var ErrStopLooping = fmt.Errorf("found key, can stop looping")

func (cfg *ViewBoltConfig) Dump(db *bolt.DB, w io.Writer) error {
	indent := "    "

	if cfg.DumpKey == "" {
		fmt.Fprintf(w, "\n")
	}

	return db.View(func(tx *bolt.Tx) error {
		return tx.ForEach(func(name []byte, buck *bolt.Bucket) error {
			switch {
			case 0 == bytes.Compare(name, peer.BoltDataBucketName):
				if cfg.DumpKey != "" {
					buck.ForEach(func(k, v []byte) error {
						if string(k) == cfg.DumpKey {
							os.Stdout.Write(v)
							os.Stdout.Sync()
							// It is okay to abort txn now.
							// And pointless to keep scanning.
							return ErrStopLooping
						}
						return nil
					})

				} else if cfg.ShowData {
					fmt.Fprintf(w, "* 'data' bucket:\n")
					j := 0
					buck.ForEach(func(k, v []byte) error {
						fmt.Fprintf(w, "%s%v) '%v' -> '%v'\n",
							indent, j, string(k), string(v))
						j++
						return nil
					})
				}
			case 0 == bytes.Compare(name, peer.BoltMetaBucketName):
				if cfg.DumpKey != "" {
					return nil
				}
				fmt.Fprintf(w, "* 'meta' bucket:\n")
				j := 0
				buck.ForEach(func(k, v []byte) error {
					var meta api.KeyInv
					_, err := meta.UnmarshalMsg(v)
					if err != nil {
						return err
					}

					fmt.Fprintf(w, "%s%v) '%v' -> %s\n",
						indent, j, string(k), &meta)
					j++
					return nil
				})
			}
			return nil // keep iterating
		})
	})
}

func FileExists(name string) bool {
	fi, err := os.Stat(name)
	if err != nil {
		return false
	}
	if fi.IsDir() {
		return false
	}
	return true
}

func BytesToInt64(by []byte) int64 {
	return int64(binary.LittleEndian.Uint64(by))
}
