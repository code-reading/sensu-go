# Sensu Go

## Release 6.0 code reading notes

Sensu 是一款开源的基础设施和分布式应用程序监控解决方案；

> Sensu is an open source monitoring tool for ephemeral infrastructure
and distributed applications.

Sensu 是一款基于Agent的监控系统， 内置自动发现功能， 非常适合云计算环境;

> It is an agent based monitoring **system** with built-in auto-discovery, making it very well-suited for cloud environments.

Sensu 通过服务检测 来监控服务的健康状态和收集遥测数据;

> Sensu uses service checks to monitor service health and collect telemetry data.

Sensu 同时提供了大量友好的OpenAPI, 可以通过API 进行配置， 外部数据传输，及Sensu 数据访问等;
> It also has a number of well defined APIs for configuration, external data input, and to provide access to Sensu's data.

Sensu 可扩展性非常强， 被称之为"监控路由器";

> Sensu is extremely extensible and is commonly referred to as "the monitoring router".

To learn more about Sensu, [please visit the
website](https://sensu.io/) and [read the documentation](https://docs.sensu.io/sensu-go/latest/).

## What is Sensu Go?

Sensu Go is a complete rewrite of Sensu in Go, with new capabilities
and reduced operational overhead. It eliminates several sources of
friction for new and experienced Sensu users.

The original Sensu required external services like Redis or RabbitMQ.
Sensu Go can rely on an embedded etcd datastore for persistence, making
the product easier to get started with. External etcd services can also be
used, in the event that you already have them deployed.

Sensu Go replaces Ruby expressions with JavaScript filter expressions, by
embedding a JavaScript interpreter.

Unlike the original Sensu, Sensu Go events are always handled, unless
explicitly filtered.

## Installation

Sensu Go installer packages are available for a number of computing
platforms (e.g. Debian/Ubuntu, RHEL/Centos, etc), but the easiest way
to get started is with the official Docker image, sensu/sensu.

See the [installation documentation](https://docs.sensu.io/sensu-go/latest/installation/install-sensu/) to get started.

### Building from source

The various components of Sensu Go can be manually built from this repository.
You will first need [Go 1.13.3](https://golang.org/doc/install#install)
installed. Then, you should clone this repository **outside** of the GOPATH
since Sensu Go uses [Go Modules](https://github.com/golang/go/wiki/Modules):
```
$ git clone https://github.com/sensu/sensu-go.git
$ cd sensu-go
```

To compile and then run Sensu Go within a single step:
```
$ go run ./cmd/sensu-agent
$ go run ./cmd/sensu-backend
$ go run ./cmd/sensuctl
```

To build Sensu Go binaries and save them into the `bin/` directory:
```
$ go build -o bin/sensu-agent ./cmd/sensu-agent
$ go build -o bin/sensu-backend ./cmd/sensu-backend
$ go build -o bin/sensuctl ./cmd/sensuctl
```

To build Sensu Go binaries with the version information:
```
# sensu-agent
$ go build -ldflags '-X "github.com/sensu/sensu-go/version.Version=5.14.0" -X "github.com/sensu/sensu-go/version.BuildDate=2019-10-08" -X "github.com/sensu/sensu-go/version.BuildSHA='`git rev-parse HEAD`'"' -o bin/sensu-agent ./cmd/sensu-agent

# sensu-backend
$ go build -ldflags '-X "github.com/sensu/sensu-go/version.Version=5.14.0" -X "github.com/sensu/sensu-go/version.BuildDate=2019-10-08" -X "github.com/sensu/sensu-go/version.BuildSHA='`git rev-parse HEAD`'"' -o bin/sensu-backend ./cmd/sensu-backend

# sensuctl
$ go build -ldflags '-X "github.com/sensu/sensu-go/version.Version=5.14.0" -X "github.com/sensu/sensu-go/version.BuildDate=2019-10-08" -X "github.com/sensu/sensu-go/version.BuildSHA='`git rev-parse HEAD`'"' -o bin/sensuctl ./cmd/sensuctl
```

