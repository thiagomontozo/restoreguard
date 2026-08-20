.PHONY: test integration e2e build cleanup
test:
	go test -C backend -p 1 ./...
integration:
	powershell -ExecutionPolicy Bypass -File scripts/test.ps1
e2e:
	powershell -ExecutionPolicy Bypass -File scripts/e2e.ps1
build:
	go build -C backend ./cmd/server
cleanup:
	powershell -ExecutionPolicy Bypass -File scripts/cleanup.ps1
