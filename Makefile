.DEFAULT_GOAL=build

install:
	go install github.com/caddyserver/xcaddy/cmd/xcaddy@v0.4.4
	go install github.com/go-delve/delve/cmd/dlv@v1.25.0

clean:
	rm caddy

build: install
	XCADDY_DEBUG=1 xcaddy build --with $(shell awk '/^module/ {print $$2}' go.mod)=$(PWD)

debug:
	dlv --listen=:2345 --headless=true --api-version=2 --accept-multiclient exec ./caddy run

run:
	./caddy run

test:
	go test -v ./...
