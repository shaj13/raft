# raftkit

### TODO 
- [ ] add node docs 
- [ ] add tcp rpc 
- [ ] check the conf change v2 in etcd
- [ ] Repair wall 
- [ ] test forceSnapshot


## Test cases 
- [ ] create 1 mem + add data + snapshot + join new meem + drop mem = force new cluster
- [ ] create 1 mem +  join new meem + drop mem = force new cluster
- [ ] sanity check staging member 
- [ ] remove nodes + restart
- [ ] demote voter 
- [ ] promote member + on non leader 
- [ ] disable forwarding 
- [ ] test shutdown gracfull and non gracefull (sanity check)




