#!/usr/bin/env bash

# Application name and build directory
APP_NAME="gbr"
APP_BUILD_DIR="./bin"
APP_MAIN_FILE="./main.go"

# Function to build for a specific OS and architecture
build() {
  OS=$1
  ARCH=$2
  OUTPUT_DIR=$3
  OUTPUT_FILE=$4

  echo "Building for $OS $ARCH..."
  GOOS=$OS GOARCH=$ARCH go build -o "$OUTPUT_DIR"/"$OUTPUT_FILE" "$APP_MAIN_FILE"
  if [ $? -ne 0 ]; then
    echo "Build failed for $OS $ARCH"
    exit 1
  fi
}

# Build based on arguments
if [ "$1" == "all" ]; then
  build "windows" "amd64" $APP_BUILD_DIR/amd64/windows "${APP_NAME}.exe"
  build "linux" "amd64" $APP_BUILD_DIR/amd64/linux "${APP_NAME}"
  build "darwin" "amd64" $APP_BUILD_DIR/amd64/macos "${APP_NAME}"
  build "darwin" "arm64" $APP_BUILD_DIR/arm64/macos "${APP_NAME}"
elif [ "$1" == "windows" ]; then
  build "windows" "amd64" $APP_BUILD_DIR/amd64/windows "${APP_NAME}.exe"
elif [ "$1" == "linux" ]; then
  build "linux" "amd64" $APP_BUILD_DIR/amd64/linux "${APP_NAME}"
elif [ "$1" == "macos" ]; then
  build "darwin" "amd64" $APP_BUILD_DIR/amd64/macos "${APP_NAME}"
  build "darwin" "arm64" $APP_BUILD_DIR/arm64/macos "${APP_NAME}"
else
  # Build for the current OS and architecture
  echo "Build for the current OS and architecture..."
  go build -o "$APP_BUILD_DIR"/"$APP_NAME" "$APP_MAIN_FILE"
  if [ $? -ne 0 ]; then
    echo "Build failed for the current OS and architecture"
    exit 1
  fi
fi