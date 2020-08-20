
```go
// Backend represents the backend server, which is used to hold the datastore
// and coordinating the daemons
// backend 服务 运行内嵌的etcd存储服务 并且协调daemons之间的操作;
type Backend struct {
	Client                 *clientv3.Client         // etcd client
	Daemons                []daemon.Daemon          // daemon 列表，Daemon 是一个接口类型， 用于管理一组goroutine
	Etcd                   *etcd.Etcd               // etcd 对象
	Store                  store.Store              // 存储对象
	EventStore             EventStoreUpdater        // 事件更新落盘
	GraphQLService         *graphql.Service         // graphql 服务
	SecretsProviderManager *secrets.ProviderManager // 密码提供管理者
	HealthRouter           *routers.HealthRouter    // 监控路由
	EtcdClientTLSConfig    *tls.Config              // etcd tls

	ctx       context.Context    // cancel context 监听外部中断信号
	runCtx    context.Context    // 运行时ctx
	runCancel context.CancelFunc // 运行时cancel ctx
	cfg       *Config            // 配置
}
```


A Daemon is a managed subprocess comprised of one or more goroutines that can be managed via a consistent, simple interface.

> 守护进程(Daemon)是由一个或多个goroutine组成的托管子进程

- **Daemons**
	1.  backendID 
	2.  WizardBus
	3.  pipeline 
	4.  event 
	5.  scheduler 
	6.  agent 
	7.  keepalive 
	8.  api 
	9.  dashboard

backendID   是为便于识别一个sensu backend 的自定义类型实例

```go
// BackendIDGetter is a type that facilitates identifying a sensu backend.
type BackendIDGetter struct {
	id     int64
	wg     sync.WaitGroup
	ctx    context.Context
	client BackendIDGetterClient
	errors chan error
}
```
pipeline 将Sensu 事件(events) 通过管道送到对应的handler;

示例流程: filter -> mutator -> handler 

handler 的配置决定使用哪些filters和mutator
```go
// Pipelined handles incoming Sensu events and puts them through a
// Sensu event pipeline, i.e. filter -> mutator -> handler. The Sensu
// handler configuration determines which Sensu filters and mutator
// are used.
type Pipelined struct {
	assetGetter            asset.Getter //assetGetter
	stopping               chan struct{}
	running                *atomic.Value
	wg                     *sync.WaitGroup
	errChan                chan error
	eventChan              chan interface{}
	subscription           messaging.Subscription
	store                  store.Store // etcd.store
	bus                    messaging.MessageBus //WizardBus
	extensionExecutor      pipeline.ExtensionExecutorGetterFunc //GRPCExtensionExecutor
	executor               command.Executor 
	workerCount            int
	storeTimeout           time.Duration
	secretsProviderManager *secrets.ProviderManager  //SecretsProviderManager
	backendEntity          *corev2.Entity //backendEntity
	LicenseGetter          licensing.Getter
}
```
Eventd handles incoming sensu events and stores them in etcd.

> Eventd 将events 存储到etcd中

```go
type Eventd struct {
	ctx             context.Context
	cancel          context.CancelFunc
	store           store.Store //eventstore.client
	eventStore      store.EventStore //event.store.client.proxy 防止gc
	bus             messaging.MessageBus //wizardBus
	workerCount     int
	livenessFactory liveness.Factory // etcd.client 第一次使用后被缓存起来
	eventChan       chan interface{}
	subscription    messaging.Subscription
	errChan         chan error
	mu              *sync.Mutex
	shutdownChan    chan struct{}
	wg              *sync.WaitGroup
	Logger          Logger
	silencedCache   *cache.Resource
	storeTimeout    time.Duration
}
```

Schedulerd 定期检查每个配置的请求 并将其发布到消息总线上;

```go
// Schedulerd handles scheduling check requests for each check's
// configured interval and publishing to the message bus.
type Schedulerd struct {
	store                  store.Store
	queueGetter            types.QueueGetter
	bus                    messaging.MessageBus
	checkWatcher           *CheckWatcher
	adhocRequestExecutor   *AdhocRequestExecutor
	ctx                    context.Context
	cancel                 context.CancelFunc
	errChan                chan error
	ringPool               *ringv2.Pool
	entityCache            *cache.Resource
	secretsProviderManager *secrets.ProviderManager
}
```

Agentd 是后端HTTP API 模块

```go
// Agentd is the backend HTTP API.
type Agentd struct {
	// Host is the hostname Agentd is running on.
	Host string

	// Port is the port Agentd is running on.
	Port int

	stopping     chan struct{}
	running      *atomic.Value
	wg           *sync.WaitGroup
	errChan      chan error
	httpServer   *http.Server
	store        store.Store
	bus          messaging.MessageBus
	tls          *corev2.TLSOptions
	ringPool     *ringv2.Pool
	ctx          context.Context
	cancel       context.CancelFunc
	writeTimeout int
}
```

Keepalived 负责为entities 监听并记录它的keepalive events

```go
// Keepalived is responsible for monitoring keepalive events and recording
// keepalives for entities.
type Keepalived struct {
	bus                   messaging.MessageBus
	workerCount           int
	store                 store.Store
	eventStore            store.EventStore
	deregistrationHandler string
	mu                    *sync.Mutex
	wg                    *sync.WaitGroup
	keepaliveChan         chan interface{}
	subscription          messaging.Subscription
	errChan               chan error
	livenessFactory       liveness.Factory
	ringPool              *ringv2.Pool
	ctx                   context.Context
	cancel                context.CancelFunc
	storeTimeout          time.Duration
}
```

apid  backend http api 服务

```go
	// Initialize apid
	apidConfig := apid.Config{
		ListenAddress:       config.APIListenAddress,
		URL:                 config.APIURL,
		Bus:                 bus,
		Store:               stor,
		EventStore:          eventStoreProxy,
		QueueGetter:         queueGetter,
		TLS:                 config.TLS,
		Cluster:             b.Client.Cluster,
		EtcdClientTLSConfig: etcdClientTLSConfig,
		Authenticator:       authenticator,
		ClusterVersion:      clusterVersion,
		GraphQLService:      b.GraphQLService,
		HealthRouter:        b.HealthRouter,
	}
```

dashboard 服务初始化

```go
// Dashboardd represents the dashboard daemon
type Dashboardd struct {
	stopping   chan struct{}
	running    *atomic.Value
	wg         *sync.WaitGroup
	errChan    chan error
	httpServer *http.Server
	logger     *logrus.Entry

	Config
	Assets *asset.Collection

	AuthenticationSubrouter *mux.Router
	CoreSubrouter           *mux.Router
	GraphQLSubrouter        *mux.Router
}
```
