bin:
	go build github.com/factorysh/containers-watch/

test:
	go test github.com/factorysh/containers-watch/watcher
	
docker:
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