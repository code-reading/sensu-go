#### version

对外统一提供Println(componentName), 打印每个组件的版本信息

```
组件名称 版本信息(版本前缀, 版本标识) 编译SHA  编译日期 GoVersion
```

其中,
Version,BuildSHA,BuildDate 在编译时注入,

```sh
go build -ldflags '-X "github.com/sensu/sensu-go/version.Version=5.14.0" -X "github.com/sensu/sensu-go/version.BuildDate=2019-10-08" -X "github.com/sensu/sensu-go/version.BuildSHA='`git rev-parse HEAD`'"' -o bin/sensu-agent ./cmd/sensu-agent
```

```go
package version

import (
	"runtime"
	"runtime/debug"
)

var (
    // Version stores the version of the current build (e.g. 2.0.0)
	Version = ""
	// GoVersion stores the version of Go used to build the binary
	// (e.g. go1.14.2)
	GoVersion string = runtime.Version()
)

// Semver returns full semantic versioning compatible identifier.
// Format: VERSION-PRERELEASE+METADATA
func Semver() string {
	version := Version

	// If we don't have a version because it has been manually built from source,
	// use Go build info to display the main module version
	if version == "" {
		buildInfo, ok := debug.ReadBuildInfo()
		if ok {
			version = buildInfo.Main.Version
		}
	}

	return version
}
```

可以通过golang的runtime 获取编译的版本信息 及Go的版本信息;
