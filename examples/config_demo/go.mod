module github.com/najoast/sngo/examples/config_demo

go 1.21

replace github.com/najoast/sngo => ../..

require github.com/najoast/sngo v0.0.0-00010101000000-000000000000

require (
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	golang.org/x/sys v0.13.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
