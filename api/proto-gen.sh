protoc \
-I ./ \
-I ../contrib/googleapis \
-I ../contrib/grpc-gateway \
--go_out ./ --go_opt paths=source_relative \
--go-grpc_out ./ --go-grpc_opt paths=source_relative \
--grpc-gateway_out ./ --grpc-gateway_opt paths=source_relative \
--openapiv2_out=allow_merge=true,merge_file_name=api:./ \
./*.proto