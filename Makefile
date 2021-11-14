run: bget
	./bget

test:
	go test ./...

bget:
	go build .

cpu.pprof: run
	./bget

mem.pprof:
	go test -memprofile=mem.pprof -memprofilerate=1

profile_cpu: bget cpu.pprof
	go tool pprof -http=:8080 ./bget ./cpu.pprof

profile_mem: bget mem.pprof
	go tool pprof -http=:8080 ./bget ./mem.pprof

clean:
	rm -f bget *.pprof

cov:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out
