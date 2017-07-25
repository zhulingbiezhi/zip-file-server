package main

import (
	"flag"
	"strconv"
	"strings"

	"./models/h5server"
	"./models/hfserver"
)

func main() {
	h5server.H5server_main("23064", 911515829)
	return
	flag.Parse()
	if flag.NArg() < 3 {
		return
	}
	size, err := strconv.Atoi(flag.Arg(2))
	if err != nil {
		return
	}
	if size <= 0 || size > (1024*1024*1024) {
		return
	}
	if strings.Compare(flag.Arg(0), "hfserver") == 0 {
		if flag.NArg() < 4 {
			return
		}
		hfserver.Hfserver_main(flag.Arg(1), size, flag.Arg(3))
	} else if strings.Compare(flag.Arg(0), "h5server") == 0 {
		h5server.H5server_main(flag.Arg(1), size)
	} else {
		return
	}
}
