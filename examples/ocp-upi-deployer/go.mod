module github.com/sudeeshjohn/powerhmc-go/examples/ocp-upi-deployer

go 1.25.0

require (
	github.com/sudeeshjohn/powerhmc-go v0.0.0-00010101000000-000000000000
	github.com/sudeeshjohn/svc-go-sdk v0.0.0
	golang.org/x/crypto v0.49.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/beevik/etree v1.6.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
)

replace github.com/sudeeshjohn/powerhmc-go => ../../

replace github.com/sudeeshjohn/svc-go-sdk => ../../../svc-go-sdk
