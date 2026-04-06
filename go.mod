module github.com/sudeeshjohn/powerhmc-go

go 1.23.0

toolchain go1.24.4

require (
	github.com/beevik/etree v1.6.0
	github.com/sudeeshjohn/svc-go-sdk v0.0.0
	golang.org/x/crypto v0.31.0
)

require golang.org/x/sys v0.28.0 // indirect

replace github.com/sudeeshjohn/svc-go-sdk => ../svc-go-sdk
