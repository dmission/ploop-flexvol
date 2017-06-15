SOURCES := $(shell find . 2>&1 | grep -E '.*\.(c|h|go)$$')

.DEFAULT: ploop.bin

ploop.bin: $(SOURCES)
	go build -o ploop.bin .
install: ploop.bin
	cp ploop.sh /usr/libexec/kubernetes/kubelet-plugins/volume/exec/virtuozzo~ploop/ploop
	cp ploop.bin /usr/libexec/kubernetes/kubelet-plugins/volume/exec/virtuozzo~ploop/ploop.bin
