build:
	./build-all.bash

deps:
	go mod download && go mod tidy
