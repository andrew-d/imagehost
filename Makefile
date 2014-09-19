.SUFFIXES:

.PHONY: all
all: build/imagehost


build/imagehost: *.go
	godep go build -o $@ $^


.PHONY: test
test:
	godep go test -v .
