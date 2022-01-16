package main

import (
	"fmt"
	"runtime"
	"time"
)

func main() {
	var x int
	n := runtime.GOMAXPROCS(0)
	for i := 0; i < n; i++ {
		go func() {
			for {
				x = x + 1
			}
		}()
	}
	fmt.Println("start sleep")
	time.Sleep(time.Second * 3)
	fmt.Println("end sleep")
	fmt.Println("x =", x)
}
