
protoc: 
	docker run \
	-v ${PWD}/vendor/github.com/gogo/protobuf/gogoproto/:/opt/include/gogoproto/ \
	-v ${PWD}/vendor/go.etcd.io/:/opt/include/go.etcd.io/ \
	-v ${PWD}:/defs \
	namely/protoc-all -f ./internal/raftpb/raft.proto -l gogo -o .

	docker run \
	-v ${PWD}/vendor/github.com/gogo/protobuf/gogoproto/:/opt/include/gogoproto/ \
	-v ${PWD}/vendor/go.etcd.io/:/opt/include/go.etcd.io/ \
	-v ${PWD}:/opt/include/github.com/shaj13/raftkit/ \
	-v ${PWD}:/defs \
	namely/protoc-all -f ./internal/raftpb/raft.proto -l gogo -o .
