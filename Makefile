MODPATH=	github.com/soylent-io/coredns-containerddiscovery

bin:
	go build ${MODPATH}

test:
	go test ${MODPATH}/watcher
	
docker:
	docker run -ti --rm \
		-v `pwd`:/go/src/${MODPATH} \
		-w /go/src/${MODPATH} \
		bearstech/golang-dep \
		make bin

upx:
	docker run -ti --rm \
		-v `pwd`:/upx \
		-w /upx \
		bearstech/upx \
		upx coredns-containerddiscovery
