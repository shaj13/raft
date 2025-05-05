test:
	go clean -testcache
	GOFLAGS=-mod=vendor go test ./... -race 

install:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v1.19.0
	go mod tidy 
	go mod vendor

clean: 
	rm -rf ${PWD}/cover 

cover: clean 
	mkdir ${PWD}/cover 
	go clean -testcache
	GOFLAGS=-mod=vendor go test `go list ./... | grep -v github.com/shaj13/raft/rafttest` -timeout 30s -race -v -cover -coverprofile=${PWD}/cover/coverage.out

rafttest: clean
	go clean -testcache
	GOFLAGS=-mod=vendor go test github.com/shaj13/raft/rafttest -race -v 

deploy-cover:
#	goveralls -coverprofile=${PWD}/cover/coverage.out -service=circle-ci -repotoken=$$COVERALLS_TOKEN

lint: 
	./bin/golangci-lint run -c .golangci.yml ./...
	
lint-fix: 
	@FILES="$(shell find . -type f -name '*.go' -not -path "./vendor/*")"; goimports -local "github.com/shaj13/raft" -w $$FILES
	./bin/golangci-lint run -c .golangci.yml ./... --fix 
	./bin/golangci-lint run -c .golangci.yml ./... --fix

protoc:
	docker run \
	-v ${PWD}/vendor/github.com/gogo/protobuf/gogoproto/:/opt/include/gogoproto/ \
	-v ${PWD}/vendor/go.etcd.io/raft/v3/raftpb/:/opt/include/go.etcd.io/raft/v3/raftpb/ \
	-v ${PWD}/internal/raftpb:/defs/gen/pb-go/raftpb \
	namely/protoc-all:1.50_0 -i /defs -f gen/pb-go/raftpb/raft.proto -l gogo -o .

	docker run \
	-v ${PWD}/vendor/github.com/gogo/protobuf/gogoproto/:/opt/include/gogoproto/ \
	-v ${PWD}/vendor/go.etcd.io/:/opt/include/go.etcd.io/ \
	-v ${PWD}/internal/raftpb/:/opt/include/github.com/shaj13/raftkit/internal/raftpb/ \
	-v ${PWD}/internal/transport/raftgrpc/pb:/defs/gen/pb-go/pb \
	namely/protoc-all:1.50_0 -i /defs -f gen/pb-go/pb/raft.proto -l gogo -o .
