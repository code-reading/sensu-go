
##### problem01 - etcd 版本不对
> sensu-go v6.0 要求etcd版本> v3.4以上;

./sensu-backend start -c backend.yaml

error:
```diff
- {"component":"etcd","level":"fatal","msg":"cluster cannot be downgraded (current version: 3.3.22 is lower than determined cluster version: 3.4).","pkg":"etcdserver/membership","time":"2020-08-18T10:12:35+08:00"}
```

按照下面的指令升级etcd 版本为3.4

```diff
➜ cat go.mod|grep etcd
        github.com/coreos/etcd v3.3.22+incompatible

+ ➜ go mod edit -require=github.com/coreos/etcd@v3.4.0

➜ cat go.mod|grep etcd
        github.com/coreos/etcd v3.4.0
        go.etcd.io/bbolt v1.3.4
```

- [go1.13 mod 实践和常见问题](https://www.jianshu.com/p/9e6375da325d)
- [Etcd使用go module的灾难](https://colobu.com/2020/04/09/accidents-of-etcd-and-go-module/)
- [Go 解决国内下载 go get golang.org/x 包失败 非原创](https://learnku.com/articles/31559)
