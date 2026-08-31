module go.patchbase.net/agent

go 1.27

require (
	github.com/spf13/afero v1.15.0
	github.com/spf13/cobra v1.10.2
	github.com/stretchr/testify v1.12.1
	go.patchbase.net/proto/agent v0.0.0
	google.golang.org/protobuf v1.36.12
)

require (
	github.com/envoyproxy/protoc-gen-validate v1.3.3 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/text v0.41.0 // indirect
)

replace go.patchbase.net/proto/agent => ../proto/agent
