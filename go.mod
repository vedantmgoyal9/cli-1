module github.com/databricks/bricks

go 1.18

require (
	github.com/atotto/clipboard v0.1.4
	github.com/databricks/databricks-sdk-go v0.0.0
	github.com/ghodss/yaml v1.0.0 // MIT + NOTICE
	github.com/manifoldco/promptui v0.9.0 // BSD-3-Clause license
	github.com/mitchellh/go-homedir v1.1.0 // MIT
	github.com/pkg/browser v0.0.0-20210911075715-681adbf594b8 // BSD-2-Clause
	github.com/sabhiram/go-gitignore v0.0.0-20210923224102-525f6e181f06 // MIT
	github.com/spf13/cobra v1.5.0 // Apache 2.0
	github.com/stretchr/testify v1.8.0 // MIT
	github.com/whilp/git-urls v1.0.0 // MIT
	golang.org/x/mod v0.6.0 // BSD-3-Clause
	gopkg.in/ini.v1 v1.67.0 // Apache 2.0
)

require (
	cloud.google.com/go/compute v1.6.1 // indirect
	github.com/briandowns/spinner v1.19.0
	github.com/chzyer/readline v0.0.0-20180603132655-2972be24d48e // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fatih/color v1.7.0
	github.com/golang/groupcache v0.0.0-20200121045136-8c9f03a8e57e // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/mattn/go-colorable v0.1.2 // indirect
	github.com/mattn/go-isatty v0.0.8 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	go.opencensus.io v0.23.0 // indirect
	golang.org/x/exp v0.0.0-20221028150844-83b7d23a625f
	golang.org/x/net v0.0.0-20220526153639-5463443f8c37 // indirect
	golang.org/x/oauth2 v0.0.0-20220628200809-02e64fa58f26 // indirect
	golang.org/x/sync v0.0.0-20220513210516-0976fa681c29
	golang.org/x/sys v0.1.0 // indirect
	golang.org/x/text v0.3.7 // indirect
	golang.org/x/time v0.0.0-20210723032227-1f47c861a9ac // indirect
	google.golang.org/api v0.82.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20220527130721-00d5c0f3be58 // indirect
	google.golang.org/grpc v1.46.2 // indirect
	google.golang.org/protobuf v1.28.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/databricks/databricks-sdk-go v0.0.0 => ./ext/databricks-sdk-go
