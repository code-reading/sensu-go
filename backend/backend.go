package backend

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"sync"
	"syscall"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/pkg/transport"
	"github.com/prometheus/client_golang/prometheus"
	corev2 "github.com/sensu/sensu-go/api/core/v2"
	"github.com/sensu/sensu-go/asset"
	"github.com/sensu/sensu-go/backend/agentd"
	"github.com/sensu/sensu-go/backend/api"
	"github.com/sensu/sensu-go/backend/apid"
	"github.com/sensu/sensu-go/backend/apid/actions"
	"github.com/sensu/sensu-go/backend/apid/graphql"
	"github.com/sensu/sensu-go/backend/apid/routers"
	"github.com/sensu/sensu-go/backend/authentication"
	"github.com/sensu/sensu-go/backend/authentication/jwt"
	"github.com/sensu/sensu-go/backend/authentication/providers/basic"
	"github.com/sensu/sensu-go/backend/authorization/rbac"
	"github.com/sensu/sensu-go/backend/daemon"
	"github.com/sensu/sensu-go/backend/dashboardd"
	"github.com/sensu/sensu-go/backend/etcd"
	"github.com/sensu/sensu-go/backend/eventd"
	"github.com/sensu/sensu-go/backend/keepalived"
	"github.com/sensu/sensu-go/backend/liveness"
	"github.com/sensu/sensu-go/backend/messaging"
	"github.com/sensu/sensu-go/backend/pipelined"
	"github.com/sensu/sensu-go/backend/queue"
	"github.com/sensu/sensu-go/backend/ringv2"
	"github.com/sensu/sensu-go/backend/schedulerd"
	"github.com/sensu/sensu-go/backend/secrets"
	"github.com/sensu/sensu-go/backend/store"
	etcdstore "github.com/sensu/sensu-go/backend/store/etcd"
	"github.com/sensu/sensu-go/backend/tessend"
	"github.com/sensu/sensu-go/rpc"
	"github.com/sensu/sensu-go/system"
	"github.com/sensu/sensu-go/util/retry"
	"github.com/spf13/viper"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
)

type ErrStartup struct {
	Err  error
	Name string
}

func (e ErrStartup) Error() string {
	return fmt.Sprintf("error starting %s: %s", e.Name, e.Err)
}

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

// EventStoreUpdater offers a way to update an event store to a different
// implementation in-place.
type EventStoreUpdater interface {
	UpdateEventStore(to store.EventStore)
}

func newClient(ctx context.Context, config *Config, backend *Backend) (*clientv3.Client, error) {
	// NoEmbedEtcd=true, 表示使用外部etcd服务;
	if config.NoEmbedEtcd {
		logger.Info("dialing etcd server")
		tlsInfo := (transport.TLSInfo)(config.EtcdClientTLSInfo)
		tlsConfig, err := tlsInfo.ClientConfig()
		if err != nil {
			return nil, err
		}

		clientURLs := config.EtcdClientURLs
		if len(clientURLs) == 0 {
			clientURLs = config.EtcdAdvertiseClientURLs
		}

		// Don't start up an embedded etcd, return a client that connects to an
		// external etcd instead.
		// 实例化一个连接外部etcd服务的etcd client实例
		client, err := clientv3.New(clientv3.Config{
			Endpoints:   clientURLs,
			DialTimeout: 5 * time.Second,
			TLS:         tlsConfig,
			DialOptions: []grpc.DialOption{
				grpc.WithBlock(),
			},
		})
		if err != nil {
			return nil, err
		}
		if _, err := client.Get(ctx, "/sensu.io"); err != nil {
			return nil, err
		}
		// 返回外部的etcd.client 实例
		return client, nil
	}

	// Initialize and start etcd, because we'll need to provide an etcd client to
	// the Wizard bus, which requires etcd to be started.
	cfg := etcd.NewConfig()
	cfg.DataDir = config.StateDir
	cfg.ListenClientURLs = config.EtcdListenClientURLs
	cfg.ListenPeerURLs = config.EtcdListenPeerURLs
	cfg.InitialCluster = config.EtcdInitialCluster
	cfg.InitialClusterState = config.EtcdInitialClusterState
	cfg.InitialAdvertisePeerURLs = config.EtcdInitialAdvertisePeerURLs
	cfg.AdvertiseClientURLs = config.EtcdAdvertiseClientURLs
	cfg.Discovery = config.EtcdDiscovery
	cfg.DiscoverySrv = config.EtcdDiscoverySrv
	cfg.Name = config.EtcdName

	// Heartbeat interval
	if config.EtcdHeartbeatInterval > 0 {
		cfg.TickMs = config.EtcdHeartbeatInterval
	}

	// Election timeout
	if config.EtcdElectionTimeout > 0 {
		cfg.ElectionMs = config.EtcdElectionTimeout
	}

	// Etcd TLS config
	cfg.ClientTLSInfo = config.EtcdClientTLSInfo
	cfg.PeerTLSInfo = config.EtcdPeerTLSInfo
	cfg.CipherSuites = config.EtcdCipherSuites

	if config.EtcdQuotaBackendBytes != 0 {
		cfg.QuotaBackendBytes = config.EtcdQuotaBackendBytes
	}
	if config.EtcdMaxRequestBytes != 0 {
		cfg.MaxRequestBytes = config.EtcdMaxRequestBytes
	}

	// Start etcd
	e, err := etcd.NewEtcd(cfg)
	if err != nil {
		return nil, fmt.Errorf("error starting etcd: %s", err)
	}
	// etcd 服务器实例
	backend.Etcd = e

	// Create an etcd client
	var client *clientv3.Client
	if config.EtcdUseEmbeddedClient {
		// 当设置了EtcdUseEmbeddedClient 会生成一个测试用的etcd client
		client = e.NewEmbeddedClient()
	} else {
		// 正常流程走这里;
		cl, err := e.NewClientContext(backend.runCtx)
		if err != nil {
			return nil, err
		}
		client = cl
	}
	// 测试新建的etcd.client 是否可以用, etcd服务正常时, 返回的err为nil
	if _, err := client.Get(ctx, "/sensu.io"); err != nil {
		return nil, err
	}
	return client, nil
}

// Initialize instantiates a Backend struct with the provided config, by
// configuring etcd and establishing a list of daemons, which constitute our
// backend. The daemons will later be started according to their position in the
// b.Daemons list, and stopped in reverse order

// 初始化一个backend 实例;
func Initialize(ctx context.Context, config *Config) (*Backend, error) {
	var err error
	// Initialize a Backend struct
	b := &Backend{cfg: config}

	b.ctx = ctx
	b.runCtx, b.runCancel = context.WithCancel(b.ctx)

	// 初始化一个etcd client
	b.Client, err = newClient(b.RunContext(), config, b)
	if err != nil {
		return nil, err
	}

	// Create the store, which lives on top of etcd
	// 默认创建一个 /sensu.io/keepalives/default 的 etcd 命名空间
	// 所有的数据都存储在这个空间下;
	stor := etcdstore.NewStore(b.Client, config.EtcdName)
	b.Store = stor

	// 存储一个 sensu cluster id
	if _, err := stor.GetClusterID(b.RunContext()); err != nil {
		return nil, err
	}

	// Initialize the JWT secret. This method is idempotent and needs to be ran
	// at every startup so the JWT signatures remain valid
	if err := jwt.InitSecret(b.Store); err != nil {
		return nil, err
	}

	// 初始化一个事件存储代理， 防止在未被引用时被GC掉;
	eventStoreProxy := store.NewEventStoreProxy(stor)
	b.EventStore = eventStoreProxy

	logger.Debug("Registering backend...")

	// 生成一个backend lease标识, 同时保存在etcd中;
	// 返回一个带etcd.client的backendGetter 即backendID
	backendID := etcd.NewBackendIDGetter(b.RunContext(), b.Client)
	logger.Debug("Done registering backend.")
	// 添加到b.Daemons 队列中, 这个daemon 其实是一组goroutine的管理者;
	b.Daemons = append(b.Daemons, backendID)

	// Initialize an etcd getter
	// 初始化一个带etcd.client 和backend 的 queue 实例
	queueGetter := queue.EtcdGetter{Client: b.Client, BackendIDGetter: backendID}

	// Initialize the bus
	bus, err := messaging.NewWizardBus(messaging.WizardBusConfig{})
	if err != nil {
		return nil, fmt.Errorf("error initializing %s: %s", bus.Name(), err)
	}
	// daemon 是一组goroutine的管理接口
	b.Daemons = append(b.Daemons, bus)

	// Initialize asset manager
	// 初始化event管理者 backend
	backendEntity := b.getBackendEntity(config)
	logger.WithField("entity", backendEntity).Info("backend entity information")
	// assetManager
	assetManager := asset.NewManager(config.CacheDir, backendEntity, &sync.WaitGroup{})
	limit := b.cfg.AssetsRateLimit
	if limit == 0 {
		limit = rate.Limit(asset.DefaultAssetsRateLimit)
	}
	assetGetter, err := assetManager.StartAssetManager(b.RunContext(), rate.NewLimiter(limit, b.cfg.AssetsBurstLimit))
	if err != nil {
		return nil, fmt.Errorf("error initializing asset manager: %s", err)
	}

	// Initialize the secrets provider manager
	b.SecretsProviderManager = secrets.NewProviderManager()

	// Initialize pipelined
	// pipeline 负责将events 送到对应的handler处理
	pipeline, err := pipelined.New(pipelined.Config{
		Store:                   stor,                         // etcd.store 客户端
		Bus:                     bus,                          //bus
		ExtensionExecutorGetter: rpc.NewGRPCExtensionExecutor, //grpc executor
		AssetGetter:             assetGetter,                  // asset getter
		BufferSize:              viper.GetInt(FlagPipelinedBufferSize),
		WorkerCount:             viper.GetInt(FlagPipelinedWorkers),
		StoreTimeout:            2 * time.Minute, // 存储超时2分钟
		SecretsProviderManager:  b.SecretsProviderManager,
		BackendEntity:           backendEntity,
	})
	if err != nil {
		return nil, fmt.Errorf("error initializing %s: %s", pipeline.Name(), err)
	}
	b.Daemons = append(b.Daemons, pipeline)

	// Initialize eventd
	// eventd 负责将events 存储到etcd中
	event, err := eventd.New(
		b.RunContext(),
		eventd.Config{
			Store:           stor,            //etcd.store client
			EventStore:      eventStoreProxy, //etcd.client proxy
			Bus:             bus,
			LivenessFactory: liveness.EtcdFactory(b.RunContext(), b.Client), // etcd.client, 第一次使用后被缓存起来
			Client:          b.Client,
			BufferSize:      viper.GetInt(FlagEventdBufferSize),
			WorkerCount:     viper.GetInt(FlagEventdWorkers),
			StoreTimeout:    2 * time.Minute, //存储超时2分钟
		},
	)
	if err != nil {
		return nil, fmt.Errorf("error initializing %s: %s", event.Name(), err)
	}
	b.Daemons = append(b.Daemons, event)

	// etcd 连接池, ring queue
	ringPool := ringv2.NewPool(b.Client)

	// Initialize schedulerd
	// 定期检查配置的请求, 并将其发布到消息总线;
	scheduler, err := schedulerd.New(
		b.RunContext(),
		schedulerd.Config{
			Store:                  stor,
			Bus:                    bus,
			QueueGetter:            queueGetter,
			RingPool:               ringPool,
			Client:                 b.Client,
			SecretsProviderManager: b.SecretsProviderManager,
		})
	if err != nil {
		return nil, fmt.Errorf("error initializing %s: %s", scheduler.Name(), err)
	}
	b.Daemons = append(b.Daemons, scheduler)

	// Use the common TLS flags for agentd if wasn't explicitely configured with
	// its own TLS configuration
	if config.TLS != nil && config.AgentTLSOptions == nil {
		config.AgentTLSOptions = config.TLS
	}

	// Initialize agentd
	// 后端HTTP API 模块
	// 负责backend 与agent 之间的请求逻辑
	agent, err := agentd.New(agentd.Config{
		Host:         config.AgentHost,
		Port:         config.AgentPort,
		Bus:          bus,
		Store:        stor,
		TLS:          config.AgentTLSOptions,
		RingPool:     ringPool,
		WriteTimeout: config.AgentWriteTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("error initializing %s: %s", agent.Name(), err)
	}
	b.Daemons = append(b.Daemons, agent)

	// Initialize keepalived
	// 监听entity 的keepalive 事件并记录;
	keepalive, err := keepalived.New(keepalived.Config{
		DeregistrationHandler: config.DeregistrationHandler,
		Bus:                   bus,
		Store:                 stor,
		EventStore:            eventStoreProxy,
		LivenessFactory:       liveness.EtcdFactory(b.RunContext(), b.Client),
		RingPool:              ringPool,
		BufferSize:            viper.GetInt(FlagKeepalivedBufferSize),
		WorkerCount:           viper.GetInt(FlagKeepalivedWorkers),
		StoreTimeout:          2 * time.Minute,
	})
	if err != nil {
		return nil, fmt.Errorf("error initializing %s: %s", keepalive.Name(), err)
	}
	b.Daemons = append(b.Daemons, keepalive)

	// Prepare the etcd client TLS config
	etcdClientTLSInfo := (transport.TLSInfo)(config.EtcdClientTLSInfo)
	etcdClientTLSConfig, err := etcdClientTLSInfo.ClientConfig()
	if err != nil {
		return nil, err
	}
	b.EtcdClientTLSConfig = etcdClientTLSConfig

	// Prepare the authentication providers
	authenticator := &authentication.Authenticator{}
	basic := &basic.Provider{
		ObjectMeta: corev2.ObjectMeta{Name: basic.Type},
		Store:      stor,
	}
	authenticator.AddProvider(basic)

	var clusterVersion string
	// only retrieve the cluster version if etcd is embedded
	if !config.NoEmbedEtcd {
		clusterVersion = b.Etcd.GetClusterVersion()
	}

	// Load the JWT key pair
	if err := jwt.LoadKeyPair(viper.GetString(FlagJWTPrivateKeyFile), viper.GetString(FlagJWTPublicKeyFile)); err != nil {
		logger.WithError(err).Error("could not load the key pair for the JWT signature")
	}

	// Initialize the health router
	// etcd 健康情况
	b.HealthRouter = routers.NewHealthRouter(actions.NewHealthController(stor, b.Client.Cluster, b.EtcdClientTLSConfig))

	// Initialize GraphQL service
	auth := &rbac.Authorizer{Store: stor}
	// graphql service
	b.GraphQLService, err = graphql.NewService(graphql.ServiceConfig{
		AssetClient:       api.NewAssetClient(stor, auth),                                                // web-ui静态资源加载;
		CheckClient:       api.NewCheckClient(stor, actions.NewCheckController(stor, queueGetter), auth), // CheckClient is an API client for check configuration api 进行curd 是否授权检查的API 客服端
		EntityClient:      api.NewEntityClient(stor, eventStoreProxy, auth),                              // entity curd API client
		EventClient:       api.NewEventClient(eventStoreProxy, auth, bus),                                // event curd api client
		EventFilterClient: api.NewEventFilterClient(stor, auth),                                          // event curd filter api client
		HandlerClient:     api.NewHandlerClient(stor, auth),                                              // handlers curd api client
		HealthController:  actions.NewHealthController(stor, b.Client.Cluster, etcdClientTLSConfig),      // 查看etcd 的健康情况
		MutatorClient:     api.NewMutatorClient(stor, auth),                                              //mutator curd api client
		SilencedClient:    api.NewSilencedClient(stor, auth),                                             // silence check 的curd api client
		NamespaceClient:   api.NewNamespaceClient(stor, auth),                                            // namespace curd api client
		HookClient:        api.NewHookConfigClient(stor, auth),                                           // check hooks 的curd api client
		UserClient:        api.NewUserClient(stor, auth),                                                 // user curd api client
		RBACClient:        api.NewRBACClient(stor, auth),                                                 // rbac curd api client (比较复杂, 分role 绑定 和 cluster role 绑定)
		VersionController: actions.NewVersionController(clusterVersion),                                  //获取版本信息
		MetricGatherer:    prometheus.DefaultGatherer,                                                    // prometheus metrics
		GenericClient:     &api.GenericClient{Store: stor, Auth: auth},                                   // 通用api client  本身有auth, 有资源的curd api 操作
	})
	if err != nil {
		return nil, fmt.Errorf("error initializing graphql.Service: %s", err)
	}

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
	//backend http api 服务
	api, err := apid.New(apidConfig)
	if err != nil {
		return nil, fmt.Errorf("error initializing %s: %s", api.Name(), err)
	}
	b.Daemons = append(b.Daemons, api)

	// Initialize tessend
	tessen, err := tessend.New(
		b.RunContext(),
		tessend.Config{
			Store:      stor,
			EventStore: eventStoreProxy,
			RingPool:   ringPool,
			Client:     b.Client,
			Bus:        bus,
		})
	if err != nil {
		return nil, fmt.Errorf("error initializing %s: %s", tessen.Name(), err)
	}
	b.Daemons = append(b.Daemons, tessen)

	// Initialize dashboardd TLS config
	var dashboardTLSConfig *corev2.TLSOptions

	// Always use dashboard tls options when they are specified
	if config.DashboardTLSCertFile != "" && config.DashboardTLSKeyFile != "" {
		dashboardTLSConfig = &corev2.TLSOptions{
			CertFile: config.DashboardTLSCertFile,
			KeyFile:  config.DashboardTLSKeyFile,
		}
	} else if config.TLS != nil {
		// use apid tls config if no dashboard tls options are specified
		dashboardTLSConfig = &corev2.TLSOptions{
			CertFile: config.TLS.GetCertFile(),
			KeyFile:  config.TLS.GetKeyFile(),
		}
	}
	// 启动一个dashboard 服务
	dashboard, err := dashboardd.New(dashboardd.Config{
		APIDConfig: apidConfig,
		Host:       config.DashboardHost,
		Port:       config.DashboardPort,
		TLS:        dashboardTLSConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("error initializing %s: %s", dashboard.Name(), err)
	}
	b.Daemons = append(b.Daemons, dashboard)

	return b, nil
}

func (b *Backend) runOnce(sighup <-chan os.Signal) error {
	eCloser := b.EventStore.(closer)
	defer eCloser.Close()

	var derr error

	eg := errGroup{
		out: make(chan error),
	}

	defer eg.WaitStop()

	if b.Etcd != nil {
		defer func() {
			logger.Info("shutting down etcd")
			if err := recover(); err != nil {
				trace := string(debug.Stack())
				logger.WithField("panic", trace).WithError(fmt.Errorf("%s", err)).
					Error("recovering from panic due to error, shutting down etcd")
			}
			err := b.Etcd.Shutdown()
			if derr == nil {
				derr = err
			}
		}()
	}

	sg := stopGroup{}

	// Loop across the daemons in order to start them, then add them to our groups
	for _, d := range b.Daemons {
		if err := d.Start(); err != nil {
			_ = sg.Stop()
			return ErrStartup{Err: err, Name: d.Name()}
		}

		// Add the daemon to our errGroup
		eg.daemons = append(eg.daemons, d)

		// Add the daemon to our stopGroup
		sg = append(sg, d)
	}

	// Reverse the order of our stopGroup so daemons are stopped in the proper
	// order (last one started is first one stopped)
	for i := len(sg)/2 - 1; i >= 0; i-- {
		opp := len(sg) - 1 - i
		sg[i], sg[opp] = sg[opp], sg[i]
	}

	if b.Etcd != nil {
		// Add etcd to our errGroup, since it's not included in the daemon list
		eg.daemons = append(eg.daemons, b.Etcd)
	}

	errCtx, errCancel := context.WithCancel(b.RunContext())
	defer errCancel()
	eg.Go(errCtx)

	select {
	case err := <-eg.Err():
		logger.WithError(err).Error("backend stopped working and is restarting")
	case <-b.RunContext().Done():
		logger.Info("backend shutting down")
	case <-sighup:
		logger.Warn("got SIGHUP, restarting")
	}
	if err := sg.Stop(); err != nil {
		if derr == nil {
			derr = err
		}
	}
	if derr == nil {
		derr = b.RunContext().Err()
	}

	return derr
}

type closer interface {
	Close() error
}

// RunContext returns the context for the current run of the backend.
func (b *Backend) RunContext() context.Context {
	return b.runCtx
}

// RunWithInitializer is like Run but accepts an initialization function to use
// for initialization, instead of using the default Initialize().
func (b *Backend) RunWithInitializer(initialize func(context.Context, *Config) (*Backend, error)) error {
	// we allow inErrChan to leak to avoid panics from other
	// goroutines writing errors to either after shutdown has been initiated.
	backoff := retry.ExponentialBackoff{
		Ctx:                  b.ctx,
		InitialDelayInterval: time.Second,
		MaxDelayInterval:     time.Second,
		Multiplier:           1,
	}

	sighup := make(chan os.Signal, 1)
	signal.Notify(sighup, syscall.SIGHUP)

	err := backoff.Retry(func(int) (bool, error) {
		// run
		err := b.runOnce(sighup)
		b.Stop()
		if err != nil {
			if b.ctx.Err() != nil {
				logger.Warn("shutting down")
				return true, b.ctx.Err()
			}
			logger.Error(err)
			if _, ok := err.(ErrStartup); ok {
				return true, err
			}
		}

		_ = b.Client.Close()

		// 重新初始化一个backend 实例， 避免上次stop后 带入的副作用;
		// Yes, two levels of retry... this could improve. Unfortunately Intialize()
		// is called elsewhere.
		err = backoff.Retry(func(int) (bool, error) {
			backend, err := initialize(b.ctx, b.cfg)
			if err != nil && err != context.Canceled {
				logger.Error(err)
				return false, nil
			} else if err == context.Canceled {
				return true, err
			}
			// Replace b with a new backend - this is done to ensure that there is
			// no side effects from the execution of b that have carried over
			b = backend
			return true, nil
		})
		if err != nil {
			return true, err
		}
		return false, nil
	})

	if err == context.Canceled {
		return nil
	}

	return err
}

// Run starts all of the Backend server's daemons
func (b *Backend) Run() error {
	return b.RunWithInitializer(Initialize)
}

type stopper interface {
	Stop() error
	Name() string
}

type stopGroup []stopper

func (s stopGroup) Stop() (err error) {
	for _, stopper := range s {
		logger.Info("shutting down ", stopper.Name())
		e := stopper.Stop()
		if err == nil {
			err = e
		}
	}
	return err
}

type errorer interface {
	Err() <-chan error
	Name() string
}

type errGroup struct {
	out     chan error
	daemons []errorer
	wg      sync.WaitGroup
}

func (e *errGroup) Go(ctx context.Context) {
	e.wg.Add(len(e.daemons))
	for _, daemon := range e.daemons {
		daemon := daemon
		go func() {
			defer e.wg.Done()
			select {
			case err := <-daemon.Err():
				err = fmt.Errorf("error from %s: %s", daemon.Name(), err)
				select {
				case e.out <- err:
				case <-ctx.Done():
				}
			case <-ctx.Done():
			}
		}()
	}
}

func (e *errGroup) Err() <-chan error {
	return e.out
}

func (e *errGroup) WaitStop() {
	e.wg.Wait()
}

// Stop the Backend cleanly.
func (b *Backend) Stop() {
	b.runCancel()
}

func (b *Backend) getBackendEntity(config *Config) *corev2.Entity {
	entity := &corev2.Entity{
		EntityClass: corev2.EntityBackendClass,
		System:      getSystemInfo(),
		ObjectMeta: corev2.ObjectMeta{
			Name:        getDefaultBackendID(), //hostname
			Labels:      b.cfg.Labels,
			Annotations: b.cfg.Annotations,
		},
	}

	if config.DeregistrationHandler != "" {
		entity.Deregistration = corev2.Deregistration{
			Handler: config.DeregistrationHandler,
		}
	}

	return entity
}

// getDefaultBackendID returns the default backend ID
func getDefaultBackendID() string {
	defaultBackendID, err := os.Hostname()
	if err != nil {
		logger.WithError(err).Error("error getting hostname")
		defaultBackendID = "unidentified-sensu-backend"
	}
	return defaultBackendID
}

// getSystemInfo returns the system info of the backend
func getSystemInfo() corev2.System {
	info, err := system.Info()
	if err != nil {
		logger.WithError(err).Error("error getting system info")
	}
	return info
}
