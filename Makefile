build:
	go build -o project-launcher main.go
cp:
	cp project-launcher ~/.local/bin/   

install: build cp