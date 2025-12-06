# Makefile for simple-agent

BINARY_NAME=simple-agent
INSTALL_DIR=/usr/local/bin

.PHONY: all build install clean

all: build

build:
	go build -o $(BINARY_NAME) main.go

install: build
	@echo "Installing to $(INSTALL_DIR)..."
	sudo cp $(BINARY_NAME) $(INSTALL_DIR)
	@echo "Installation complete. Ensure $(INSTALL_DIR) is in your PATH."

clean:
	go clean
	rm -f $(BINARY_NAME)
