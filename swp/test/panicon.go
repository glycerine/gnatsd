package main

import (
	"log"
)

func panicOn(err error) {
	if err != nil {
		panic(err)
	}
}

func logOn(err error) {
	if err != nil {
		log.Println(err)
	}
}
