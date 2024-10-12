
all: build

clean:
	rm *.so

lint:
	golangci-lint run

build:
	go build -buildmode=plugin -o parquet.so *.go

test: lint
	go test -coverprofile=cov.out github.com/ohler55/slip-parquet/slipparquet

.PHONY: all build
