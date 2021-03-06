package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/boltdb/bolt"
)

type ViewBoltConfig struct {
	DbPath   string
	ShowData bool
	DumpKey  string

	Bopt *bolt.Options
	Db   *bolt.DB
}

// DefineFlags should be called before myflags.Parse().
func (c *ViewBoltConfig) DefineFlags(fs *flag.FlagSet) {

	fs.StringVar(&c.DumpKey, "key", "", "single key to dump as msgpack to stdout")
	fs.StringVar(&c.DbPath, "db", "", "path to our boltdb file")
	fs.BoolVar(&c.ShowData, "data", false, "show the data")
}

// ValidateConfig() should be called after myflags.Parse().
func (c *ViewBoltConfig) ValidateConfig() error {

	// -db default
	if len(os.Args) == 2 && c.DbPath == "" {
		c.DbPath = os.Args[1]
	}

	// -db
	if c.DbPath == "" {
		return fmt.Errorf("-db <path to boltdb file> required and missing")
	}
	if !FileExists(c.DbPath) {
		return fmt.Errorf("bad -db '%s': path does not exist.", c.DbPath)
	}

	return nil
}
