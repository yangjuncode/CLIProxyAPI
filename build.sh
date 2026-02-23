#!/bin/bash

# 确保 ~/bin 目录存在
mkdir -p ~/bin

# 编译项目
echo "Building cli-proxy-api..."
go build -o ~/bin/cli-proxy-api ./cmd/server

if [ $? -eq 0 ]; then
    echo "Successfully built to ~/bin/cli-proxy-api"
else
    echo "Build failed"
    exit 1
fi
