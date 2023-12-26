module bench

go 1.21.1

require (
	github.com/cloudflare/golibs v0.0.0-20210909181612-21743d7dd02a
	github.com/maypok86/otter v0.0.0-20231222143008-a9479c80c78a
	github.com/phuslu/lru v0.0.0-20231226170143-1f4be1b88a11
)

require (
	github.com/dolthub/maphash v0.1.0 // indirect
	github.com/dolthub/swiss v0.2.1 // indirect
	github.com/gammazero/deque v0.2.1 // indirect
)

replace github.com/phuslu/lru => ../
