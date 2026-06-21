#!/bin/bash

echo "========================================"
echo "   C2-DECT Lite Build System v1.0"
echo "========================================"
echo

# Create build directory
mkdir -p build

echo "[+] Building server..."
cd server
go build -ldflags="-s -w" -o ../build/server ./cmd/main.go
if [ $? -ne 0 ]; then
    echo "[-] Server build failed!"
    cd ..
    exit 1
fi
cd ..
echo "[+] Server built successfully"

echo "[+] Building agent..."
cd agent
go build -ldflags="-s -w" -o ../build/agent .
if [ $? -ne 0 ]; then
    echo "[-] Agent build failed!"
    cd ..
    exit 1
fi
cd ..
echo "[+] Agent built successfully"

echo
echo "========================================"
echo "   Build Complete!"
echo "========================================"
echo
echo "Binaries location: build/"
echo "  - server"
echo "  - agent"
echo
echo "Quick start:"
echo "  1. Start server: ./build/server"
echo "  2. Start agent: ./build/agent -s 127.0.0.1 -p 8443"
echo
echo "Default credentials: admin / c2-dect"
echo
