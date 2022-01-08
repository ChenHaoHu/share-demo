package main

import (
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net"
	"net/http"
	"sync/atomic"
)

func main() {
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Fatal(http.ListenAndServe("127.0.0.1:9092", nil))
	}()
	listener, err := net.Listen("tcp", "0.0.0.0:8081")
	if err != nil {
		log.Fatal(err)
	}
	var execCount int32
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
		}
		tcpConn := conn.(*net.TCPConn)
		go handConn(tcpConn)
		log.Println("execCount: ", atomic.AddInt32(&execCount, 1))
	}
}
func handConn(conn *net.TCPConn) {
	defer func() {
		_ = conn.Close()
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
