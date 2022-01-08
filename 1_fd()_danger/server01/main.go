package main

import (
	"github.com/shirou/gopsutil/v3/process"
	"log"
	"net"
	"os"
	"runtime"
	"time"
)

func main() {
	printMetrics()

	listener, err := net.Listen("tcp", "127.0.0.1:8081")
	if err != nil {
		log.Fatal(err)
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
		}
		tcpConn := conn.(*net.TCPConn)
		go handConn(tcpConn)
	}
}

func handConn(conn *net.TCPConn) {
	defer func() {
		log.Println(conn.Close())
	}()

	//加了如下代码
	/**********************************/
	file, err := conn.File()
	if err != nil {
		log.Println(err)
		return
	}
	defer func() {
		_ = file.Close()
	}()
	fd := file.Fd()
	log.Printf("conn fd: %d", fd)
	/**********************************/

	revBuf := make([]byte, 100, 100)
	_, err = conn.Read(revBuf)
	if err != nil {
		log.Println(err)
		return
	}
	log.Printf("recv: %s", revBuf)
	_, err = conn.Write([]byte("back msg"))
	if err != nil {
		log.Println(err)
		return
	}
}

func printMetrics() {
	proc, err := process.NewProcess(int32(os.Getpid()))
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		threads, err := proc.NumThreads()
		if err != nil {
			log.Fatal(err)
			return
		}
		log.Println("NumThreads:", threads)
		log.Println("NumGoroutine:", runtime.NumGoroutine())
		log.Println("")
		time.Sleep(4 * time.Second)
	}()
}
