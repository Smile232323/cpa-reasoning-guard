PLUGIN_ID := cpa-reasoning-guard
VERSION := 0.2.0
DIST := dist
ARTIFACT := $(DIST)/$(PLUGIN_ID)-v$(VERSION).so

.PHONY: test release clean

test:
	CGO_ENABLED=1 go test cpa-plugin/main.go cpa-plugin/main_test.go

release:
	mkdir -p $(DIST)
	docker run --rm --platform linux/amd64 \
		-v "$(CURDIR):/src" -w /src golang:1.26-bookworm \
		bash -lc 'export PATH="$$PATH:/usr/local/go/bin"; apt-get update >/dev/null && apt-get install -y --no-install-recommends gcc libc6-dev >/dev/null && CGO_ENABLED=1 go build -buildmode=c-shared -o $(ARTIFACT) cpa-plugin/main.go && rm -f $(ARTIFACT:.so=.h) && sha256sum $(ARTIFACT) > $(DIST)/checksums.txt'

clean:
	rm -rf $(DIST)
