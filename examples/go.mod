module example.com/svc-demo

go 1.24.1

require (
	github.com/joho/godotenv v1.5.1
	github.com/mkumatag/svc-go-sdk v1.0.0
)

replace github.com/mkumatag/svc-go-sdk => ../
