
##### sensu-backend init

```
[root@host-178-104-163-55 monitor]# ./sensu-backend init
{"level":"warn","ts":"2020-08-18T14:04:54.299+0800","caller":"clientv3/retry_interceptor.go:61","msg":"retrying of unary invoker failed","target":"passthrough:///http://localhost:2379","attempt":0,"error":"rpc error: code = DeadlineExceeded desc = latest connection error: connection closed"}
{"component":"backend","error":"context deadline exceeded","level":"fatal","msg":"error executing sensu-backend","time":"2020-08-18T14:04:54+08:00"}
[root@host-178-104-163-55 monitor]# ./sensu-backend init -c backend.yml
{"component":"backend.seeds","level":"info","msg":"seeding etcd store with intial data","time":"2020-08-18T14:05:11+08:00"}
[root@host-178-104-163-55 monitor]#
```
