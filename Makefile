bin: vendor
	go build gitlab.bearstech.com/factory/containers-watch/

vendor:
	dep ensure -v

test:
	go test gitlab.bearstech.com/factory/containers-watch/watcher
	
docker: vendor
	docker run -ti --rm \
	-v `pwd`:/go/src/gitlab.bearstech.com/factory/containers-watch/ \
	-w /go/src/gitlab.bearstech.com/factory/containers-watch/ \
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