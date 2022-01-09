package main

import (
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
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
		seq := atomic.AddInt32(&execCount, 1)
		go handConn(tcpConn, seq)
		//log.Println("submit exec seq: ", seq)
	}
}

func handConn(conn *net.TCPConn, seq int32) {
	//log.Println("start exec seq: ", seq)

	defer func() {
		//log.Println("end exec seq: ", seq)
	}()

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
	file.Fd()
	//fd := file.Fd()
	//log.Printf("conn fd: %d", fd)
	/**********************************/

	//接收消息后发送一次消息 然后主动结束
	revBuf := make([]byte, 100, 100)
	_, err = conn.Read(revBuf)
	if err != nil {
		log.Println(err)
		return
	}
	//log.Printf("seq: %d recv: %s", seq, revBuf)

	_, err = conn.Write([]byte("back msg"))
	if err != nil {
		log.Println(err)
		return
	}
}
