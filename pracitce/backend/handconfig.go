package main
import (
	"io/ioutil"
	"fmt"
	"os"

	"path/filepath"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)
const (
	flagConfigFile            = "config-file"
	flagAgentHost             = "agent-host"
	flagAgentPort             = "agent-port"
	flagEtcdAdvertiseClientURLs      = "etcd-advertise-client-urls"
	defaultEtcdAdvertiseClientURL = "http://localhost:2379"
)
func main() {
	rootCmd := &cobra.Command{
		Use:   "test-cli",
		Short: "test cli",
	}

	rootCmd.AddCommand(CliCommand())

	if err := rootCmd.Execute(); err != nil {
		fmt.Println("rootCmd.error:",err)
	}

}
func CliCommand() *cobra.Command {
	cli := &cobra.Command{
		Use:           "start",
		Short:         "start the sensu backend",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// 将cmd.Flags 绑定到viper 配置上
			// 下面通过viper 配置读取所有配置参数值;
			 _ = viper.BindPFlags(cmd.Flags())
			val, err := cmd.Flags().GetString(flagAgentHost)
			fmt.Println("val:", val, " err:",err)
			fmt.Println("viper.host:", viper.GetString(flagAgentHost))
			return nil
		},
	}

	fmt.Println(handleConfig(cli, true))
	return cli
}

func handleConfig(cmd *cobra.Command, server bool) error {
	// Set up distinct flagset for handling config file
	configFlagSet := pflag.NewFlagSet("sensu", pflag.ContinueOnError)
	configFileDefaultLocation := filepath.Join(filepath.Join("/tmp/sensu"), "backend.yml")
	configFileDefault := fmt.Sprintf("path to sensu-backend config file (default %q)", configFileDefaultLocation)
	configFlagSet.StringP(flagConfigFile, "c", "", configFileDefault)
	configFlagSet.SetOutput(ioutil.Discard)
	//parses flag definitions from the argument list, which should not include the command name.
	// 读取命令行参数值对;
	_ = configFlagSet.Parse(os.Args[1:])
	// 检查当前是否有可读取的Flags
	fmt.Println("Aflags:",configFlagSet.HasAvailableFlags())
	// 当前有的Flags数量
	fmt.Println("Nflags:",configFlagSet.NFlag())
	// 读取所有的Flags
	configFlagSet.VisitAll(func(flag *pflag.Flag) {
		fmt.Println(flag.Name, " -- ", flag.Value)
	})

	// Get the given config file path
	configFile, _ := configFlagSet.GetString(flagConfigFile)
	configFilePath := configFile

	// use the default config path if flagConfigFile was used
	// 如果配置文件路径为空， 使用默认配置文件路径
	if configFile == "" {
		configFilePath = configFileDefaultLocation
	}

	// Configure location of backend configuration
	viper.SetConfigType("yaml")
	viper.SetConfigFile(configFilePath)

	//sensu-backend server端默认配置
	if server {
		// Flag defaults
		viper.SetDefault(flagAgentHost, "[::]")
		viper.SetDefault(flagAgentPort, 8081)
	}

	// Etcd defaults
	viper.SetDefault(flagEtcdAdvertiseClientURLs, defaultEtcdAdvertiseClientURL)

	// Merge in config flag set so that it appears in command usage
	// 将配置文件中的Flag 合并到 cmd.Flags中
	cmd.Flags().AddFlagSet(configFlagSet)

	if server {
		// Main Flags
		cmd.Flags().String(flagAgentHost, viper.GetString(flagAgentHost), "agent listener host")
		cmd.Flags().Int(flagAgentPort, viper.GetInt(flagAgentPort), "agent listener port")
	}
	// Load the configuration file but only error out if flagConfigFile is used
	if err := viper.ReadInConfig(); err != nil && configFile != "" {
		return err
	}
	return nil
}

