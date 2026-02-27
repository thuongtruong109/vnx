build-api:
	go build -o ./api/vnx.exe ./api/main.go

api-run:
	cd api && go run main.go