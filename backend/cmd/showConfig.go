package cmd

import (
	"fmt"

	"github.com/sensu/sensu-go/backend"
)

func showConfig(cfg *backend.Config, step string) {
	if len(step) != 0 {
		fmt.Println("step:", step)
	}
	fmt.Println("AgentHost:", cfg.AgentHost)
	fmt.Println("AgentPort:", cfg.AgentPort)
	fmt.Println("AgentWriteTimeout:", cfg.AgentWriteTimeout)
	fmt.Println("APIListenAddress:", cfg.APIListenAddress)
	fmt.Println("APIURL:", cfg.APIURL)
	fmt.Println("AssetsRateLimit:", cfg.AssetsRateLimit)
	fmt.Println("AssetsBurstLimit:", cfg.AssetsBurstLimit)
	fmt.Println("DashboardHost:", cfg.DashboardHost)
	fmt.Println("DashboardPort:", cfg.DashboardPort)
	fmt.Println("DashboardTLSCertFile:", cfg.DashboardTLSCertFile)
	fmt.Println("DashboardTLSKeyFile:", cfg.DashboardTLSKeyFile)
	fmt.Println("DeregistrationHandler:", cfg.DeregistrationHandler)
	fmt.Println("CacheDir:", cfg.CacheDir)
	fmt.Println("StateDir:", cfg.StateDir)
	fmt.Println("EtcdAdvertiseClientURLs:", cfg.EtcdAdvertiseClientURLs)
	fmt.Println("EtcdListenClientURLs:", cfg.EtcdListenClientURLs)
	fmt.Println("EtcdClientURLs:", cfg.EtcdClientURLs)
	fmt.Println("EtcdListenPeerURLs:", cfg.EtcdListenPeerURLs)
	fmt.Println("EtcdInitialCluster:", cfg.EtcdInitialCluster)
	fmt.Println("EtcdInitialClusterState:", cfg.EtcdInitialClusterState)
	fmt.Println("EtcdDiscovery:", cfg.EtcdDiscovery)
	fmt.Println("EtcdDiscoverySrv:", cfg.EtcdDiscoverySrv)
	fmt.Println("EtcdInitialAdvertisePeerURLs:", cfg.EtcdInitialAdvertisePeerURLs)
	fmt.Println("EtcdInitialClusterToken:", cfg.EtcdInitialClusterToken)
	fmt.Println("EtcdName:", cfg.EtcdName)
	fmt.Println("EtcdCipherSuites:", cfg.EtcdCipherSuites)
	fmt.Println("EtcdQuotaBackendBytes:", cfg.EtcdQuotaBackendBytes)
	fmt.Println("EtcdMaxRequestBytes:", cfg.EtcdMaxRequestBytes)
	fmt.Println("EtcdHeartbeatInterval:", cfg.EtcdHeartbeatInterval)
	fmt.Println("EtcdElectionTimeout:", cfg.EtcdElectionTimeout)
	fmt.Println("NoEmbedEtcd:", cfg.NoEmbedEtcd)
	fmt.Println("Labels:", cfg.Labels)
	fmt.Println("Annotations:", cfg.Annotations)

}
