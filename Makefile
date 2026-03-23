BINARY  := kurt
BIN_DIR := bin

.PHONY: build clean

build:
	@mkdir -p $(BIN_DIR)
	go mod tidy
	go build -o $(BIN_DIR)/$(BINARY) .

build-linux:
	@mkdir -p $(BIN_DIR)
	go mod tidy
	GOOS=linux GOARCH=amd64 go build -o $(BIN_DIR)/$(BINARY)-linux .

clean:
	rm -rf $(BIN_DIR)
