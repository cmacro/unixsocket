build:
	go build -o ./bin/serversocket ./example/server/
	go build -o ./bin/echoServer ./example/echoserver/
	go build -o ./bin/clientsocket ./example/client/
	go build -o ./bin/autoclient ./example/autoclient/
	go build -o ./bin/clientgnet ./example/clientgnet/

deps:
	go mod tidy