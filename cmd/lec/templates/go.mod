module {{.Module}}

go 1.22

require {{.CoreModule}} v0.10.0

replace {{.CoreModule}} => ../../go-core
