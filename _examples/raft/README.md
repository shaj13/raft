# raft cluster 

raft cluster is an example usage of raft library. It provides a simple REST API for a key-value store cluster backed by the [Raft][raft] consensus algorithm.

[raft]: http://raftconsensus.github.io/

## Getting Started

### Building raft cluster example

get `raft` example

```sh
mkdir -p raftexample
cd raftexample
GOBIN=${PWD} go get github.com/shaj13/raft/_examples/raft
```

### Running single node raft

First start a single-member cluster of raft:

```sh
./raft -state_dir=$TMPDIR/1 -raft :8080 -api :9090 
```

Each raft process maintains a single raft instance and a key-value server.
raft server address (-raft), state dir (-state_dir), and http key-value server address (-api) are passed through the command line.

Next, store a value ("world") to a key ("hello"):

```
curl -L http://127.0.0.1:9090/ -X PUT -d '{"Key":"hello", "Value":"world"}'
```

Finally, retrieve the stored key:

```
curl -L http://127.0.0.1:9090/hello
```

### Running a local cluster
Lets bring two additional raft instances.

```sh
./raft -state_dir $TMPDIR/2 -raft :8081 -api :9091 -join :8080
./raft -state_dir $TMPDIR/3 -raft :8082 -api :9092 -join :8080
```

Now it's possible to write a key-value pair to any member of the cluster and likewise retrieve it from any member.

### Fault Tolerance

To test cluster recovery, write a value "foo" to key "foo":
```sh
curl -L http://127.0.0.1:9090/ -X PUT -d '{"Key":"foo", "Value":"foo"}'
```

Next, stop a node (9092) and replace the value with "bar" to check cluster availability:

```sh
curl -L http://127.0.0.1:9090/ -X PUT -d '{"Key":"foo", "Value":"bar"}'
curl -L http://127.0.0.1:9090/foo
```

Finally, bring the node back up and verify it recovers with the updated value "bar":
```sh
curl -L http://127.0.0.1:9092/foo
```

### cluster reconfiguration

Nodes can be added, removed, updated from a running cluster,
lets remove node using requests to the REST API.

First, list all raft cluster nodes, and get node id.
```sh
curl -L http://127.0.0.1:9090/mgmt/nodes
```

Then remove a node using a DELETE request:
```sh
curl -L http://127.0.0.1:9090/<ID> -X DELETE
```
Node will shut itself down once the cluster has processed this request.

