compile-proto:
	protoc --go_out=paths=source_relative:. --go-grpc_out=paths=source_relative:. proto/*.proto

launch-server:
	go run server/main.go

launch-client:
	go run client/main.go
	
