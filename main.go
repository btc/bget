package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime/pprof"
)

func main() {
	f, perr := os.Create("cpu.pprof")
	if perr != nil {
		log.Fatal(perr)
	}
	pprof.StartCPUProfile(f)
	defer pprof.StopCPUProfile()
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {

	url := flag.String("url", "https://nodejs.org/dist/v16.13.0/node-v16.13.0.tar.gz", "a URL to download")
	numWorkers := flag.Int("workers", 10, "number of workers")
	flag.Parse()

	fmt.Println("url: ", *url)
	fmt.Println("workers: ", *numWorkers)

	path, name, err := MultiSourceGet(*url, *numWorkers, false)
	if err != nil {
		log.Println(path)
		return err
	}
	if err := os.Rename(path, name); err != nil {
		return err
	}
	fmt.Println("saved to: ", name)
	return nil
}
