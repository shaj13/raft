# Multi-Raft cluster 

Multi-Raft cluster is an example usage of raft library. It provides a simple REST API for a key-value store cluster backed by the [Raft][raft] consensus algorithm.
Here Multi-Raft only means that each node may be participating in multiple raft consensus groups.

Scales raft into multiple raft groups requires data sharding and gossip protocol to announce metadata to the cluster that doesn't have to be consistent. 
for simplicity, the example does not implement data sharding 
or gossip protocol instead you need to do it manually by the rest API. 
E.g, in production the application, shard data based on the availability of disk space and cluster group members, in this example you need to tell the application on which group to save the data. 

[raft]: http://raftconsensus.github.io/

## Getting Started

### Building Multi-Raft example

get `multiraft` example

```sh
mkdir -p raftexample
cd raftexample
GOBIN=${PWD} go get github.com/franklee0817/raft/_examples/multiraft
```

### Running single node Multi-Raft

First start a single-member cluster of multiraft:

```sh
./multiraft -state_dir=$TMPDIR/1 -raft :8080 -api :9090 -id 1 
```

Each raftcluster process maintains a 1..N raft instance and a key-value server's.
raft server address (-raft), state dir (-state_dir), and http key-value server address (-api) are passed through the command line.

Next, store a value ("world") to a key ("hello"):

```
curl -L http://127.0.0.1:9090/1 -X PUT -d '{"Key":"hello", "Value":"world"}'
```

Finally, retrieve the stored key:

```
curl -L http://127.0.0.1:9090/1/hello
```

### Running a single local cluster
Lets bring two additional raftcluster instances.

```sh
./multiraft -state_dir $TMPDIR/2 -raft :8081 -api :9091 -join :8080 -id 2
./multiraft -state_dir $TMPDIR/3 -raft :8082 -api :9092 -join :8080 -id 3 
```

Now it's possible to write a key-value pair to any member of the cluster and likewise retrieve it from any member.

### Fault Tolerance

To test cluster recovery, write a value "foo" to key "foo":
```sh
curl -L http://127.0.0.1:9090/1 -X PUT -d '{"Key":"foo", "Value":"foo"}'
```

Next, stop a node (9092) and replace the value with "bar" to check cluster availability:

```sh
curl -L http://127.0.0.1:9090/1 -X PUT -d '{"Key":"foo", "Value":"bar"}'
curl -L http://127.0.0.1:9090/1/foo
```

Finally, bring the node back up and verify it recovers with the updated value "bar":
```sh
curl -L http://127.0.0.1:9092/1/foo
```


### Running a multi-raft local cluster
First Lets creaet a new raft group.
```sh
curl -L http://127.0.0.1:9090/mgmt/groups -d '{"GroupID":2, "IDs":[1,2],"JoinAddr":":8080"}' -XPUT
```

Now Lets bring one additional multiraft instances to the new group. 
```sh 
./multiraft -state_dir $TMPDIR/4 -raft :8083 -api :9093 -join :8080 -initial_group_id 2 -id 4
```

Now it's possible to write a key-value pair to any group member of the cluster and likewise retrieve it from any group member (data sharding).

Lets write data to group one.
So it can be replicated to node 1,2,3.
```sh 
curl -L http://127.0.0.1:9090/1 -X PUT -d '{"Key":"mykey", "Value":"1"}'
```

This will fail cause node 4 is not part of group 1, in the real-world application node 4 redirect the user to group 1, this is where gossip protocol is used.
```sh 
curl -L http://127.0.0.1:9093/1/mykey
```


Lets write data to group two.
So it can be replicated to node 1,2,4.
```sh 
curl -L http://127.0.0.1:9090/2 -X PUT -d '{"Key":"mykey", "Value":"2"}'
```

```sh 
curl -L http://127.0.0.1:9093/2/mykey
```

```sh 
curl -L http://127.0.0.1:9091/2/mykey
```




### cluster reconfiguration
Nodes can be added, removed, updated from a running cluster,
lets remove node using requests to the REST API.

First, list all raft cluster group nodes, and get node id.
```sh
curl -L http://127.0.0.1:9090/1/mgmt/nodes
```

Then remove a node using a DELETE request:
```sh
curl -L http://127.0.0.1:9090/1/mgmt/nodes/3 -X DELETE
```
Node will shut itself down once the cluster has processed this request.
