#!/bin/bash

# chmod +x scripts/reset_env.sh

set -e

# 删除 downloads 目录下所有文件和子目录
if [ -d "./downloads" ]; then
  echo "删除 downloads 目录内容..."
  rm -rf ./downloads/*
fi

# 删除数据库文件
if [ -f "./favlist.db" ]; then
  echo "删除 favlist.db..."
  rm -f ./favlist.db
fi

echo "环境已初始化。"