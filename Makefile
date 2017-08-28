CURR=$(shell pwd)

export GOPATH=$(CURR)/linux-build

#If you have non default go paths (like me change the vailables)
#export GOROOT=/usr/local/go17/go
#GO=/usr/local/go17/go/bin/go
GO=go

all: clean full
	echo "Build done"

clean:
	rm -rf sc_start sc_proxy sc_runtime

#copy: 
#	/bin/mkdir -p linux-build && \
#       	cd $(GOPATH) && \
#        /bin/ln -s ../src  && \
#        cd -

full: 
	echo "Building with " && \
        $(GO) version && \
        $(GO) build -i -x -race -o sc_start host && \
	$(GO) build -i -x -race -o sc_proxy proxy && \
	$(GO) build -i -x -race -o sc_runtime cont

