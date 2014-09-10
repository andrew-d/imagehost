.SUFFIXES:

.PHONY: all
all: build/imagehost


build/imagehost: *.go
	godep go build -o $@ $^
