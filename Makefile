
protoc: 
	docker run \
	-v ${PWD}/vendor/github.com/gogo/protobuf/gogoproto/:/opt/include/gogoproto/ \
	-v ${PWD}/vendor/go.etcd.io/:/opt/include/go.etcd.io/ \
	-v ${PWD}:/defs \
	namely/protoc-all -f ./api/raft.proto -l gogo -o .