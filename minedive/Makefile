all: be middlemine mined minecli remined sigmined

be: cmd/be/main.go
	GOOS=js GOARCH=wasm go build -o ../browser_extension/minedive.wasm cmd/be/main.go

middlemine: cmd/middlemine/main.go
	go build -o bin/middlemine cmd/middlemine/main.go

mined: cmd/mined/main.go cmd/mined/logger.go
	go build -o bin/mined cmd/mined/*.go

remined: cmd/remined/main.go cmd/remined/logger.go
	go build -o bin/remined cmd/remined/*.go

sigmined: cmd/sigmined/main.go cmd/sigmined/logger.go
	go build -o bin/sigmined cmd/sigmined/*.go

minecli: cmd/minecli/main.go
	go build -o bin/minecli cmd/minecli/main.go

clean:
	rm bin/*
