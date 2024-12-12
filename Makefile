REPO ?= pcm720/nhddl
CORS_PROXY ?=
VERSION ?= $(shell git describe --always --dirty --tags --exclude pages)

all: nhddl-psu

wasm:
	mkdir out
	GOOS=js GOARCH=wasm tinygo build -o out/app.wasm -ldflags "-X main.Repo=$(REPO) -X main.CORSProxy=$(CORS_PROXY)" ./cmd/nhddl-psu

nhddl-psu: clean wasm
	cp "$(shell tinygo env TINYGOROOT)/targets/wasm_exec.js" ./out/
	cp -r ./cmd/nhddl-psu/res/* ./out/

psubuilder: clean
	go build -ldflags "-X main.Version=$(VERSION)" -o out/psubuilder ./cmd/psubuilder
 
clean:
	rm -rf out