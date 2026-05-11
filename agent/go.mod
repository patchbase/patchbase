module go.patchbase.net/agent

go 1.25

require (
	github.com/spf13/afero v1.14.0
	github.com/stretchr/testify v1.10.0
	go.patchbase.net/proto/agent v0.0.0
	google.golang.org/protobuf v1.36.11
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/text v0.23.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace go.patchbase.net/proto/agent => ../proto/agent
