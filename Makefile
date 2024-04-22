dev:
	go1.20 mod init docker-image
	go1.20 mod tidy && go1.20 mod vendor

build-linux:
	GOROOT=/usr/local/go1.20 GOOS=linux go1.20 build -o docker-image main.go