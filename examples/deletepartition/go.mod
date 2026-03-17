module github.com/sudeeshjohn/PowerHMC/examples/deletepartition

go 1.24.4

require (
	github.com/sudeeshjohn/PowerHMC v0.0.0
	github.com/sudeeshjohn/svc-go-sdk v0.0.0
)

require github.com/beevik/etree v1.6.0 // indirect

replace github.com/sudeeshjohn/PowerHMC => ../../

replace github.com/sudeeshjohn/svc-go-sdk => ../../../svc-go-sdk
