module bench

go 1.21.1

require (
	github.com/DmitriyVTitov/size v1.5.0
	github.com/cloudflare/golibs v0.0.0-20210909181612-21743d7dd02a
	github.com/dgraph-io/ristretto v0.1.1
	github.com/goburrow/cache v0.1.4
	github.com/karlseguin/ccache/v3 v3.0.5
	github.com/phuslu/lru v0.0.0-20231229145559-356b4a3d88ba
)

require (
	github.com/cespare/xxhash/v2 v2.1.1 // indirect
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/stretchr/testify v1.8.1 // indirect
	golang.org/x/sys v0.0.0-20221010170243-090e33056c14 // indirect
)

replace github.com/phuslu/lru => ../
