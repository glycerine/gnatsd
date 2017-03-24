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
		panic(err)
	}
	cfg.Dump(cfg.Db, os.Stdout)
}

func (cfg *ViewBoltConfig) Dump(db *bolt.DB, w io.Writer) error {
	fmt.Fprintf(w, "\n")
	indent := "    "

	return db.View(func(tx *bolt.Tx) error {
		return tx.ForEach(func(name []byte, buck *bolt.Bucket) error {
			switch {
			case 0 == bytes.Compare(name, peer.BoltDataBucketName):
				if cfg.ShowData {
					fmt.Fprintf(w, "* 'data' bucket:\n")
					j := 0
					buck.ForEach(func(k, v []byte) error {
						fmt.Fprintf(w, "%s%v) '%v' -> '%v'\n",
							indent, j, string(k), string(v))
						j++
						return nil
					})
				}
			case 0 == bytes.Compare(name, peer.BoltTsBucketName):
				fmt.Fprintf(w, "* 'ts' bucket:\n")
				j := 0
				buck.ForEach(func(k, v []byte) error {
					n := BytesToInt64(v)
					fmt.Fprintf(w, "%s%v) '%v' -> %v\n",
						indent, j, string(k), time.Unix(0, n).UTC())
					j++
					return nil
				})

			case 0 == bytes.Compare(name, peer.BoltSizeBucketName):
				fmt.Fprintf(w, "* 'size' bucket:\n")
				j := 0
				buck.ForEach(func(k, v []byte) error {
					n := BytesToInt64(v)
					fmt.Fprintf(w, "%s%v) '%v' -> %v #bytes\n",
						indent, j, string(k), n)
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
