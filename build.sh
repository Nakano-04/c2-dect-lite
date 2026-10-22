#!/bin/bash

echo "========================================"
echo "   C2-DECT Lite Build System v2.0"
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
echo "  1. export C2_JWT_SECRET and C2_BOOTSTRAP_USER / C2_BOOTSTRAP_PASSWORD"
echo "  2. Start server: ./build/server"
echo "  3. Start agent:  ./build/agent -s 127.0.0.1 -p 8443"
echo
echo "There are no default credentials: the administrator is created on the"
echo "first start from C2_BOOTSTRAP_USER / C2_BOOTSTRAP_PASSWORD."
echo
