GOPATH=$(HOME)
PKG=github.com/sami2020pro/vile/src

all:
	go install $(PKG)/cmd/vile

dep:
	go get -d $(PKG)/cmd/vile

test:
	go test $(PKG)

clean:
	go clean $(PKG)/...
	rm -rf *~

check:
	(cd $(GOPATH)/src/$(PKG); go vet $(PKG); go fmt $(PKG)
