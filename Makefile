.PHONY: test check-lines build-wasm run-local clean

test:
	go test -race -v ./...

check-lines:
	./scripts/check-line-limit.sh

build-wasm:
	go run github.com/syumai/workers-go/cmd/workers-assets-gen
	tinygo build -o ./build/app.wasm -target wasm -no-debug .
	@ls -lh ./build/app.wasm

run-local:
	go run .

clean:
	rm -rf build
