test:
	go clean -testcache
	GOFLAGS=-mod=vendor go test ./... -race 

install:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v1.19.0
	GO111MODULE=off go get github.com/mattn/goveralls
	go mod tidy 
	go mod vendor

deploy-cover:
	goveralls -coverprofile=${PWD}/cover/coverage.out -service=circle-ci -repotoken=$$COVERALLS_TOKEN

lint: 
	./bin/golangci-lint run -c .golangci.yml ./...
	
lint-fix: 
	@FILES="$(shell find . -type f -name '*.go' -not -path "./vendor/*")"; goimports -local "github.com/shaj13/go-guardian/v2" -w $$FILES
	./bin/golangci-lint run -c .golangci.yml ./... --fix 
	./bin/golangci-lint run -c .golangci.yml ./... --fix

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
