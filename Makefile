ifdef RS_SHELL
LDFLAGS := $(LDFLAGS) -X 'main.defaultShell=$(RS_SHELL)'
endif

ifdef RS_PUB
LDFLAGS := $(LDFLAGS) -X 'main.authorizedKey=$(RS_PUB)'
endif

#RS_PASS ?= $(shell hexdump -n 8 -e '2/4 "%08x"' /dev/urandom)
ifdef RS_PASS
LDFLAGS := $(LDFLAGS) -X 'main.localPassword=$(RS_PASS)'
endif

ifdef LUSER
LDFLAGS := $(LDFLAGS) -X 'main.LUSER=$(LUSER)'
endif

ifdef LHOST
LDFLAGS := $(LDFLAGS) -X 'main.LHOST=$(LHOST)'
endif

ifdef LPORT
LDFLAGS := $(LDFLAGS) -X 'main.LPORT=$(LPORT)'
endif

ifdef BPORT
LDFLAGS := $(LDFLAGS) -X 'main.HomeBindPort=$(BPORT)'
endif

.PHONY: build
build: clean
	CGO_ENABLED=0 					go build -ldflags="$(LDFLAGS) -s -w" -o bin/rssh .
	CGO_ENABLED=0 GOARCH=amd64	GOOS=linux	go build -ldflags="$(LDFLAGS) -s -w" -o bin/rsshx64 .
	CGO_ENABLED=0 GOARCH=arm64	GOOS=linux	go build -ldflags="$(LDFLAGS) -s -w" -o bin/rssh_a64 .
	CGO_ENABLED=0 GOARCH=amd64	GOOS=windows	go build -ldflags="$(LDFLAGS) -s -w" -o bin/rsshx64.exe .
	CGO_ENABLED=0 GOARCH=arm64	GOOS=windows	go build -ldflags="$(LDFLAGS) -s -w" -o bin/rssh_a64.exe .

.PHONY: clean
clean:
	rm -f bin/*rssh*

.PHONY: compressed
compressed: build
	@for f in $(shell ls bin); do upx -o "bin/upx_$${f}" "bin/$${f}"; done
