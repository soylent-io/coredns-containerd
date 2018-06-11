bin: vendor
	go build github.com/factorysh/containers-watch/

vendor:
	dep ensure -v

test:
	go test github.com/factorysh/containers-watch/watcher
	
docker: vendor
	docker run -ti --rm \
	-v `pwd`:/go/src/github.com/factorysh/containers-watch/ \
	-w /go/src/github.com/factorysh/containers-watch/ \
    bearstech/golang-dep \
	make bin

upx:
	docker run -ti --rm \
	-v `pwd`:/upx \
	-w /upx \
	bearstech/upx \
	upx containers-watch

clean:
	rm -rf vendor