module github.com/sudeeshjohn/powerhmc-go/examples/ocp-upi-deployer

go 1.23.0

toolchain go1.24.4

require (
	github.com/sudeeshjohn/powerhmc-go v0.0.0
	golang.org/x/crypto v0.31.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/beevik/etree v1.6.0 // indirect
	golang.org/x/sys v0.28.0 // indirect
)

replace github.com/sudeeshjohn/powerhmc-go => ../../

replace github.com/sudeeshjohn/svc-go-sdk => ../../../svc-go-sdk
