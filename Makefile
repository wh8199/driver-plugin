all:
	protoc --go_out=. --go-grpc_out=. protobuf/driver.proto