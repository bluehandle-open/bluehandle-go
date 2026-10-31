VERSION ?= dev
GUI     := cmd/bluehandle
WAILS   ?= wails3

.PHONY: all test vet build cli gui frontend bindings syso clean

all: vet test build

test:
	go test ./...

vet:
	go vet ./...

build: cli gui

# Windowless debugging tool.
cli:
	go build -o bin/bluehandle-cli ./cmd/bluehandle-cli

# Desktop app (Windows). Embeds the built frontend; no cgo needed.
gui: frontend
	go build -tags production -trimpath -ldflags "-s -w -H windowsgui -X main.version=$(VERSION)" -o bin/bluehandle.exe ./$(GUI)

frontend:
	cd $(GUI)/frontend && npm ci && npm run build

# Regenerate after changing exported methods or types in internal/gui.
bindings:
	cd $(GUI) && $(WAILS) generate bindings -ts -i -d frontend/bindings ./

# Regenerate after changing the icon, version info or manifest.
syso:
	cd $(GUI) && $(WAILS) generate syso -arch amd64 -icon build/icon.ico -info build/info.json -manifest build/bluehandle.exe.manifest -out rsrc_windows_amd64.syso

clean:
	rm -rf bin dist $(GUI)/frontend/dist
