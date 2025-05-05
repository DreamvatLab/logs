# Generate models
protoc  -I ./ --go_out=./ --go_opt=paths=source_relative ./*.proto

# Generate service
protoc -I ./ --go-grpc_out=require_unimplemented_servers=false:./ --go-grpc_opt=paths=source_relative ./*.proto

# Generate db tags
protoc-go-inject-tag --input=./*.go