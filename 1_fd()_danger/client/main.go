package main

import (
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	var count int32 = 0
	startTime := time.Now()
	execCount := 10
	for j := 0; j < execCount; j++ {
		wg := sync.WaitGroup{}
		for i := 0; i < 1000; i++ {
			wg.Add(1)
			go func() {
				seq := atomic.AddInt32(&count, 1)
				defer func() {
					wg.Done()
				}()
				conn, err := net.Dial("tcp", "127.0.0.1:8082")
				if err != nil {
					log.Printf("dial conn seq :[%d] err: [%v]", seq, err)
					return
				}
				defer func() {
					_ = conn.Close()
					log.Printf("close conn seq [%d]", seq)
				}()
				time.Sleep(3 * time.Second)
				_, err = conn.Write([]byte("hello bug!"))
				if err != nil {
					log.Println(err)
					return
				}
				revBuf := make([]byte, 100, 100)
				_, err = conn.Read(revBuf)
				if err != nil {
					log.Println(err)
					return
				}
				log.Printf("recv: %s", revBuf)
			}()
		}
		wg.Wait()
		log.Println("next: ", j+1)
		if j < execCount-1 {
			time.Sleep(10 * time.Second)
		}
	}
	log.Println("game over")
	log.Println("Duration: ", time.Since(startTime))
}
