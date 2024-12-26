build:
	go build -o ./bin/serversocket ./example/server/
	go build -o ./bin/clientsocket ./example/client/
	go build -o ./bin/autoclient ./example/autoclient/

deps:
	go mod tidy