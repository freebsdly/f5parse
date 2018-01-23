package main

import (
	"flag"
	"fmt"
)

var (
	srcfile   = flag.String("from", "", "which f5 configuration file will be read")
	poolsfile = flag.String("op", "pools.txt", "which file the pools result will be write to")
	vsfile    = flag.String("ov", "vs.txt", "which file the virtual servers result will be write to")
)

func main() {
	flag.Parse()

	cfg := NewF5Config(*srcfile)
	err := cfg.Parse()
	if err != nil {
		fmt.Printf("error: %s\n", err)
	}

	cfg.WritePools(*poolsfile)
	cfg.WriteVS(*vsfile)

}
