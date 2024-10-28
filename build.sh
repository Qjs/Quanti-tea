export GOPATH=$HOME/go  
export PATH=$PATH:$GOPATH/bin 

protoc --go_out=. --go-grpc_out=. ./server/proto/metrics.proto

go build -o ./build/quanti-tea-steep ./server/main.go
go build -o ./build/quanti-tea ./tui/main.go

# Exit immediately if a command exits with a non-zero status
set -e

# Define source and destination directories
SRC_DIR="./server/webapp/templates"
DEST_DIR="./build/server/webapp/templates"

# Create the destination directory if it doesn't exist
if [ ! -d "$DEST_DIR" ]; then
    mkdir -p "$DEST_DIR"
fi

# Copy all files from the source to the destination directory
# Including hidden files and directories
cp -r "$SRC_DIR"/. "$DEST_DIR"/
