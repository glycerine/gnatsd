package swp

func panicOn(err error) {
	if err != nil {
		panic(err)
	}
}

func logOn(err error) {
	if err != nil {
		mylog.Println(err)
	}
}
