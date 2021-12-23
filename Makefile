all: proto build

build:
	go build -ldflags='-w -s' -o bin/driver-plugin main.go 

proto:
	protoc --go_out=. --go-grpc_out=. protobuf/driver.proto
	go mod tidy

clean:
	rm -rf bin
	rm -rf pkg/driverplugin

.PHONY: all build proto