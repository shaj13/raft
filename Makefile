
protoc: 
	docker run \
	-v ${PWD}/vendor/github.com/gogo/protobuf/gogoproto/:/opt/include/gogoproto/ \
	-v ${PWD}/vendor/go.etcd.io/etcd/raft/v3/raftpb/:/opt/include/go.etcd.io/etcd/raft/v3/raftpb/ \
	-v ${PWD}:/defs \
	namely/protoc-all -f ./internal/raftpb/raft.proto -l gogo -o .

	docker run \
	-v ${PWD}/vendor/github.com/gogo/protobuf/gogoproto/:/opt/include/gogoproto/ \
	-v ${PWD}/vendor/go.etcd.io/:/opt/include/go.etcd.io/ \
	-v ${PWD}/internal/raftpb/:/opt/include/github.com/shaj13/raftkit/internal/raftpb/ \
	-v ${PWD}:/defs \
	namely/protoc-all -f ./internal/transport/grpc/pb/raft.proto -l gogo -o .
