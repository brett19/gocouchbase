devsetup:
	go get "github.com/kisielk/errcheck"
	go get "golang.org/x/lint/golint"
	go get "github.com/gordonklaus/ineffassign"
	go get "github.com/client9/misspell/cmd/misspell"
	git submodule update --remote --init --recursive

test:
	go test ./
fasttest:
	go test -short ./

cover:
	go test -coverprofile=cover.out ./

checkerrs:
	errcheck -blank -asserts -ignoretests *.go

checkfmt:
	! gofmt -l -d *.go 2>&1 | read

checkvet:
	go vet *.go

checkiea:
	ineffassign -n .

checkspell:
	misspell -error *.*

lint: checkfmt checkerrs checkvet checkiea checkspell
	golint -set_exit_status -min_confidence 0.81 *.go

check: lint
	go test -short -cover -race ./

bench:
	go test -bench=. -run=none --disable-logger=true

updatetestcases:
	git submodule update --remote --init --recursive

.PHONY: all test devsetup fasttest lint cover checkerrs checkfmt checkvet checkiea checkspell check bench updatetestcases
