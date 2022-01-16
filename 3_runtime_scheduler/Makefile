

run:
	go run main.go

build:
	go build -o main main.go
	./main

trace: build
	go tool pprof main cpu.out