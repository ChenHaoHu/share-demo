package main

import (
	"log"
	"os"
	"runtime/pprof"
	"time"
)

func main() {
	f, err := os.Create("cpu.out")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	err = pprof.StartCPUProfile(f)
	if err != nil {
		log.Fatal(err)
	}
	defer pprof.StopCPUProfile()

	ch := make(chan int)
	go func() {
		time.Sleep(13 * time.Second)
		log.Println("hello goruntine")
		ch <- 1
	}()
	log.Println("start sleep")
	time.Sleep(23 * time.Second)
	log.Println("end sleep")
	<-ch
}
